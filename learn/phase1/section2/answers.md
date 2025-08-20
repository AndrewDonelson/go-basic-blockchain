# Section 2 Quiz Answers

## 📋 Answer Key

Here are the correct answers and explanations for the Section 2 quiz.

---

## **Multiple Choice Questions**

### **Question 1: Go Structs**
**Answer: C) `func (s *StructName) methodName() returnType { }`**

**Explanation**: In Go, methods are defined with a receiver parameter. The `*` indicates a pointer receiver, which is commonly used when you want to modify the struct or for efficiency with larger structs.

### **Question 2: Interfaces**
**Answer: B) They allow for polymorphism and code flexibility**

**Explanation**: Interfaces in Go provide polymorphism by allowing different types to be used interchangeably as long as they implement the required methods. This enables flexible, testable code.

### **Question 3: Goroutines**
**Answer: A) `go functionName()`**

**Explanation**: The `go` keyword is used to start a goroutine in Go. It's a simple prefix that launches the function in a concurrent goroutine.

### **Question 4: Channels**
**Answer: A) 0**

**Explanation**: Unbuffered channels have a capacity of 0, meaning they block until both sender and receiver are ready (synchronous communication).

### **Question 5: Cryptographic Hashing**
**Answer: B) `crypto/sha256`**

**Explanation**: The `crypto/sha256` package provides SHA-256 hashing functionality in Go's standard library.

### **Question 6: JSON Tags**
**Answer: B) It omits the field from JSON output if it's empty**

**Explanation**: The `omitempty` tag tells the JSON encoder to skip the field if it has a zero value (empty string, 0, nil, etc.).

### **Question 7: Error Handling**
**Answer: B) Returning error values and checking them**

**Explanation**: Go's idiomatic error handling involves returning error values from functions and explicitly checking them, rather than using exceptions.

### **Question 8: File I/O**
**Answer: A) `os.ReadFile()`**

**Explanation**: `os.ReadFile()` is the modern way to read an entire file into memory in Go (since Go 1.16). `ioutil.ReadFile()` is deprecated.

---

## **True/False Questions**

### **Question 9**
**Answer: True**

**Explanation**: Go uses implicit interface implementation. A type automatically implements an interface if it has all the required methods, without any explicit declaration.

### **Question 10**
**Answer: True**

**Explanation**: Goroutines are managed by the Go runtime scheduler, not the operating system. They're much more lightweight than OS threads.

### **Question 11**
**Answer: False**

**Explanation**: Channels can be used for communication between any parts of a Go program, not just goroutines. They're often used with goroutines but not exclusively.

### **Question 12**
**Answer: True**

**Explanation**: The `crypto/rand` package provides cryptographically secure random number generation, which is essential for cryptographic operations.

### **Question 13**
**Answer: True**

**Explanation**: `json.Marshal()` converts structs to JSON, while `json.Unmarshal()` converts JSON back to structs. They are complementary functions.

### **Question 14**
**Answer: True**

**Explanation**: The `error` interface requires an `Error() string` method. Custom error types must implement this method to satisfy the interface.

---

## **Practical Questions**

### **Question 15: Struct and Interface Implementation**

```go
package main

import (
    "errors"
    "fmt"
)

// Transaction interface defines what a transaction must implement
type Transaction interface {
    GetAmount() float64
    GetSender() string
    GetRecipient() string
    Validate() error
}

// Wallet represents a blockchain wallet
type Wallet struct {
    Address string
    Balance float64
}

// NewWallet creates a new wallet
func NewWallet(address string, initialBalance float64) *Wallet {
    return &Wallet{
        Address: address,
        Balance: initialBalance,
    }
}

// Send creates a transaction to send funds
func (w *Wallet) Send(recipient string, amount float64) (*SimpleTransaction, error) {
    if amount <= 0 {
        return nil, errors.New("amount must be positive")
    }
    
    if amount > w.Balance {
        return nil, errors.New("insufficient balance")
    }
    
    // Deduct from sender's balance
    w.Balance -= amount
    
    // Create transaction
    tx := &SimpleTransaction{
        Sender:    w.Address,
        Recipient: recipient,
        Amount:    amount,
    }
    
    return tx, nil
}

// Receive adds funds to the wallet
func (w *Wallet) Receive(amount float64) {
    if amount > 0 {
        w.Balance += amount
    }
}

// GetBalance returns the current balance
func (w *Wallet) GetBalance() float64 {
    return w.Balance
}

// SimpleTransaction implements the Transaction interface
type SimpleTransaction struct {
    Sender    string
    Recipient string
    Amount    float64
}

func (tx *SimpleTransaction) GetAmount() float64 {
    return tx.Amount
}

func (tx *SimpleTransaction) GetSender() string {
    return tx.Sender
}

func (tx *SimpleTransaction) GetRecipient() string {
    return tx.Recipient
}

func (tx *SimpleTransaction) Validate() error {
    if tx.Amount <= 0 {
        return errors.New("transaction amount must be positive")
    }
    if tx.Sender == "" || tx.Recipient == "" {
        return errors.New("sender and recipient cannot be empty")
    }
    return nil
}

func main() {
    // Create wallets
    alice := NewWallet("alice123", 100.0)
    bob := NewWallet("bob456", 50.0)
    
    fmt.Printf("Alice's balance: $%.2f\n", alice.GetBalance())
    fmt.Printf("Bob's balance: $%.2f\n", bob.GetBalance())
    
    // Alice sends money to Bob
    tx, err := alice.Send("bob456", 25.0)
    if err != nil {
        fmt.Printf("Error: %v\n", err)
        return
    }
    
    // Bob receives the money
    bob.Receive(tx.GetAmount())
    
    fmt.Printf("After transaction:\n")
    fmt.Printf("Alice's balance: $%.2f\n", alice.GetBalance())
    fmt.Printf("Bob's balance: $%.2f\n", bob.GetBalance())
}
```

