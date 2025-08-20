# Section 2: Go Fundamentals for Blockchain

## 🚀 Mastering Go for Blockchain Development

Welcome to Section 2! This section focuses on essential Go programming concepts that are crucial for blockchain development. You'll learn the fundamentals of Go syntax, data structures, and programming patterns that will form the foundation of your blockchain implementation.

### **What You'll Learn in This Section**

- Go basics review (structs, interfaces, goroutines, channels)
- Cryptographic libraries in Go
- JSON handling and serialization
- File I/O and persistence
- Error handling patterns
- Testing in Go

### **Section Overview**

This section builds upon your basic Go knowledge and introduces advanced concepts that are essential for building a robust blockchain system. We'll focus on practical examples that directly relate to blockchain development.

---

## 📚 Go Basics Review

### **Structs and Methods**

Structs are fundamental to Go programming and are heavily used in blockchain development for representing blocks, transactions, and other data structures.

#### **Basic Struct Definition**

```go
// Block represents a basic blockchain block
type Block struct {
    Index     int
    Timestamp time.Time
    Data      string
    Hash      string
}

// Method to calculate hash for a block
func (b *Block) CalculateHash() string {
    // Implementation here
    return hash
}

// Method to validate a block
func (b *Block) IsValid() bool {
    // Validation logic here
    return true
}
```

#### **Struct Composition**

```go
// Header contains block metadata
type Header struct {
    Index        int
    Timestamp    time.Time
    PreviousHash string
    Nonce        int
}

// Block with composed header
type Block struct {
    Header      Header
    Transactions []Transaction
    Hash        string
}
```

### **Interfaces**

Interfaces are crucial for creating flexible, testable code in blockchain systems.

#### **Transaction Interface**

```go
// Transaction interface defines what a transaction must implement
type Transaction interface {
    GetID() string
    GetSender() string
    GetRecipient() string
    GetAmount() float64
    Validate() error
    Sign(privateKey []byte) error
    Verify(publicKey []byte) bool
}

// BankTransaction implements Transaction interface
type BankTransaction struct {
    ID        string
    Sender    string
    Recipient string
    Amount    float64
    Signature []byte
}

func (bt *BankTransaction) GetID() string {
    return bt.ID
}

func (bt *BankTransaction) GetSender() string {
    return bt.Sender
}

func (bt *BankTransaction) GetRecipient() string {
    return bt.Recipient
}

func (bt *BankTransaction) GetAmount() float64 {
    return bt.Amount
}

func (bt *BankTransaction) Validate() error {
    if bt.Amount <= 0 {
        return errors.New("amount must be positive")
    }
    if bt.Sender == "" || bt.Recipient == "" {
        return errors.New("sender and recipient cannot be empty")
    }
    return nil
}

func (bt *BankTransaction) Sign(privateKey []byte) error {
    // Signing implementation
    return nil
}

func (bt *BankTransaction) Verify(publicKey []byte) bool {
    // Verification implementation
    return true
}
```

### **Goroutines and Channels**

Concurrency is essential for blockchain operations like mining, network communication, and transaction processing.

#### **Basic Goroutine Example**

```go
// Mining goroutine
func (bc *Blockchain) StartMining() {
    go func() {
        for {
            select {
            case <-bc.stopMining:
                return
            default:
                bc.mineNextBlock()
                time.Sleep(time.Second)
            }
        }
    }()
}

// Transaction processing goroutine
func (bc *Blockchain) ProcessTransactions() {
    go func() {
        for tx := range bc.txChannel {
            if err := bc.validateTransaction(tx); err != nil {
                log.Printf("Invalid transaction: %v", err)
                continue
            }
            bc.addTransactionToPool(tx)
        }
    }()
}
```

#### **Channel Communication**

