# Section 4: Core Data Structures

## 🏗️ Building the Foundation Blocks

Welcome to Section 4! This section focuses on implementing the core data structures that form the foundation of your blockchain. You'll learn how to design and implement efficient, secure data structures for blocks, transactions, wallets, and more.

### **What You'll Learn in This Section**

- Building the Block struct with proper organization
- Transaction interfaces and implementations
- Wallet system design and implementation
- Address generation and validation
- PUID (Persistent Unique ID) system
- Merkle tree implementation

### **Section Overview**

This section bridges theory and implementation by creating the actual data structures you'll use in your blockchain. We'll focus on clean, efficient designs that are both secure and performant.

---

## 🧱 Building the Block Struct

### **Enhanced Block Structure**

```go
package blockchain

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "math/big"
    "time"
)

// BlockHeader contains metadata about the block
type BlockHeader struct {
    Index        int       `json:"index"`
    Timestamp    time.Time `json:"timestamp"`
    PreviousHash string    `json:"previous_hash"`
    MerkleRoot   string    `json:"merkle_root"`
    Nonce        int       `json:"nonce"`
    Difficulty   int       `json:"difficulty"`
    Version      int       `json:"version"`
}

// Block represents a blockchain block
type Block struct {
    Header       BlockHeader   `json:"header"`
    Transactions []Transaction `json:"transactions"`
    Hash         string        `json:"hash"`
    Size         int           `json:"size"`
}

// NewBlock creates a new block
func NewBlock(index int, transactions []Transaction, previousHash string) *Block {
    block := &Block{
        Header: BlockHeader{
            Index:        index,
            Timestamp:    time.Now(),
            PreviousHash: previousHash,
            Nonce:        0,
            Difficulty:   4, // Default difficulty
            Version:      1,
        },
        Transactions: transactions,
    }
    
    // Calculate Merkle root
    block.Header.MerkleRoot = block.calculateMerkleRoot()
    
    // Calculate initial hash
    block.Hash = block.calculateHash()
    
    // Calculate block size
    block.Size = block.calculateSize()
    
    return block
}

// calculateHash calculates the hash of the block
func (b *Block) calculateHash() string {
    // Create a string representation of the header
    headerData := fmt.Sprintf("%d%s%s%s%d%d%d",
        b.Header.Index,
        b.Header.Timestamp.Format(time.RFC3339),
        b.Header.PreviousHash,
        b.Header.MerkleRoot,
        b.Header.Nonce,
        b.Header.Difficulty,
        b.Header.Version)
    
    hash := sha256.Sum256([]byte(headerData))
    return hex.EncodeToString(hash[:])
}

// calculateMerkleRoot calculates the Merkle root of transactions
func (b *Block) calculateMerkleRoot() string {
    if len(b.Transactions) == 0 {
        return ""
    }
    
    // Create leaf hashes
    leaves := make([]string, len(b.Transactions))
    for i, tx := range b.Transactions {
        leaves[i] = tx.GetHash()
    }
    
    // Build Merkle tree
    return buildMerkleTree(leaves)
}

// buildMerkleTree builds a Merkle tree from transaction hashes
func buildMerkleTree(leaves []string) string {
    if len(leaves) == 0 {
        return ""
    }
    
    if len(leaves) == 1 {
        return leaves[0]
    }
    
    // If odd number of leaves, duplicate the last one
    if len(leaves)%2 != 0 {
        leaves = append(leaves, leaves[len(leaves)-1])
    }
    
    // Create parent level
    parents := make([]string, len(leaves)/2)
    for i := 0; i < len(leaves); i += 2 {
        combined := leaves[i] + leaves[i+1]
        hash := sha256.Sum256([]byte(combined))
        parents[i/2] = hex.EncodeToString(hash[:])
    }
    
    // Recursively build the tree
    return buildMerkleTree(parents)
}

// calculateSize calculates the size of the block in bytes
func (b *Block) calculateSize() int {
    data, err := json.Marshal(b)
    if err != nil {
        return 0
    }
    return len(data)
}

// MineBlock mines the block with proof of work
func (b *Block) MineBlock() {
    target := ""
    for i := 0; i < b.Header.Difficulty; i++ {
        target += "0"
    }
    
    for {
        b.Hash = b.calculateHash()
        if b.Hash[:b.Header.Difficulty] == target {
            break
        }
        b.Header.Nonce++
    }
}

// Validate validates the block
func (b *Block) Validate() error {
    // Check basic fields
    if b.Header.Index < 0 {
        return fmt.Errorf("block index cannot be negative")
    }
    
    if len(b.Transactions) == 0 {
        return fmt.Errorf("block must contain at least one transaction")
    }
    
    // Verify hash
    expectedHash := b.calculateHash()
    if b.Hash != expectedHash {
        return fmt.Errorf("invalid block hash")
    }
    
    // Verify Merkle root
    expectedMerkleRoot := b.calculateMerkleRoot()
    if b.Header.MerkleRoot != expectedMerkleRoot {
        return fmt.Errorf("invalid Merkle root")
    }
    
    // Validate all transactions
    for i, tx := range b.Transactions {
        if err := tx.Validate(); err != nil {
            return fmt.Errorf("invalid transaction %d: %w", i, err)
        }
    }
    
    return nil
}

// ToJSON serializes the block to JSON
func (b *Block) ToJSON() ([]byte, error) {
    return json.MarshalIndent(b, "", "  ")
}

// FromJSON deserializes JSON to a block
func (b *Block) FromJSON(data []byte) error {
    return json.Unmarshal(data, b)
}
```