### **Question 16: Goroutine and Channel Usage**

```go
package main

import (
    "fmt"
    "time"
)

// Transaction represents a simple transaction
type Transaction struct {
    ID     string
    Amount float64
    From   string
    To     string
}

// TransactionProcessor processes transactions
type TransactionProcessor struct {
    txChannel chan Transaction
    done      chan bool
}

// NewTransactionProcessor creates a new processor
func NewTransactionProcessor() *TransactionProcessor {
    return &TransactionProcessor{
        txChannel: make(chan Transaction, 10), // Buffered channel
        done:      make(chan bool),
    }
}

// Start begins processing transactions
func (tp *TransactionProcessor) Start() {
    go func() {
        for {
            select {
            case tx := <-tp.txChannel:
                tp.processTransaction(tx)
            case <-tp.done:
                fmt.Println("Transaction processor stopped")
                return
            }
        }
    }()
}

// Stop stops the processor
func (tp *TransactionProcessor) Stop() {
    close(tp.done)
}

// AddTransaction adds a transaction to the processing queue
func (tp *TransactionProcessor) AddTransaction(tx Transaction) {
    tp.txChannel <- tx
}

// processTransaction processes a single transaction
func (tp *TransactionProcessor) processTransaction(tx Transaction) {
    fmt.Printf("Processing transaction %s: $%.2f from %s to %s\n", 
        tx.ID, tx.Amount, tx.From, tx.To)
    
    // Simulate processing time
    time.Sleep(100 * time.Millisecond)
    
    fmt.Printf("Transaction %s completed successfully\n", tx.ID)
}

func main() {
    // Create transaction processor
    processor := NewTransactionProcessor()
    
    // Start processing
    processor.Start()
    
    // Add some transactions
    transactions := []Transaction{
        {ID: "tx1", Amount: 10.0, From: "Alice", To: "Bob"},
        {ID: "tx2", Amount: 25.0, From: "Bob", To: "Charlie"},
        {ID: "tx3", Amount: 5.0, From: "Charlie", To: "Alice"},
    }
    
    // Send transactions to processor
    for _, tx := range transactions {
        processor.AddTransaction(tx)
        time.Sleep(50 * time.Millisecond) // Small delay between transactions
    }
    
    // Wait for processing to complete
    time.Sleep(1 * time.Second)
    
    // Stop the processor
    processor.Stop()
    
    fmt.Println("All transactions processed!")
}
```

### **Question 17: Cryptographic Operations**