```go
// Blockchain with channels for communication
type Blockchain struct {
    blocks     []*Block
    txChannel  chan Transaction
    stopMining chan bool
    mu         sync.RWMutex
}

// Add transaction through channel
func (bc *Blockchain) AddTransaction(tx Transaction) {
    bc.txChannel <- tx
}

// Stop mining
func (bc *Blockchain) Stop() {
    close(bc.stopMining)
}
```

---

## 🔐 Cryptographic Libraries in Go

### **Hash Functions**

Cryptographic hashing is fundamental to blockchain security.

#### **SHA-256 Hashing**

```go
import (
    "crypto/sha256"
    "encoding/hex"
    "fmt"
)

// Calculate SHA-256 hash
func calculateHash(data string) string {
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

// Hash a block
func (b *Block) CalculateHash() string {
    data := fmt.Sprintf("%d%s%s%s", 
        b.Index, 
        b.Timestamp.Format(time.RFC3339), 
        b.Data, 
        b.PreviousHash)
    return calculateHash(data)
}
```

#### **RIPEMD-160 (for Bitcoin-style addresses)**

```go
import (
    "golang.org/x/crypto/ripemd160"
)

// Calculate RIPEMD-160 hash
func calculateRIPEMD160(data []byte) []byte {
    hasher := ripemd160.New()
    hasher.Write(data)
    return hasher.Sum(nil)
}
```

### **Digital Signatures**

Digital signatures are essential for transaction security.

#### **ECDSA Signatures**

```go
import (
    "crypto/ecdsa"
    "crypto/elliptic"
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "math/big"
)

// Generate key pair
func generateKeyPair() (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
    privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil {
        return nil, nil, err
    }
    return privateKey, &privateKey.PublicKey, nil
}

// Sign data
func signData(privateKey *ecdsa.PrivateKey, data []byte) ([]byte, error) {
    hash := sha256.Sum256(data)
    r, s, err := ecdsa.Sign(rand.Reader, privateKey, hash[:])
    if err != nil {
        return nil, err
    }
    
    // Combine r and s into signature
    signature := append(r.Bytes(), s.Bytes()...)
    return signature, nil
}

// Verify signature
func verifySignature(publicKey *ecdsa.PublicKey, data, signature []byte) bool {
    hash := sha256.Sum256(data)
    
    // Split signature into r and s
    sigLen := len(signature)
    r := new(big.Int).SetBytes(signature[:sigLen/2])
    s := new(big.Int).SetBytes(signature[sigLen/2:])
    
    return ecdsa.Verify(publicKey, hash[:], r, s)
}
```

### **Key Derivation**

```go
import (
    "crypto/rand"
    "encoding/hex"
)

// Generate random bytes
func generateRandomBytes(length int) ([]byte, error) {
    bytes := make([]byte, length)
    _, err := rand.Read(bytes)
    return bytes, err
}

// Generate wallet address
func generateWalletAddress() string {
    bytes, err := generateRandomBytes(32)
    if err != nil {
        return ""
    }
    return hex.EncodeToString(bytes)
}
```

---

## 📄 JSON Handling and Serialization

### **Struct Tags for JSON**

```go
// Block with JSON tags
type Block struct {
    Index        int       `json:"index"`
    Timestamp    time.Time `json:"timestamp"`
    Data         string    `json:"data"`
    PreviousHash string    `json:"previous_hash"`
    Hash         string    `json:"hash"`
    Nonce        int       `json:"nonce"`
}

// Transaction with JSON tags
type Transaction struct {
    ID        string  `json:"id"`
    Sender    string  `json:"sender"`
    Recipient string  `json:"recipient"`
    Amount    float64 `json:"amount"`
    Fee       float64 `json:"fee"`
    Signature []byte  `json:"signature,omitempty"`
}
```

### **JSON Marshaling and Unmarshaling**