---

## 💰 Transaction Interfaces and Implementations

### **Transaction Interface**

```go
// Transaction interface defines what a transaction must implement
type Transaction interface {
    GetID() string
    GetHash() string
    GetSender() string
    GetRecipient() string
    GetAmount() float64
    GetFee() float64
    GetTimestamp() time.Time
    GetType() string
    Validate() error
    Sign(privateKey []byte) error
    VerifySignature() bool
    ToJSON() ([]byte, error)
    FromJSON(data []byte) error
}

// BaseTransaction provides common transaction functionality
type BaseTransaction struct {
    ID        string    `json:"id"`
    Timestamp time.Time `json:"timestamp"`
    Sender    string    `json:"sender"`
    Recipient string    `json:"recipient"`
    Amount    float64   `json:"amount"`
    Fee       float64   `json:"fee"`
    Signature []byte    `json:"signature"`
    Type      string    `json:"type"`
}

// NewBaseTransaction creates a new base transaction
func NewBaseTransaction(sender, recipient string, amount, fee float64, txType string) *BaseTransaction {
    return &BaseTransaction{
        ID:        generateTransactionID(),
        Timestamp: time.Now(),
        Sender:    sender,
        Recipient: recipient,
        Amount:    amount,
        Fee:       fee,
        Type:      txType,
    }
}

// generateTransactionID generates a unique transaction ID
func generateTransactionID() string {
    data := fmt.Sprintf("%d%s%s%f%f", 
        time.Now().UnixNano(),
        "sender",
        "recipient",
        0.0,
        0.0)
    
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

// GetID returns the transaction ID
func (bt *BaseTransaction) GetID() string {
    return bt.ID
}

// GetHash returns the transaction hash
func (bt *BaseTransaction) GetHash() string {
    data := fmt.Sprintf("%s%s%s%f%f%s", 
        bt.ID,
        bt.Sender,
        bt.Recipient,
        bt.Amount,
        bt.Fee,
        bt.Type)
    
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

// GetSender returns the sender address
func (bt *BaseTransaction) GetSender() string {
    return bt.Sender
}

// GetRecipient returns the recipient address
func (bt *BaseTransaction) GetRecipient() string {
    return bt.Recipient
}

// GetAmount returns the transaction amount
func (bt *BaseTransaction) GetAmount() float64 {
    return bt.Amount
}

// GetFee returns the transaction fee
func (bt *BaseTransaction) GetFee() float64 {
    return bt.Fee
}

// GetTimestamp returns the transaction timestamp
func (bt *BaseTransaction) GetTimestamp() time.Time {
    return bt.Timestamp
}

// GetType returns the transaction type
func (bt *BaseTransaction) GetType() string {
    return bt.Type
}

// Validate validates the base transaction
func (bt *BaseTransaction) Validate() error {
    if bt.Amount < 0 {
        return fmt.Errorf("amount cannot be negative")
    }
    
    if bt.Fee < 0 {
        return fmt.Errorf("fee cannot be negative")
    }
    
    if bt.Sender == "" {
        return fmt.Errorf("sender cannot be empty")
    }
    
    if bt.Recipient == "" {
        return fmt.Errorf("recipient cannot be empty")
    }
    
    return nil
}

// Sign signs the transaction
func (bt *BaseTransaction) Sign(privateKey []byte) error {
    // Implementation would use ECDSA or similar
    // For now, we'll create a simple signature
    data := bt.GetHash()
    bt.Signature = []byte(data)
    return nil
}

// VerifySignature verifies the transaction signature
func (bt *BaseTransaction) VerifySignature() bool {
    // Implementation would verify ECDSA signature
    // For now, we'll do a simple check
    return len(bt.Signature) > 0
}

// ToJSON serializes the transaction to JSON
func (bt *BaseTransaction) ToJSON() ([]byte, error) {
    return json.MarshalIndent(bt, "", "  ")
}

// FromJSON deserializes JSON to a transaction
func (bt *BaseTransaction) FromJSON(data []byte) error {
    return json.Unmarshal(data, bt)
}
```

