package sidechain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"
)

// TransactionStatus represents the status of a transaction
type TransactionStatus string

const (
	StatusPending   TransactionStatus = "pending"
	StatusValidated TransactionStatus = "validated"
	StatusFailed    TransactionStatus = "failed"
	StatusRolledUp  TransactionStatus = "rolled_up"
)

// ProtocolTransaction represents a transaction routed to a specific protocol
type ProtocolTransaction struct {
	ID           string            `json:"id"`
	Protocol     string            `json:"protocol"` // "BANK" or "MESSAGE"
	Data         []byte            `json:"data"`
	Status       TransactionStatus `json:"status"`
	CreatedAt    time.Time         `json:"created_at"`
	ValidatedAt  *time.Time        `json:"validated_at,omitempty"`
	FailedAt     *time.Time        `json:"failed_at,omitempty"`
	ErrorMessage string            `json:"error_message,omitempty"`
	Sender       string            `json:"sender"`
	Recipient    string            `json:"recipient"`
}

// RollupBlock represents a rollup block containing validated transactions
type RollupBlock struct {
	ID           string                 `json:"id"`
	Protocol     string                 `json:"protocol"`
	Transactions []*ProtocolTransaction `json:"transactions"`
	MerkleRoot   string                 `json:"merkle_root"`
	CreatedAt    time.Time              `json:"created_at"`
	BlockNumber  int                    `json:"block_number"`
}

// ProtocolRouter handles protocol-based transaction routing
type ProtocolRouter struct {
	mu sync.RWMutex

	// Protocol queues
	bankQueue    []*ProtocolTransaction
	messageQueue []*ProtocolTransaction

	// Rollup configuration
	rollupInterval time.Duration // 20 seconds
	lastRollup     map[string]time.Time

	// Callbacks for transaction processing
	onTransactionValidated func(*ProtocolTransaction) error
	onTransactionFailed    func(*ProtocolTransaction, string) error
	onRollupCreated        func(*RollupBlock) error

	// Statistics
	stats *RouterStats
}

// RouterStats holds router statistics
type RouterStats struct {
	mu sync.RWMutex

	TotalTransactions     int64 `json:"total_transactions"`
	ValidatedTransactions int64 `json:"validated_transactions"`
	FailedTransactions    int64 `json:"failed_transactions"`
	RollupBlocksCreated   int64 `json:"rollup_blocks_created"`

	BankTransactions    int64 `json:"bank_transactions"`
	MessageTransactions int64 `json:"message_transactions"`

	AverageValidationTime time.Duration `json:"average_validation_time"`
	AverageRollupTime     time.Duration `json:"average_rollup_time"`
}

// NewProtocolRouter creates a new protocol router
func NewProtocolRouter() *ProtocolRouter {
	router := &ProtocolRouter{
		rollupInterval: 20 * time.Second,
		lastRollup:     make(map[string]time.Time),
		stats:          &RouterStats{},
	}

	// Start rollup timer
	go router.startRollupTimer()

	return router
}