```go
import (
    "encoding/json"
    "fmt"
)

// Serialize block to JSON
func (b *Block) ToJSON() ([]byte, error) {
    return json.Marshal(b)
}

// Deserialize block from JSON
func BlockFromJSON(data []byte) (*Block, error) {
    var block Block
    err := json.Unmarshal(data, &block)
    if err != nil {
        return nil, err
    }
    return &block, nil
}

// Serialize blockchain
func (bc *Blockchain) ToJSON() ([]byte, error) {
    return json.MarshalIndent(bc.Blocks, "", "  ")
}

// Example usage
func main() {
    block := &Block{
        Index:        1,
        Timestamp:    time.Now(),
        Data:         "Hello Blockchain",
        PreviousHash: "0000000000000000",
        Hash:         "abc123...",
        Nonce:        42,
    }
    
    // Serialize
    jsonData, err := block.ToJSON()
    if err != nil {
        log.Fatal(err)
    }
    fmt.Println(string(jsonData))
    
    // Deserialize
    newBlock, err := BlockFromJSON(jsonData)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Deserialized block: %+v\n", newBlock)
}
```

---

## 💾 File I/O and Persistence

### **Saving Blocks to Disk**

```go
import (
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
)

// Save block to file
func (b *Block) SaveToFile(dataDir string) error {
    // Create data directory if it doesn't exist
    if err := os.MkdirAll(dataDir, 0755); err != nil {
        return fmt.Errorf("failed to create directory: %v", err)
    }
    
    // Create filename
    filename := filepath.Join(dataDir, fmt.Sprintf("%d.json", b.Index))
    
    // Serialize block
    data, err := json.MarshalIndent(b, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal block: %v", err)
    }
    
    // Write to file
    if err := os.WriteFile(filename, data, 0644); err != nil {
        return fmt.Errorf("failed to write file: %v", err)
    }
    
    return nil
}

// Load block from file
func LoadBlockFromFile(dataDir string, index int) (*Block, error) {
    filename := filepath.Join(dataDir, fmt.Sprintf("%d.json", index))
    
    data, err := os.ReadFile(filename)
    if err != nil {
        return nil, fmt.Errorf("failed to read file: %v", err)
    }
    
    var block Block
    if err := json.Unmarshal(data, &block); err != nil {
        return nil, fmt.Errorf("failed to unmarshal block: %v", err)
    }
    
    return &block, nil
}
```

### **Loading Blockchain from Disk**

```go
// Load all blocks from disk
func (bc *Blockchain) LoadFromDisk(dataDir string) error {
    // Get all JSON files in the data directory
    pattern := filepath.Join(dataDir, "*.json")
    files, err := filepath.Glob(pattern)
    if err != nil {
        return fmt.Errorf("failed to glob files: %v", err)
    }
    
    // Sort files by block index
    sort.Strings(files)
    
    // Load each block
    for _, file := range files {
        data, err := os.ReadFile(file)
        if err != nil {
            log.Printf("Failed to read file %s: %v", file, err)
            continue
        }
        
        var block Block
        if err := json.Unmarshal(data, &block); err != nil {
            log.Printf("Failed to unmarshal block from %s: %v", file, err)
            continue
        }
        
        bc.Blocks = append(bc.Blocks, &block)
    }
    
    return nil
}
```

---

## ⚠️ Error Handling Patterns

### **Custom Error Types**

```go
// Custom error types for blockchain
type BlockchainError struct {
    Code    string
    Message string
    Err     error
}

func (e *BlockchainError) Error() string {
    if e.Err != nil {
        return fmt.Sprintf("[%s] %s: %v", e.Code, e.Message, e.Err)
    }
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

func (e *BlockchainError) Unwrap() error {
    return e.Err
}

// Error codes
const (
    ErrInvalidBlock     = "INVALID_BLOCK"
    ErrInvalidHash      = "INVALID_HASH"
    ErrInvalidSignature = "INVALID_SIGNATURE"
    ErrBlockNotFound    = "BLOCK_NOT_FOUND"
    ErrChainCorrupted   = "CHAIN_CORRUPTED"
)

// Error constructors
func NewInvalidBlockError(message string, err error) *BlockchainError {
    return &BlockchainError{
        Code:    ErrInvalidBlock,
        Message: message,
        Err:     err,
    }
}

func NewInvalidHashError(message string, err error) *BlockchainError {
    return &BlockchainError{
        Code:    ErrInvalidHash,
        Message: message,
        Err:     err,
    }
}
```