### **Specific Transaction Types**

```go
// BankTransaction represents a bank transfer
type BankTransaction struct {
    BaseTransaction
    Currency string `json:"currency"`
}

// NewBankTransaction creates a new bank transaction
func NewBankTransaction(sender, recipient string, amount, fee float64) *BankTransaction {
    return &BankTransaction{
        BaseTransaction: *NewBaseTransaction(sender, recipient, amount, fee, "BANK"),
        Currency:        "USD",
    }
}

// MessageTransaction represents a message transaction
type MessageTransaction struct {
    BaseTransaction
    Message string `json:"message"`
}

// NewMessageTransaction creates a new message transaction
func NewMessageTransaction(sender, recipient, message string, fee float64) *MessageTransaction {
    return &MessageTransaction{
        BaseTransaction: *NewBaseTransaction(sender, recipient, 0, fee, "MESSAGE"),
        Message:         message,
    }
}

// CoinbaseTransaction represents a coinbase transaction (mining reward)
type CoinbaseTransaction struct {
    BaseTransaction
    TokenCount int64 `json:"token_count"`
}

// NewCoinbaseTransaction creates a new coinbase transaction
func NewCoinbaseTransaction(recipient string, tokenCount int64) *CoinbaseTransaction {
    return &CoinbaseTransaction{
        BaseTransaction: *NewBaseTransaction("coinbase", recipient, 0, 0, "COINBASE"),
        TokenCount:      tokenCount,
    }
}

// Override Validate for coinbase transactions
func (ct *CoinbaseTransaction) Validate() error {
    if ct.TokenCount <= 0 {
        return fmt.Errorf("token count must be positive")
    }
    
    if ct.Recipient == "" {
        return fmt.Errorf("recipient cannot be empty")
    }
    
    return nil
}
```

---

## 👛 Wallet System Design

### **Wallet Structure**