// RouteTransaction routes a transaction to the appropriate protocol
func (pr *ProtocolRouter) RouteTransaction(
	protocol string,
	data []byte,
	sender string,
	recipient string,
) (*ProtocolTransaction, error) {

	pr.mu.Lock()
	defer pr.mu.Unlock()

	// Create transaction
	tx := &ProtocolTransaction{
		ID:        generateTransactionID(data, sender, recipient),
		Protocol:  protocol,
		Data:      data,
		Status:    StatusPending,
		CreatedAt: time.Now(),
		Sender:    sender,
		Recipient: recipient,
	}

	// Route to appropriate queue
	switch protocol {
	case "BANK":
		pr.bankQueue = append(pr.bankQueue, tx)
		pr.stats.BankTransactions++
	case "MESSAGE":
		pr.messageQueue = append(pr.messageQueue, tx)
		pr.stats.MessageTransactions++
	default:
		return nil, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	pr.stats.TotalTransactions++

	// Start validation in background
	go pr.validateTransaction(tx)

	return tx, nil
}

// validateTransaction validates a transaction
func (pr *ProtocolRouter) validateTransaction(tx *ProtocolTransaction) {
	startTime := time.Now()

	// Simulate validation (in real implementation, this would validate the transaction)
	// For now, we'll just mark it as validated after a short delay
	time.Sleep(100 * time.Millisecond)

	pr.mu.Lock()
	defer pr.mu.Unlock()

	// Update transaction status
	tx.Status = StatusValidated
	now := time.Now()
	tx.ValidatedAt = &now

	// Update statistics
	pr.stats.ValidatedTransactions++
	pr.stats.AverageValidationTime = (pr.stats.AverageValidationTime + time.Since(startTime)) / 2

	// Call validation callback if set
	if pr.onTransactionValidated != nil {
		if err := pr.onTransactionValidated(tx); err != nil {
			pr.markTransactionFailed(tx, err.Error())
		}
	}
}

// markTransactionFailed marks a transaction as failed
func (pr *ProtocolRouter) markTransactionFailed(tx *ProtocolTransaction, errorMsg string) {
	tx.Status = StatusFailed
	now := time.Now()
	tx.FailedAt = &now
	tx.ErrorMessage = errorMsg

	pr.stats.FailedTransactions++

	// Call failure callback if set
	if pr.onTransactionFailed != nil {
		if err := pr.onTransactionFailed(tx, errorMsg); err != nil {
			// Log the error but don't fail the transaction marking
			// This is a callback error, not a transaction error
			_ = err // Suppress unused variable warning
		}
	}
}

// startRollupTimer starts the rollup timer
func (pr *ProtocolRouter) startRollupTimer() {
	ticker := time.NewTicker(pr.rollupInterval)
	defer ticker.Stop()

	for range ticker.C {
		pr.processRollups()
	}
}

// processRollups processes rollups for all protocols
func (pr *ProtocolRouter) processRollups() {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	// Process BANK rollup
	if len(pr.bankQueue) > 0 {
		pr.createRollup("BANK", pr.bankQueue)
		pr.bankQueue = nil // Clear queue after rollup
	}

	// Process MESSAGE rollup
	if len(pr.messageQueue) > 0 {
		pr.createRollup("MESSAGE", pr.messageQueue)
		pr.messageQueue = nil // Clear queue after rollup
	}
}

// createRollup creates a rollup block for a protocol
func (pr *ProtocolRouter) createRollup(protocol string, transactions []*ProtocolTransaction) {
	// Filter only validated transactions
	var validatedTxs []*ProtocolTransaction
	for _, tx := range transactions {
		if tx.Status == StatusValidated {
			validatedTxs = append(validatedTxs, tx)
			tx.Status = StatusRolledUp
		}
	}

	if len(validatedTxs) == 0 {
		return
	}

	// Create rollup block
	rollup := &RollupBlock{
		ID:           generateRollupID(protocol, time.Now()),
		Protocol:     protocol,
		Transactions: validatedTxs,
		CreatedAt:    time.Now(),
		BlockNumber:  int(pr.stats.RollupBlocksCreated) + 1,
	}

	// Calculate merkle root
	rollup.MerkleRoot = pr.calculateMerkleRoot(validatedTxs)

	// Update statistics
	pr.stats.RollupBlocksCreated++
	pr.lastRollup[protocol] = time.Now()

	// Call rollup callback if set
	if pr.onRollupCreated != nil {
		if err := pr.onRollupCreated(rollup); err != nil {
			// Log the error but don't fail the rollup creation
			// This is a callback error, not a rollup error
			_ = err // Suppress unused variable warning
		}
	}
}

// calculateMerkleRoot calculates the merkle root of transactions
func (pr *ProtocolRouter) calculateMerkleRoot(transactions []*ProtocolTransaction) string {
	if len(transactions) == 0 {
		return ""
	}

	// Create leaf hashes
	var hashes [][]byte
	for _, tx := range transactions {
		data := fmt.Sprintf("%s:%s:%s", tx.ID, tx.Protocol, string(tx.Data))
		hash := sha256.Sum256([]byte(data))
		hashes = append(hashes, hash[:])
	}

	// Build merkle tree
	for len(hashes) > 1 {
		var newHashes [][]byte
		for i := 0; i < len(hashes); i += 2 {
			if i+1 < len(hashes) {
				combined := append(hashes[i], hashes[i+1]...)
				hash := sha256.Sum256(combined)
				newHashes = append(newHashes, hash[:])
			} else {
				newHashes = append(newHashes, hashes[i])
			}
		}
		hashes = newHashes
	}

	return hex.EncodeToString(hashes[0])
}

// SetCallbacks sets the callback functions
func (pr *ProtocolRouter) SetCallbacks(
	onValidated func(*ProtocolTransaction) error,
	onFailed func(*ProtocolTransaction, string) error,
	onRollup func(*RollupBlock) error,
) {
	pr.mu.Lock()
	defer pr.mu.Unlock()

	pr.onTransactionValidated = onValidated
	pr.onTransactionFailed = onFailed
	pr.onRollupCreated = onRollup
}

// GetStats returns router statistics
func (pr *ProtocolRouter) GetStats() *RouterStats {
	pr.stats.mu.RLock()
	defer pr.stats.mu.RUnlock()

	// Create a copy to avoid race conditions
	stats := &RouterStats{}
	// Copy stats without copying the mutex
	stats.TotalTransactions = pr.stats.TotalTransactions
	stats.ValidatedTransactions = pr.stats.ValidatedTransactions
	stats.FailedTransactions = pr.stats.FailedTransactions
	stats.RollupBlocksCreated = pr.stats.RollupBlocksCreated
	stats.BankTransactions = pr.stats.BankTransactions
	stats.MessageTransactions = pr.stats.MessageTransactions
	stats.AverageValidationTime = pr.stats.AverageValidationTime
	stats.AverageRollupTime = pr.stats.AverageRollupTime
	return stats
}

// GetQueueStatus returns the current queue status
func (pr *ProtocolRouter) GetQueueStatus() map[string]int {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	return map[string]int{
		"BANK":    len(pr.bankQueue),
		"MESSAGE": len(pr.messageQueue),
	}
}

// GetLastRollup returns the last rollup time for each protocol
func (pr *ProtocolRouter) GetLastRollup() map[string]time.Time {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	result := make(map[string]time.Time)
	for protocol, lastTime := range pr.lastRollup {
		result[protocol] = lastTime
	}
	return result
}

// GetValidatedTransactions returns validated transactions for a specific protocol
func (pr *ProtocolRouter) GetValidatedTransactions(protocol string) []*ProtocolTransaction {
	pr.mu.RLock()
	defer pr.mu.RUnlock()

	var validatedTxs []*ProtocolTransaction

	switch protocol {
	case "BANK":
		for _, tx := range pr.bankQueue {
			if tx.Status == StatusValidated {
				validatedTxs = append(validatedTxs, tx)
			}
		}
	case "MESSAGE":
		for _, tx := range pr.messageQueue {
			if tx.Status == StatusValidated {
				validatedTxs = append(validatedTxs, tx)
			}
		}
	}

	return validatedTxs
}

// Helper functions
func generateTransactionID(data []byte, sender, recipient string) string {
	input := fmt.Sprintf("%s:%s:%s", string(data), sender, recipient)
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}

func generateRollupID(protocol string, timestamp time.Time) string {
	input := fmt.Sprintf("%s:%d", protocol, timestamp.UnixNano())
	hash := sha256.Sum256([]byte(input))
	return hex.EncodeToString(hash[:])
}