```go
package main

import (
    "crypto/rand"
    "crypto/sha256"
    "encoding/hex"
    "fmt"
)

// generateSHA256Hash generates a SHA-256 hash of the given string
func generateSHA256Hash(data string) string {
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

// generateWalletAddress generates a random wallet address
func generateWalletAddress() (string, error) {
    // Generate 32 random bytes
    bytes := make([]byte, 32)
    _, err := rand.Read(bytes)
    if err != nil {
        return "", fmt.Errorf("failed to generate random bytes: %v", err)
    }
    
    // Convert to hexadecimal string
    address := hex.EncodeToString(bytes)
    return address, nil
}

// generateMultipleAddresses generates multiple wallet addresses
func generateMultipleAddresses(count int) ([]string, error) {
    addresses := make([]string, count)
    
    for i := 0; i < count; i++ {
        address, err := generateWalletAddress()
        if err != nil {
            return nil, fmt.Errorf("failed to generate address %d: %v", i+1, err)
        }
        addresses[i] = address
    }
    
    return addresses, nil
}

func main() {
    // Test SHA-256 hashing
    testData := "Hello, Blockchain!"
    hash := generateSHA256Hash(testData)
    fmt.Printf("SHA-256 hash of '%s': %s\n", testData, hash)
    
    // Test wallet address generation
    address, err := generateWalletAddress()
    if err != nil {
        fmt.Printf("Error generating wallet address: %v\n", err)
        return
    }
    fmt.Printf("Generated wallet address: %s\n", address)
    
    // Generate multiple addresses
    addresses, err := generateMultipleAddresses(3)
    if err != nil {
        fmt.Printf("Error generating multiple addresses: %v\n", err)
        return
    }
    
    fmt.Println("Multiple wallet addresses:")
    for i, addr := range addresses {
        fmt.Printf("  %d: %s\n", i+1, addr)
    }
    
    // Verify hash consistency
    sameData := "Hello, Blockchain!"
    sameHash := generateSHA256Hash(sameData)
    fmt.Printf("Hash verification: %t\n", hash == sameHash)
}
```

### **Question 18: JSON Serialization**

```go
package main

import (
    "encoding/json"
    "fmt"
    "time"
)

// Block represents a blockchain block with JSON tags
type Block struct {
    Index        int       `json:"index"`
    Timestamp    time.Time `json:"timestamp"`
    Data         string    `json:"data"`
    PreviousHash string    `json:"previous_hash"`
    Hash         string    `json:"hash"`
    Nonce        int       `json:"nonce"`
}

// NewBlock creates a new block
func NewBlock(index int, data, previousHash string) *Block {
    return &Block{
        Index:        index,
        Timestamp:    time.Now(),
        Data:         data,
        PreviousHash: previousHash,
        Hash:         "", // Will be calculated later
        Nonce:        0,
    }
}

// ToJSON serializes the block to JSON
func (b *Block) ToJSON() ([]byte, error) {
    return json.MarshalIndent(b, "", "  ")
}

// FromJSON deserializes JSON to a block
func FromJSON(data []byte) (*Block, error) {
    var block Block
    err := json.Unmarshal(data, &block)
    if err != nil {
        return nil, err
    }
    return &block, nil
}

// Blockchain represents a collection of blocks
type Blockchain struct {
    Blocks []*Block `json:"blocks"`
}

// NewBlockchain creates a new blockchain
func NewBlockchain() *Blockchain {
    return &Blockchain{
        Blocks: []*Block{},
    }
}

// AddBlock adds a block to the blockchain
func (bc *Blockchain) AddBlock(block *Block) {
    bc.Blocks = append(bc.Blocks, block)
}

// ToJSON serializes the blockchain to JSON
func (bc *Blockchain) ToJSON() ([]byte, error) {
    return json.MarshalIndent(bc, "", "  ")
}

// FromJSON deserializes JSON to a blockchain
func BlockchainFromJSON(data []byte) (*Blockchain, error) {
    var blockchain Blockchain
    err := json.Unmarshal(data, &blockchain)
    if err != nil {
        return nil, err
    }
    return &blockchain, nil
}

func main() {
    // Create a blockchain
    blockchain := NewBlockchain()
    
    // Add some blocks
    genesisBlock := NewBlock(0, "Genesis Block", "")
    genesisBlock.Hash = "0000000000000000"
    
    block1 := NewBlock(1, "First Block", genesisBlock.Hash)
    block1.Hash = "abc123def456"
    block1.Nonce = 42
    
    blockchain.AddBlock(genesisBlock)
    blockchain.AddBlock(block1)
    
    // Serialize to JSON
    jsonData, err := blockchain.ToJSON()
    if err != nil {
        fmt.Printf("Error serializing blockchain: %v\n", err)
        return
    }
    
    fmt.Println("Serialized Blockchain:")
    fmt.Println(string(jsonData))
    
    // Deserialize from JSON
    newBlockchain, err := BlockchainFromJSON(jsonData)
    if err != nil {
        fmt.Printf("Error deserializing blockchain: %v\n", err)
        return
    }
    
    fmt.Printf("\nDeserialized blockchain has %d blocks\n", len(newBlockchain.Blocks))
    
    // Test individual block serialization
    blockJSON, err := genesisBlock.ToJSON()
    if err != nil {
        fmt.Printf("Error serializing block: %v\n", err)
        return
    }
    
    fmt.Println("\nSerialized Block:")
    fmt.Println(string(blockJSON))
    
    // Deserialize individual block
    newBlock, err := FromJSON(blockJSON)
    if err != nil {
        fmt.Printf("Error deserializing block: %v\n", err)
        return
    }
    
    fmt.Printf("\nDeserialized block index: %d\n", newBlock.Index)
    fmt.Printf("Deserialized block data: %s\n", newBlock.Data)
}
```