```go
package wallet

import (
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "time"
)

// Wallet represents a blockchain wallet
type Wallet struct {
    ID              string            `json:"id"`
    Address         string            `json:"address"`
    PublicKey       []byte            `json:"public_key"`
    PrivateKey      []byte            `json:"private_key,omitempty"` // Encrypted in production
    Balance         float64           `json:"balance"`
    CreatedAt       time.Time         `json:"created_at"`
    LastUpdated     time.Time         `json:"last_updated"`
    TransactionCount int              `json:"transaction_count"`
    Metadata        map[string]string `json:"metadata"`
}

// NewWallet creates a new wallet
func NewWallet() (*Wallet, error) {
    // Generate key pair
    privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil {
        return nil, fmt.Errorf("failed to generate key pair: %w", err)
    }
    
    // Generate address from public key
    address := generateAddress(privateKey.PublicKey)
    
    wallet := &Wallet{
        ID:              generateWalletID(),
        Address:         address,
        PublicKey:       publicKeyToBytes(privateKey.PublicKey),
        PrivateKey:      privateKeyToBytes(privateKey),
        Balance:         0.0,
        CreatedAt:       time.Now(),
        LastUpdated:     time.Now(),
        TransactionCount: 0,
        Metadata:        make(map[string]string),
    }
    
    return wallet, nil
}

// generateWalletID generates a unique wallet ID
func generateWalletID() string {
    data := fmt.Sprintf("%d", time.Now().UnixNano())
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:16]) // 16 bytes = 32 hex chars
}

// generateAddress generates an address from a public key
func generateAddress(publicKey ecdsa.PublicKey) string {
    // In a real implementation, this would use RIPEMD-160 and base58 encoding
    // For simplicity, we'll use SHA-256
    pubBytes := publicKeyToBytes(publicKey)
    hash := sha256.Sum256(pubBytes)
    return hex.EncodeToString(hash[:])
}

// publicKeyToBytes converts ECDSA public key to bytes
func publicKeyToBytes(publicKey ecdsa.PublicKey) []byte {
    return append(publicKey.X.Bytes(), publicKey.Y.Bytes()...)
}

// privateKeyToBytes converts ECDSA private key to bytes
func privateKeyToBytes(privateKey *ecdsa.PrivateKey) []byte {
    return privateKey.D.Bytes()
}

// GetBalance returns the current balance
func (w *Wallet) GetBalance() float64 {
    return w.Balance
}

// UpdateBalance updates the wallet balance
func (w *Wallet) UpdateBalance(amount float64) {
    w.Balance += amount
    w.LastUpdated = time.Now()
}

// IncrementTransactionCount increments the transaction count
func (w *Wallet) IncrementTransactionCount() {
    w.TransactionCount++
    w.LastUpdated = time.Now()
}

// AddMetadata adds metadata to the wallet
func (w *Wallet) AddMetadata(key, value string) {
    w.Metadata[key] = value
    w.LastUpdated = time.Now()
}

// GetMetadata retrieves metadata from the wallet
func (w *Wallet) GetMetadata(key string) (string, bool) {
    value, exists := w.Metadata[key]
    return value, exists
}

// ToJSON serializes the wallet to JSON
func (w *Wallet) ToJSON() ([]byte, error) {
    return json.MarshalIndent(w, "", "  ")
}

// FromJSON deserializes JSON to a wallet
func (w *Wallet) FromJSON(data []byte) error {
    return json.Unmarshal(data, w)
}

// Validate validates the wallet
func (w *Wallet) Validate() error {
    if w.ID == "" {
        return fmt.Errorf("wallet ID cannot be empty")
    }
    
    if w.Address == "" {
        return fmt.Errorf("wallet address cannot be empty")
    }
    
    if len(w.PublicKey) == 0 {
        return fmt.Errorf("public key cannot be empty")
    }
    
    if w.Balance < 0 {
        return fmt.Errorf("balance cannot be negative")
    }
    
    return nil
}
```

---

## 🆔 PUID (Persistent Unique ID) System

### **PUID Implementation**