### **Error Handling in Functions**

```go
// Validate block with proper error handling
func (b *Block) Validate() error {
    // Check if block has required fields
    if b.Index < 0 {
        return NewInvalidBlockError("block index cannot be negative", nil)
    }
    
    if b.Data == "" {
        return NewInvalidBlockError("block data cannot be empty", nil)
    }
    
    // Validate hash
    expectedHash := b.CalculateHash()
    if b.Hash != expectedHash {
        return NewInvalidHashError(
            fmt.Sprintf("invalid hash: expected %s, got %s", expectedHash, b.Hash),
            nil,
        )
    }
    
    return nil
}

// Add block with error handling
func (bc *Blockchain) AddBlock(block *Block) error {
    // Validate block
    if err := block.Validate(); err != nil {
        return fmt.Errorf("failed to validate block: %w", err)
    }
    
    // Check if block is next in sequence
    expectedIndex := len(bc.Blocks)
    if block.Index != expectedIndex {
        return NewInvalidBlockError(
            fmt.Sprintf("block index mismatch: expected %d, got %d", expectedIndex, block.Index),
            nil,
        )
    }
    
    // Check previous hash
    if len(bc.Blocks) > 0 {
        lastBlock := bc.Blocks[len(bc.Blocks)-1]
        if block.PreviousHash != lastBlock.Hash {
            return NewInvalidBlockError(
                fmt.Sprintf("previous hash mismatch: expected %s, got %s", lastBlock.Hash, block.PreviousHash),
                nil,
            )
        }
    }
    
    // Add block
    bc.Blocks = append(bc.Blocks, block)
    
    // Save to disk
    if err := block.SaveToFile("data/blocks"); err != nil {
        return fmt.Errorf("failed to save block: %w", err)
    }
    
    return nil
}
```

---

## 🧪 Testing in Go

### **Unit Testing**

```go
// block_test.go
package blockchain

import (
    "testing"
    "time"
)

// Test block creation
func TestNewBlock(t *testing.T) {
    data := "Test block data"
    previousHash := "0000000000000000"
    
    block := NewBlock(1, data, previousHash)
    
    if block.Index != 1 {
        t.Errorf("Expected index 1, got %d", block.Index)
    }
    
    if block.Data != data {
        t.Errorf("Expected data %s, got %s", data, block.Data)
    }
    
    if block.PreviousHash != previousHash {
        t.Errorf("Expected previous hash %s, got %s", previousHash, block.PreviousHash)
    }
    
    if block.Hash == "" {
        t.Error("Expected non-empty hash")
    }
}

// Test block validation
func TestBlockValidation(t *testing.T) {
    tests := []struct {
        name    string
        block   *Block
        wantErr bool
    }{
        {
            name: "valid block",
            block: &Block{
                Index:        1,
                Timestamp:    time.Now(),
                Data:         "valid data",
                PreviousHash: "0000000000000000",
                Hash:         "validhash",
            },
            wantErr: false,
        },
        {
            name: "invalid index",
            block: &Block{
                Index:        -1,
                Timestamp:    time.Now(),
                Data:         "valid data",
                PreviousHash: "0000000000000000",
                Hash:         "validhash",
            },
            wantErr: true,
        },
        {
            name: "empty data",
            block: &Block{
                Index:        1,
                Timestamp:    time.Now(),
                Data:         "",
                PreviousHash: "0000000000000000",
                Hash:         "validhash",
            },
            wantErr: true,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            err := tt.block.Validate()
            if (err != nil) != tt.wantErr {
                t.Errorf("Block.Validate() error = %v, wantErr %v", err, tt.wantErr)
            }
        })
    }
}

// Test blockchain operations
func TestBlockchain(t *testing.T) {
    bc := NewBlockchain()
    
    // Test adding blocks
    block1 := NewBlock(0, "Genesis Block", "")
    if err := bc.AddBlock(block1); err != nil {
        t.Errorf("Failed to add genesis block: %v", err)
    }
    
    block2 := NewBlock(1, "Second Block", block1.Hash)
    if err := bc.AddBlock(block2); err != nil {
        t.Errorf("Failed to add second block: %v", err)
    }
    
    // Test blockchain length
    if len(bc.Blocks) != 2 {
        t.Errorf("Expected 2 blocks, got %d", len(bc.Blocks))
    }
    
    // Test chain validation
    if !bc.IsValid() {
        t.Error("Blockchain should be valid")
    }
}
```