---

## **Bonus Challenge**

### **Question 19: Complete Blockchain Component**

```go
package main

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "errors"
    "fmt"
    "time"
)

// Block represents a blockchain block
type Block struct {
    Index        int       `json:"index"`
    Timestamp    time.Time `json:"timestamp"`
    Data         string    `json:"data"`
    PreviousHash string    `json:"previous_hash"`
    Hash         string    `json:"hash"`
    Nonce        int       `json:"nonce"`
}

// Blockchain represents a collection of blocks
type Blockchain struct {
    Blocks []*Block `json:"blocks"`
}

// BlockchainError represents blockchain-specific errors
type BlockchainError struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

func (e *BlockchainError) Error() string {
    return fmt.Sprintf("[%s] %s", e.Code, e.Message)
}

// NewBlock creates a new block
func NewBlock(index int, data, previousHash string) *Block {
    block := &Block{
        Index:        index,
        Timestamp:    time.Now(),
        Data:         data,
        PreviousHash: previousHash,
        Nonce:        0,
    }
    
    // Calculate initial hash
    block.Hash = block.calculateHash()
    
    return block
}

// calculateHash calculates the hash of the block
func (b *Block) calculateHash() string {
    data := fmt.Sprintf("%d%s%s%s%d", 
        b.Index, 
        b.Timestamp.Format(time.RFC3339), 
        b.Data, 
        b.PreviousHash, 
        b.Nonce)
    
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

// MineBlock mines the block with proof of work
func (b *Block) MineBlock(difficulty int) {
    target := ""
    for i := 0; i < difficulty; i++ {
        target += "0"
    }
    
    for {
        b.Hash = b.calculateHash()
        if b.Hash[:difficulty] == target {
            break
        }
        b.Nonce++
    }
}

// Validate validates the block
func (b *Block) Validate() error {
    if b.Index < 0 {
        return &BlockchainError{
            Code:    "INVALID_INDEX",
            Message: "block index cannot be negative",
        }
    }
    
    if b.Data == "" {
        return &BlockchainError{
            Code:    "EMPTY_DATA",
            Message: "block data cannot be empty",
        }
    }
    
    // Verify hash
    expectedHash := b.calculateHash()
    if b.Hash != expectedHash {
        return &BlockchainError{
            Code:    "INVALID_HASH",
            Message: fmt.Sprintf("invalid hash: expected %s, got %s", expectedHash, b.Hash),
        }
    }
    
    return nil
}

// NewBlockchain creates a new blockchain
func NewBlockchain() *Blockchain {
    return &Blockchain{
        Blocks: []*Block{},
    }
}

// AddBlock adds a block to the blockchain
func (bc *Blockchain) AddBlock(block *Block) error {
    // Validate block
    if err := block.Validate(); err != nil {
        return fmt.Errorf("block validation failed: %w", err)
    }
    
    // Check if block is next in sequence
    expectedIndex := len(bc.Blocks)
    if block.Index != expectedIndex {
        return &BlockchainError{
            Code:    "INDEX_MISMATCH",
            Message: fmt.Sprintf("block index mismatch: expected %d, got %d", expectedIndex, block.Index),
        }
    }
    
    // Check previous hash
    if len(bc.Blocks) > 0 {
        lastBlock := bc.Blocks[len(bc.Blocks)-1]
        if block.PreviousHash != lastBlock.Hash {
            return &BlockchainError{
                Code:    "PREVIOUS_HASH_MISMATCH",
                Message: fmt.Sprintf("previous hash mismatch: expected %s, got %s", lastBlock.Hash, block.PreviousHash),
            }
        }
    }
    
    // Add block
    bc.Blocks = append(bc.Blocks, block)
    
    return nil
}

// IsValid validates the entire blockchain
func (bc *Blockchain) IsValid() error {
    for i := 1; i < len(bc.Blocks); i++ {
        currentBlock := bc.Blocks[i]
        previousBlock := bc.Blocks[i-1]
        
        // Validate current block
        if err := currentBlock.Validate(); err != nil {
            return fmt.Errorf("block %d validation failed: %w", i, err)
        }
        
        // Check previous hash
        if currentBlock.PreviousHash != previousBlock.Hash {
            return &BlockchainError{
                Code:    "CHAIN_CORRUPTED",
                Message: fmt.Sprintf("chain corrupted at block %d", i),
            }
        }
    }
    
    return nil
}

// ToJSON serializes the blockchain to JSON
func (bc *Blockchain) ToJSON() ([]byte, error) {
    return json.MarshalIndent(bc, "", "  ")
}

// FromJSON deserializes JSON to a blockchain
func (bc *Blockchain) FromJSON(data []byte) error {
    return json.Unmarshal(data, bc)
}

// GetLatestBlock returns the latest block
func (bc *Blockchain) GetLatestBlock() *Block {
    if len(bc.Blocks) == 0 {
        return nil
    }
    return bc.Blocks[len(bc.Blocks)-1]
}

// GetBlockByIndex returns a block by index
func (bc *Blockchain) GetBlockByIndex(index int) *Block {
    if index < 0 || index >= len(bc.Blocks) {
        return nil
    }
    return bc.Blocks[index]
}

func main() {
    // Create a new blockchain
    blockchain := NewBlockchain()
    
    // Create genesis block
    genesisBlock := NewBlock(0, "Genesis Block", "")
    genesisBlock.MineBlock(2) // Mine with difficulty 2
    
    // Add genesis block
    if err := blockchain.AddBlock(genesisBlock); err != nil {
        fmt.Printf("Error adding genesis block: %v\n", err)
        return
    }
    
    // Create and add more blocks
    for i := 1; i <= 3; i++ {
        lastBlock := blockchain.GetLatestBlock()
        newBlock := NewBlock(i, fmt.Sprintf("Block %d Data", i), lastBlock.Hash)
        newBlock.MineBlock(2) // Mine with difficulty 2
        
        if err := blockchain.AddBlock(newBlock); err != nil {
            fmt.Printf("Error adding block %d: %v\n", i, err)
            return
        }
    }
    
    // Validate the blockchain
    if err := blockchain.IsValid(); err != nil {
        fmt.Printf("Blockchain validation failed: %v\n", err)
        return
    }
    
    fmt.Println("✅ Blockchain is valid!")
    
    // Display blockchain
    fmt.Printf("Blockchain has %d blocks:\n", len(blockchain.Blocks))
    for _, block := range blockchain.Blocks {
        fmt.Printf("Block #%d: %s (Hash: %s)\n", block.Index, block.Data, block.Hash)
    }
    
    // Serialize to JSON
    jsonData, err := blockchain.ToJSON()
    if err != nil {
        fmt.Printf("Error serializing blockchain: %v\n", err)
        return
    }
    
    fmt.Println("\nSerialized Blockchain:")
    fmt.Println(string(jsonData))
    
    // Test deserialization
    newBlockchain := NewBlockchain()
    if err := newBlockchain.FromJSON(jsonData); err != nil {
        fmt.Printf("Error deserializing blockchain: %v\n", err)
        return
    }
    
    fmt.Printf("\nDeserialized blockchain has %d blocks\n", len(newBlockchain.Blocks))
    
    // Validate deserialized blockchain
    if err := newBlockchain.IsValid(); err != nil {
        fmt.Printf("Deserialized blockchain validation failed: %v\n", err)
        return
    }
    
    fmt.Println("✅ Deserialized blockchain is valid!")
}
```

---

## **Scoring Your Quiz**

### **How to Calculate Your Score:**

1. **Multiple Choice**: Count correct answers × 2 points each
2. **True/False**: Count correct answers × 1 point each  
3. **Practical Questions**: Rate each answer 0-5 points based on completeness and accuracy
4. **Bonus Challenge**: Rate 0-10 points based on code completeness and functionality

### **Grade Interpretation:**

- **Excellent (90%+)**: 47+ points - You have mastered Go fundamentals for blockchain
- **Good (80-89%)**: 42-46 points - You understand the concepts well with minor gaps
- **Satisfactory (70-79%)**: 36-41 points - You have a basic understanding but need more practice
- **Needs Improvement (<70%)**: <36 points - Review the section material and retake the quiz

### **Next Steps Based on Your Score:**

- **90%+**: Excellent! You're ready for Section 3
- **80-89%**: Good work! Review any missed concepts before moving on
- **70-79%**: Spend more time on the hands-on exercises before proceeding
- **<70%**: Review the section material thoroughly and retake the quiz

---

**Great job completing Section 2! 🎉**

Ready for the next challenge? Move on to [Section 3: Blockchain Fundamentals](../section3/README.md)!