```go
package puid

import (
    "crypto/rand"
    "encoding/hex"
    "fmt"
    "math/big"
    "time"
)

// PUID represents a persistent unique identifier
type PUID struct {
    ID        string    `json:"id"`
    Type      string    `json:"type"`
    CreatedAt time.Time `json:"created_at"`
    Metadata  map[string]interface{} `json:"metadata"`
}

// NewPUID creates a new PUID
func NewPUID(puidType string) *PUID {
    return &PUID{
        ID:        generatePUID(),
        Type:      puidType,
        CreatedAt: time.Now(),
        Metadata:  make(map[string]interface{}),
    }
}

// generatePUID generates a unique identifier
func generatePUID() string {
    // Generate 16 random bytes
    bytes := make([]byte, 16)
    rand.Read(bytes)
    
    // Add timestamp for uniqueness
    timestamp := time.Now().UnixNano()
    timestampBytes := []byte(fmt.Sprintf("%d", timestamp))
    
    // Combine random bytes and timestamp
    combined := append(bytes, timestampBytes...)
    
    // Create hash
    hash := sha256.Sum256(combined)
    return hex.EncodeToString(hash[:])
}

// GetID returns the PUID string
func (p *PUID) GetID() string {
    return p.ID
}

// GetType returns the PUID type
func (p *PUID) GetType() string {
    return p.Type
}

// AddMetadata adds metadata to the PUID
func (p *PUID) AddMetadata(key string, value interface{}) {
    p.Metadata[key] = value
}

// GetMetadata retrieves metadata from the PUID
func (p *PUID) GetMetadata(key string) (interface{}, bool) {
    value, exists := p.Metadata[key]
    return value, exists
}

// Validate validates the PUID
func (p *PUID) Validate() error {
    if p.ID == "" {
        return fmt.Errorf("PUID ID cannot be empty")
    }
    
    if p.Type == "" {
        return fmt.Errorf("PUID type cannot be empty")
    }
    
    return nil
}

// ToJSON serializes the PUID to JSON
func (p *PUID) ToJSON() ([]byte, error) {
    return json.MarshalIndent(p, "", "  ")
}

// FromJSON deserializes JSON to a PUID
func (p *PUID) FromJSON(data []byte) error {
    return json.Unmarshal(data, p)
}
```

---

## 🎯 Section Summary

In this section, you've learned:

✅ **Block Structure**: Enhanced block design with header/body separation
✅ **Transaction System**: Flexible interface with multiple transaction types
✅ **Wallet Implementation**: Complete wallet system with key management
✅ **PUID System**: Persistent unique identifier system
✅ **Merkle Trees**: Efficient transaction verification
✅ **Data Validation**: Comprehensive validation for all structures

### **Key Skills Developed**

1. **Data Structure Design**: Creating efficient, secure blockchain data structures
2. **Interface Implementation**: Building flexible, extensible transaction system
3. **Cryptographic Integration**: Proper key management and address generation
4. **Validation Systems**: Comprehensive validation for data integrity
5. **Serialization**: JSON handling for persistence and communication
6. **Merkle Trees**: Efficient transaction verification and integrity

### **Next Steps**

1. Complete the hands-on exercises below
2. Take the quiz to test your understanding
3. Move on to [Section 5: Basic Blockchain Implementation](../section5/README.md)

---

## 🛠️ Hands-On Exercises

### **Exercise 1: Enhanced Block Implementation**

Create an enhanced block implementation with:
1. Merkle tree for transaction verification
2. Block size calculation
3. Difficulty adjustment
4. Version control

### **Exercise 2: Transaction Type System**

Implement a complete transaction type system:
1. Multiple transaction types (Bank, Message, Coinbase, Contract)
2. Type-specific validation rules
3. Transaction factory pattern
4. Serialization/deserialization

### **Exercise 3: Advanced Wallet System**

Build an advanced wallet system:
1. Key derivation from mnemonic phrases
2. Multiple address types
3. Balance tracking and history
4. Transaction signing and verification

### **Exercise 4: PUID System Integration**

Integrate PUID system with blockchain components:
1. Unique identifiers for all entities
2. Metadata management
3. PUID validation and verification
4. Cross-reference system

### **Exercise 5: Data Structure Testing**

Create comprehensive tests for all data structures:
1. Unit tests for each component
2. Integration tests for interactions
3. Performance benchmarks
4. Edge case handling

---

## 📝 Quiz

Ready to test your knowledge? Take the [Section 4 Quiz](./quiz.md) to verify your understanding of core data structures.

---

**Excellent work! You've mastered the core data structures needed for blockchain development. You're ready to build your first working blockchain in [Section 5](../section5/README.md)! 🚀**
