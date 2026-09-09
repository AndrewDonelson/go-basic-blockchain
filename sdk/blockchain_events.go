// Package sdk is a software development kit for building blockchain applications.
// File sdk/blockchain_events.go - Announcements, sidechain callbacks and display state.
package sdk

import (
	"log"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/helios/sidechain"
	"github.com/AndrewDonelson/go-basic-blockchain/internal/progress"
)

// DisplayStatus displays the current status of the blockchain.
func (bc *Blockchain) DisplayStatus() {
	// Check if menu is active - if so, don't display ANY status at all
	bc.menuMutex.RLock()
	menuActive := bc.menuActive
	bc.menuMutex.RUnlock()

	if menuActive {
		return // Exit early - don't display ANY status when menu is active
	}

	bc.mux.Lock()
	defer bc.mux.Unlock()

	currentAction := ""
	peerCount := 0
	uptime := time.Duration(0)
	if bc.progressIndicator != nil {
		currentAction = bc.progressIndicator.CurrentStatus().Action
	}
	if n := GetNode(); n != nil && n.P2P != nil {
		peerCount = n.P2P.PeerCount()
		if !n.StartedAt.IsZero() {
			uptime = time.Since(n.StartedAt)
		}
	}

	// (A comparison of len(bc.Blocks) against a copy taken two lines earlier used
	// to live here. It could never be true.)

	// Update progress indicator only if not paused
	if bc.progressIndicator != nil {
		// Check if progress indicator is paused (menu is active)
		if !bc.progressIndicator.IsPaused() {
			status := progress.BlockchainStatus{
				Action:      currentAction,
				IsMining:    true, // Assume mining is active
				BlockCount:  len(bc.Blocks),
				TxQueueSize: len(bc.TransactionQueue),
				Difficulty:  bc.cfg.Difficulty,
				HashRate:    bc.Metrics().HashRate(),
				LastBlock:   "",
				Peers:       peerCount,
				IsSynced:    true,
				Uptime:      uptime,
			}

			if len(bc.Blocks) > 0 {
				status.LastBlock = bc.Blocks[len(bc.Blocks)-1].Hash
			}

			bc.progressIndicator.UpdateStatus(status)
		}
	}
}

// SetBlockAnnouncer registers a callback invoked after a block is added locally,
// so the networking layer can relay it without the chain importing the P2P code.
func (bc *Blockchain) SetBlockAnnouncer(fn func(*Block)) {
	bc.announceMu.Lock()
	defer bc.announceMu.Unlock()
	bc.announceBlock = fn
}

// SetTransactionAnnouncer registers a callback invoked when a transaction enters
// the local mempool.
func (bc *Blockchain) SetTransactionAnnouncer(fn func(Transaction)) {
	bc.announceMu.Lock()
	defer bc.announceMu.Unlock()
	bc.announceTx = fn
}

// announce relays a newly added block to the network, if an announcer is set.
func (bc *Blockchain) announce(block *Block) {
	bc.announceMu.RLock()
	fn := bc.announceBlock
	bc.announceMu.RUnlock()

	if fn != nil {
		fn(block)
	}
}

// announceTransaction relays a newly queued transaction, if an announcer is set.
func (bc *Blockchain) announceTransaction(tx Transaction) {
	bc.announceMu.RLock()
	fn := bc.announceTx
	bc.announceMu.RUnlock()

	if fn != nil {
		fn(tx)
	}
}

// Sidechain router callback methods
func (bc *Blockchain) onTransactionValidated(tx *sidechain.ProtocolTransaction) error {
	log.Printf("Transaction validated: %s (protocol: %s)", tx.ID, tx.Protocol)
	return nil
}

func (bc *Blockchain) onTransactionFailed(tx *sidechain.ProtocolTransaction, errorMsg string) error {
	log.Printf("Transaction failed: %s (protocol: %s) - %s", tx.ID, tx.Protocol, errorMsg)
	return nil
}

func (bc *Blockchain) onRollupCreated(rollup *sidechain.RollupBlock) error {
	log.Printf("Rollup block created: %s (protocol: %s) with %d transactions",
		rollup.ID, rollup.Protocol, len(rollup.Transactions))
	return nil
}

// GetProgressIndicator returns the progress indicator instance
func (bc *Blockchain) GetProgressIndicator() *progress.ProgressIndicator {
	return bc.progressIndicator
}

// SetMenuActive sets the menu active state
func (bc *Blockchain) SetMenuActive(active bool) {
	bc.menuMutex.Lock()
	defer bc.menuMutex.Unlock()
	bc.menuActive = active
}

// IsMenuActive returns true if the menu is currently active
func (bc *Blockchain) IsMenuActive() bool {
	bc.menuMutex.RLock()
	defer bc.menuMutex.RUnlock()
	return bc.menuActive
}