### **Benchmark Testing**

```go
// Benchmark block hash calculation
func BenchmarkBlockHash(b *testing.B) {
    block := &Block{
        Index:        1,
        Timestamp:    time.Now(),
        Data:         "Benchmark test data",
        PreviousHash: "0000000000000000",
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        block.CalculateHash()
    }
}

// Benchmark blockchain validation
func BenchmarkBlockchainValidation(b *testing.B) {
    bc := NewBlockchain()
    
    // Add some blocks
    for i := 0; i < 100; i++ {
        block := NewBlock(i, fmt.Sprintf("Block %d", i), "")
        if i > 0 {
            block.PreviousHash = bc.Blocks[i-1].Hash
        }
        bc.AddBlock(block)
    }
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        bc.IsValid()
    }
}
```

---

## 🎯 Section Summary

In this section, you've learned:

✅ **Go Fundamentals**: Structs, interfaces, goroutines, and channels
✅ **Cryptographic Operations**: Hashing, digital signatures, and key generation
✅ **Data Serialization**: JSON handling for blockchain data
✅ **File I/O**: Persistence and loading of blockchain data
✅ **Error Handling**: Robust error handling patterns for blockchain systems
✅ **Testing**: Unit testing and benchmarking for blockchain components

### **Key Skills Developed**

1. **Data Structure Design**: Creating efficient structs for blockchain components
2. **Interface Implementation**: Building flexible, testable code
3. **Concurrency**: Using goroutines and channels for blockchain operations
4. **Cryptography**: Implementing security features for blockchain
5. **Persistence**: Saving and loading blockchain data
6. **Error Management**: Handling errors gracefully in blockchain systems
7. **Testing**: Ensuring code quality through comprehensive testing

### **Next Steps**

1. Complete the hands-on exercises below
2. Take the quiz to test your understanding
3. Move on to [Section 3: Blockchain Fundamentals](../section3/README.md)

---

## 🛠️ Hands-On Exercises

### **Exercise 1: Enhanced Block Structure**

Create a more sophisticated block structure with:
1. Header and body separation
2. Merkle root for transactions
3. Difficulty target
4. Block size calculation

### **Exercise 2: Transaction System**

Implement a complete transaction system with:
1. Multiple transaction types (Bank, Message, Coinbase)
2. Transaction validation
3. Digital signatures
4. Transaction pool management

### **Exercise 3: Cryptographic Wallet**

Build a wallet system with:
1. Key pair generation
2. Address derivation
3. Transaction signing
4. Balance calculation

### **Exercise 4: Persistence Layer**

Create a robust persistence system with:
1. Block storage and retrieval
2. Transaction storage
3. Wallet storage
4. Chain state persistence

### **Exercise 5: Testing Suite**

Develop comprehensive tests for:
1. All blockchain components
2. Edge cases and error conditions
3. Performance benchmarks
4. Integration tests

---

## 📝 Quiz

Ready to test your knowledge? Take the [Section 2 Quiz](./quiz.md) to verify your understanding of Go fundamentals for blockchain development.

---

**Excellent work! You've mastered the Go fundamentals needed for blockchain development. You're now ready to dive into blockchain theory and implementation in [Section 3](../section3/README.md)! 🚀**
