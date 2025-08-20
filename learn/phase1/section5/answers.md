# Section 5 Quiz Answers

## 📋 Answer Key

Here are the correct answers and explanations for the Section 5 quiz.

---

## **Multiple Choice Questions**

### **Question 1: Genesis Block**
**Answer: B) To serve as the first block and establish the initial state**

**Explanation**: The genesis block is the first block in a blockchain and serves to establish the initial state of the system. It has no previous block to reference and typically contains initial transactions or configuration data.

### **Question 2: Block Mining**
**Answer: B) To adjust the block hash to meet difficulty requirements**

**Explanation**: The nonce (number used once) is incremented during mining to find a hash that meets the difficulty requirements (e.g., a certain number of leading zeros). This is the core of proof-of-work consensus.

### **Question 3: Transaction Queue**
**Answer: B) It is cleared and new transactions are added**

**Explanation**: After a block is successfully mined, the transaction queue is cleared as those transactions are now included in the block. New transactions can then be added to the queue for the next block.

### **Question 4: Chain Validation**
**Answer: C) Ensure blocks are linked by previous hash**

**Explanation**: The first step in blockchain validation is ensuring that each block's previous hash correctly references the hash of the previous block, maintaining the chain's integrity.

### **Question 5: Data Persistence**
**Answer: B) Data persistence across program restarts**

**Explanation**: Saving blockchain data to disk ensures that the blockchain state persists across program restarts, preventing data loss and allowing the system to resume from where it left off.

### **Question 6: Wallet Balance**
**Answer: B) By tracking UTXOs (Unspent Transaction Outputs)**

**Explanation**: In most blockchain systems, wallet balances are calculated by tracking UTXOs - unspent transaction outputs that belong to the wallet's address. This is more accurate than simply storing a balance field.

### **Question 7: Mining Difficulty**
**Answer: C) It increases to slow down mining**

**Explanation**: When blocks are mined too quickly, the difficulty increases to slow down the mining rate and maintain the target block time. This ensures consistent block creation intervals.

### **Question 8: Block Time**
**Answer: B) To maintain consistent block creation rate**

**Explanation**: Setting a target block time helps maintain a consistent rate of block creation, which is important for network stability and transaction processing predictability.

---

## **True/False Questions**

### **Question 9**
**Answer: True**

**Explanation**: Most blockchain implementations require at least one transaction in each block to be valid. This ensures that blocks contain meaningful data and prevents empty blocks from being mined.

### **Question 10**
**Answer: True**

**Explanation**: The genesis block has a special previous hash value, typically empty string or all zeros, since it has no previous block to reference. This distinguishes it from all other blocks.

### **Question 11**
**Answer: False**

**Explanation**: Block mining is not deterministic because it involves finding a nonce that produces a hash meeting difficulty requirements. The process is probabilistic and can take different amounts of time.

### **Question 12**
**Answer: False**

**Explanation**: Wallet balances are typically updated when transactions are included in mined blocks, not when they're added to the queue. This ensures that only confirmed transactions affect balances.

### **Question 13**
**Answer: True**

**Explanation**: Blockchain data should be saved after every block is added to ensure data integrity and prevent loss of blockchain state in case of program termination or crashes.

### **Question 14**
**Answer: True**

**Explanation**: Mining difficulty is determined by the number of leading zeros required in the block hash. More leading zeros mean higher difficulty and more computational work required.

---

## **Practical Questions**

### **Question 15: Genesis Block Creation**

```go
// createGenesisBlock creates the first block in the blockchain
func (bc *Blockchain) createGenesisBlock() {
    // Create initial wallets
    devWallet, _ := NewWallet()
    minerWallet, _ := NewWallet()
    
    bc.Wallets[devWallet.Address] = devWallet
    bc.Wallets[minerWallet.Address] = minerWallet
    
    // Create initial transactions
    coinbaseTx := NewCoinbaseTransaction(minerWallet.Address, 100)
    bankTx := NewBankTransaction(devWallet.Address, minerWallet.Address, 50, 1)
    
    // Create genesis block
    genesisBlock := NewBlock(0, []Transaction{coinbaseTx, bankTx}, "")
    genesisBlock.MineBlock()
    
    // Add to blockchain
    bc.Blocks = append(bc.Blocks, genesisBlock)
    bc.CurrentIndex = 0
    bc.NextIndex = 1
    
    // Update wallets
    minerWallet.UpdateBalance(100)
    minerWallet.UpdateBalance(-50)
    devWallet.UpdateBalance(50)
    
    // Add transactions to lookup
    bc.TXLookup[coinbaseTx.GetID()] = coinbaseTx
    bc.TXLookup[bankTx.GetID()] = bankTx
    
    // Save blockchain
    bc.Save()
    
    fmt.Printf("✅ Genesis block created with hash: %s\n", genesisBlock.Hash)
}

// NewCoinbaseTransaction creates a new coinbase transaction (mining reward)
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

### **Question 16: Block Mining Implementation**

```go
// MineBlock mines the block with proof of work
func (b *Block) MineBlock() {
    target := ""
    for i := 0; i < b.Header.Difficulty; i++ {
        target += "0"
    }
    
    fmt.Printf("⛏️  Mining block %d with difficulty %d...\n", b.Header.Index, b.Header.Difficulty)
    startTime := time.Now()
    
    for {
        b.Hash = b.calculateHash()
        if b.Hash[:b.Header.Difficulty] == target {
            break
        }
        b.Header.Nonce++
    }
    
    miningTime := time.Since(startTime)
    fmt.Printf("✅ Block %d mined in %v with hash: %s\n", b.Header.Index, miningTime, b.Hash)
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

// ValidateProof validates the proof of work
func (b *Block) ValidateProof(difficulty int) bool {
    target := ""
    for i := 0; i < difficulty; i++ {
        target += "0"
    }
    
    return b.Hash[:difficulty] == target
}

// MineNextBlock mines the next block
func (bc *Blockchain) MineNextBlock() (*Block, error) {
    bc.mu.Lock()
    defer bc.mu.Unlock()
    
    if len(bc.TransactionQueue) == 0 {
        return nil, fmt.Errorf("no transactions to mine")
    }
    
    // Get previous block hash
    var previousHash string
    if len(bc.Blocks) > 0 {
        previousHash = bc.Blocks[len(bc.Blocks)-1].Hash
    }
    
    // Create new block
    block := NewBlock(bc.NextIndex, bc.TransactionQueue, previousHash)
    block.Header.Difficulty = bc.Difficulty
    
    // Mine the block
    block.MineBlock()
    
    // Validate block
    if err := block.Validate(); err != nil {
        return nil, fmt.Errorf("invalid block: %w", err)
    }
    
    // Add block to chain
    bc.Blocks = append(bc.Blocks, block)
    bc.CurrentIndex = block.Header.Index
    bc.NextIndex = block.Header.Index + 1
    
    // Update wallets
    bc.updateWalletsFromBlock(block)
    
    // Clear transaction queue
    bc.TransactionQueue = []Transaction{}
    
    // Save blockchain
    bc.Save()
    
    return block, nil
}
```

### **Question 17: Transaction Queue Management**

```go
// AddTransaction adds a transaction to the queue
func (bc *Blockchain) AddTransaction(tx Transaction) error {
    bc.mu.Lock()
    defer bc.mu.Unlock()
    
    // Validate transaction
    if err := tx.Validate(); err != nil {
        return fmt.Errorf("invalid transaction: %w", err)
    }
    
    // Check for double spending
    if bc.TXLookup[tx.GetID()] != nil {
        return fmt.Errorf("transaction already exists")
    }
    
    // Check sender has sufficient balance (simplified)
    if tx.GetSender() != "coinbase" {
        senderBalance := bc.GetWalletBalance(tx.GetSender())
        requiredAmount := tx.GetAmount() + tx.GetFee()
        
        if senderBalance < requiredAmount {
            return fmt.Errorf("insufficient balance: required %.2f, available %.2f", 
                requiredAmount, senderBalance)
        }
    }
    
    // Add to queue
    bc.TransactionQueue = append(bc.TransactionQueue, tx)
    bc.TXLookup[tx.GetID()] = &tx
    
    fmt.Printf("📝 Transaction added to queue: %s\n", tx.GetID())
    return nil
}

// GetTransactionQueue returns the current transaction queue
func (bc *Blockchain) GetTransactionQueue() []Transaction {
    bc.mu.RLock()
    defer bc.mu.RUnlock()
    
    return bc.TransactionQueue
}

// ClearTransactionQueue clears the transaction queue
func (bc *Blockchain) ClearTransactionQueue() {
    bc.mu.Lock()
    defer bc.mu.Unlock()
    
    bc.TransactionQueue = []Transaction{}
}

// ProcessTransactionQueue processes all transactions in the queue
func (bc *Blockchain) ProcessTransactionQueue() error {
    bc.mu.Lock()
    defer bc.mu.Unlock()
    
    for i, tx := range bc.TransactionQueue {
        if err := tx.Validate(); err != nil {
            return fmt.Errorf("invalid transaction %d: %w", i, err)
        }
    }
    
    return nil
}

// updateWalletsFromBlock updates wallet balances from block transactions
func (bc *Blockchain) updateWalletsFromBlock(block *Block) {
    for _, tx := range block.Transactions {
        // Handle coinbase transactions
        if tx.GetType() == "COINBASE" {
            if wallet, exists := bc.Wallets[tx.GetRecipient()]; exists {
                wallet.UpdateBalance(float64(tx.(*CoinbaseTransaction).TokenCount))
                wallet.IncrementTransactionCount()
            }
            continue
        }
        
        // Handle regular transactions
        if wallet, exists := bc.Wallets[tx.GetSender()]; exists {
            wallet.UpdateBalance(-tx.GetAmount() - tx.GetFee())
            wallet.IncrementTransactionCount()
        }
        
        if wallet, exists := bc.Wallets[tx.GetRecipient()]; exists {
            wallet.UpdateBalance(tx.GetAmount())
            wallet.IncrementTransactionCount()
        }
    }
}
```

### **Question 18: Blockchain Persistence**

```go
// Save saves the blockchain to disk
func (bc *Blockchain) Save() error {
    // Save blockchain state
    stateData, err := json.MarshalIndent(bc, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal blockchain state: %w", err)
    }
    
    stateFile := filepath.Join(bc.DataDir, "blockchain.json")
    if err := os.WriteFile(stateFile, stateData, 0644); err != nil {
        return fmt.Errorf("failed to write blockchain state: %w", err)
    }
    
    // Save individual blocks
    blocksDir := filepath.Join(bc.DataDir, "blocks")
    if err := os.MkdirAll(blocksDir, 0755); err != nil {
        return fmt.Errorf("failed to create blocks directory: %w", err)
    }
    
    for _, block := range bc.Blocks {
        blockData, err := block.ToJSON()
        if err != nil {
            return fmt.Errorf("failed to marshal block %d: %w", block.Header.Index, err)
        }
        
        blockFile := filepath.Join(blocksDir, fmt.Sprintf("%d.json", block.Header.Index))
        if err := os.WriteFile(blockFile, blockData, 0644); err != nil {
            return fmt.Errorf("failed to write block %d: %w", block.Header.Index, err)
        }
    }
    
    return nil
}

// Load loads the blockchain from disk
func (bc *Blockchain) Load() error {
    // Load blockchain state
    stateFile := filepath.Join(bc.DataDir, "blockchain.json")
    stateData, err := os.ReadFile(stateFile)
    if err != nil {
        return fmt.Errorf("failed to read blockchain state: %w", err)
    }
    
    if err := json.Unmarshal(stateData, bc); err != nil {
        return fmt.Errorf("failed to unmarshal blockchain state: %w", err)
    }
    
    // Load individual blocks
    blocksDir := filepath.Join(bc.DataDir, "blocks")
    files, err := os.ReadDir(blocksDir)
    if err != nil {
        return fmt.Errorf("failed to read blocks directory: %w", err)
    }
    
    // Sort files by block index
    var blockFiles []string
    for _, file := range files {
        if filepath.Ext(file.Name()) == ".json" {
            blockFiles = append(blockFiles, file.Name())
        }
    }
    sort.Strings(blockFiles)
    
    // Load blocks in order
    for _, fileName := range blockFiles {
        blockFile := filepath.Join(blocksDir, fileName)
        blockData, err := os.ReadFile(blockFile)
        if err != nil {
            return fmt.Errorf("failed to read block file %s: %w", fileName, err)
        }
        
        var block Block
        if err := json.Unmarshal(blockData, &block); err != nil {
            return fmt.Errorf("failed to unmarshal block %s: %w", fileName, err)
        }
        
        bc.Blocks = append(bc.Blocks, &block)
    }
    
    return nil
}

// Backup creates a backup of the blockchain
func (bc *Blockchain) Backup(backupDir string) error {
    if err := os.MkdirAll(backupDir, 0755); err != nil {
        return fmt.Errorf("failed to create backup directory: %w", err)
    }
    
    // Save to backup location
    originalDataDir := bc.DataDir
    bc.DataDir = backupDir
    defer func() { bc.DataDir = originalDataDir }()
    
    return bc.Save()
}

// Restore restores the blockchain from a backup
func (bc *Blockchain) Restore(backupDir string) error {
    // Load from backup location
    originalDataDir := bc.DataDir
    bc.DataDir = backupDir
    defer func() { bc.DataDir = originalDataDir }()
    
    if err := bc.Load(); err != nil {
        return fmt.Errorf("failed to restore from backup: %w", err)
    }
    
    // Save to original location
    bc.DataDir = originalDataDir
    return bc.Save()
}
```

---

## **Bonus Challenge**

### **Question 19: Complete Working Blockchain**

```go
package main

import (
    "bufio"
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strconv"
    "strings"
    "sync"
    "time"
)

// Block represents a blockchain block
type Block struct {
    Index        int           `json:"index"`
    Timestamp    time.Time     `json:"timestamp"`
    Data         string        `json:"data"`
    PreviousHash string        `json:"previous_hash"`
    Hash         string        `json:"hash"`
    Nonce        int           `json:"nonce"`
    Transactions []Transaction `json:"transactions"`
}

// Transaction represents a blockchain transaction
type Transaction struct {
    ID        string    `json:"id"`
    Sender    string    `json:"sender"`
    Recipient string    `json:"recipient"`
    Amount    float64   `json:"amount"`
    Fee       float64   `json:"fee"`
    Timestamp time.Time `json:"timestamp"`
    Type      string    `json:"type"`
}

// Wallet represents a blockchain wallet
type Wallet struct {
    ID              string            `json:"id"`
    Address         string            `json:"address"`
    Balance         float64           `json:"balance"`
    CreatedAt       time.Time         `json:"created_at"`
    LastUpdated     time.Time         `json:"last_updated"`
    TransactionCount int              `json:"transaction_count"`
    Metadata        map[string]string `json:"metadata"`
}

// Blockchain represents the main blockchain structure
type Blockchain struct {
    Blocks           []*Block                    `json:"blocks"`
    TransactionQueue []Transaction               `json:"transaction_queue"`
    Wallets          map[string]*Wallet          `json:"wallets"`
    TXLookup         map[string]*Transaction     `json:"tx_lookup"`
    CurrentIndex     int                         `json:"current_index"`
    NextIndex        int                         `json:"next_index"`
    Difficulty       int                         `json:"difficulty"`
    BlockTime        time.Duration               `json:"block_time"`
    DataDir          string                      `json:"data_dir"`
    mu               sync.RWMutex                `json:"-"`
    stopMining       chan bool                   `json:"-"`
    isMining         bool                        `json:"-"`
}

// NewBlockchain creates a new blockchain
func NewBlockchain(dataDir string) *Blockchain {
    bc := &Blockchain{
        Blocks:           []*Block{},
        TransactionQueue: []Transaction{},
        Wallets:          make(map[string]*Wallet),
        TXLookup:         make(map[string]*Transaction),
        CurrentIndex:     0,
        NextIndex:        0,
        Difficulty:       4,
        BlockTime:        10 * time.Second,
        DataDir:          dataDir,
        stopMining:       make(chan bool),
        isMining:         false,
    }
    
    // Create data directory
    if err := os.MkdirAll(dataDir, 0755); err != nil {
        panic(fmt.Sprintf("failed to create data directory: %v", err))
    }
    
    // Load existing blockchain or create new one
    if err := bc.Load(); err != nil {
        bc.createGenesisBlock()
    }
    
    return bc
}

// createGenesisBlock creates the first block in the blockchain
func (bc *Blockchain) createGenesisBlock() {
    // Create initial wallets
    devWallet := NewWallet("dev_wallet")
    minerWallet := NewWallet("miner_wallet")
    
    bc.Wallets[devWallet.Address] = devWallet
    bc.Wallets[minerWallet.Address] = minerWallet
    
    // Create initial transactions
    coinbaseTx := NewCoinbaseTransaction(minerWallet.Address, 100)
    bankTx := NewBankTransaction(devWallet.Address, minerWallet.Address, 50, 1)
    
    // Create genesis block
    genesisBlock := NewBlock(0, "Genesis Block", "", []Transaction{coinbaseTx, bankTx})
    genesisBlock.MineBlock(bc.Difficulty)
    
    // Add to blockchain
    bc.Blocks = append(bc.Blocks, genesisBlock)
    bc.CurrentIndex = 0
    bc.NextIndex = 1
    
    // Update wallets
    minerWallet.UpdateBalance(100)
    minerWallet.UpdateBalance(-50)
    devWallet.UpdateBalance(50)
    
    // Add transactions to lookup
    bc.TXLookup[coinbaseTx.ID] = &coinbaseTx
    bc.TXLookup[bankTx.ID] = &bankTx
    
    // Save blockchain
    bc.Save()
    
    fmt.Printf("✅ Genesis block created with hash: %s\n", genesisBlock.Hash)
}

// NewBlock creates a new block
func NewBlock(index int, data string, previousHash string, transactions []Transaction) *Block {
    block := &Block{
        Index:        index,
        Timestamp:    time.Now(),
        Data:         data,
        PreviousHash: previousHash,
        Transactions: transactions,
        Nonce:        0,
    }
    
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
        return fmt.Errorf("block index cannot be negative")
    }
    
    if b.Data == "" {
        return fmt.Errorf("block data cannot be empty")
    }
    
    // Verify hash
    expectedHash := b.calculateHash()
    if b.Hash != expectedHash {
        return fmt.Errorf("invalid block hash")
    }
    
    return nil
}

// NewWallet creates a new wallet
func NewWallet(name string) *Wallet {
    return &Wallet{
        ID:              generateID(),
        Address:         generateAddress(),
        Balance:         0.0,
        CreatedAt:       time.Now(),
        LastUpdated:     time.Now(),
        TransactionCount: 0,
        Metadata:        map[string]string{"name": name},
    }
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

// NewTransaction creates a new transaction
func NewTransaction(sender, recipient string, amount, fee float64, txType string) Transaction {
    return Transaction{
        ID:        generateID(),
        Sender:    sender,
        Recipient: recipient,
        Amount:    amount,
        Fee:       fee,
        Timestamp: time.Now(),
        Type:      txType,
    }
}

// NewCoinbaseTransaction creates a new coinbase transaction
func NewCoinbaseTransaction(recipient string, amount float64) Transaction {
    return NewTransaction("coinbase", recipient, amount, 0, "COINBASE")
}

// NewBankTransaction creates a new bank transaction
func NewBankTransaction(sender, recipient string, amount, fee float64) Transaction {
    return NewTransaction(sender, recipient, amount, fee, "BANK")
}

// generateID generates a unique ID
func generateID() string {
    data := fmt.Sprintf("%d", time.Now().UnixNano())
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:16])
}

// generateAddress generates a wallet address
func generateAddress() string {
    data := fmt.Sprintf("%d", time.Now().UnixNano())
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

// AddTransaction adds a transaction to the queue
func (bc *Blockchain) AddTransaction(tx Transaction) error {
    bc.mu.Lock()
    defer bc.mu.Unlock()
    
    // Check for duplicate
    if bc.TXLookup[tx.ID] != nil {
        return fmt.Errorf("transaction already exists")
    }
    
    // Add to queue
    bc.TransactionQueue = append(bc.TransactionQueue, tx)
    bc.TXLookup[tx.ID] = &tx
    
    fmt.Printf("📝 Transaction added to queue: %s\n", tx.ID)
    return nil
}

// MineNextBlock mines the next block
func (bc *Blockchain) MineNextBlock() (*Block, error) {
    bc.mu.Lock()
    defer bc.mu.Unlock()
    
    if len(bc.TransactionQueue) == 0 {
        return nil, fmt.Errorf("no transactions to mine")
    }
    
    // Get previous block hash
    var previousHash string
    if len(bc.Blocks) > 0 {
        previousHash = bc.Blocks[len(bc.Blocks)-1].Hash
    }
    
    // Create new block
    block := NewBlock(bc.NextIndex, fmt.Sprintf("Block %d", bc.NextIndex), previousHash, bc.TransactionQueue)
    
    // Mine the block
    fmt.Printf("⛏️  Mining block %d with difficulty %d...\n", block.Index, bc.Difficulty)
    startTime := time.Now()
    
    block.MineBlock(bc.Difficulty)
    
    miningTime := time.Since(startTime)
    fmt.Printf("✅ Block %d mined in %v with hash: %s\n", block.Index, miningTime, block.Hash)
    
    // Validate block
    if err := block.Validate(); err != nil {
        return nil, fmt.Errorf("invalid block: %w", err)
    }
    
    // Add block to chain
    bc.Blocks = append(bc.Blocks, block)
    bc.CurrentIndex = block.Index
    bc.NextIndex = block.Index + 1
    
    // Update wallets
    bc.updateWalletsFromBlock(block)
    
    // Clear transaction queue
    bc.TransactionQueue = []Transaction{}
    
    // Save blockchain
    bc.Save()
    
    return block, nil
}

// updateWalletsFromBlock updates wallet balances from block transactions
func (bc *Blockchain) updateWalletsFromBlock(block *Block) {
    for _, tx := range block.Transactions {
        // Handle coinbase transactions
        if tx.Type == "COINBASE" {
            if wallet, exists := bc.Wallets[tx.Recipient]; exists {
                wallet.UpdateBalance(tx.Amount)
                wallet.IncrementTransactionCount()
            }
            continue
        }
        
        // Handle regular transactions
        if wallet, exists := bc.Wallets[tx.Sender]; exists {
            wallet.UpdateBalance(-tx.Amount - tx.Fee)
            wallet.IncrementTransactionCount()
        }
        
        if wallet, exists := bc.Wallets[tx.Recipient]; exists {
            wallet.UpdateBalance(tx.Amount)
            wallet.IncrementTransactionCount()
        }
    }
}

// ValidateChain validates the entire blockchain
func (bc *Blockchain) ValidateChain() error {
    bc.mu.RLock()
    defer bc.mu.RUnlock()
    
    for i := 1; i < len(bc.Blocks); i++ {
        currentBlock := bc.Blocks[i]
        previousBlock := bc.Blocks[i-1]
        
        // Check previous hash
        if currentBlock.PreviousHash != previousBlock.Hash {
            return fmt.Errorf("chain broken at block %d", i)
        }
        
        // Validate current block
        if err := currentBlock.Validate(); err != nil {
            return fmt.Errorf("block %d validation failed: %w", i, err)
        }
    }
    
    return nil
}

// GetWalletBalance returns the balance of a wallet
func (bc *Blockchain) GetWalletBalance(address string) float64 {
    bc.mu.RLock()
    defer bc.mu.RUnlock()
    
    if wallet, exists := bc.Wallets[address]; exists {
        return wallet.Balance
    }
    
    return 0.0
}

// Save saves the blockchain to disk
func (bc *Blockchain) Save() error {
    stateData, err := json.MarshalIndent(bc, "", "  ")
    if err != nil {
        return fmt.Errorf("failed to marshal blockchain state: %w", err)
    }
    
    stateFile := filepath.Join(bc.DataDir, "blockchain.json")
    if err := os.WriteFile(stateFile, stateData, 0644); err != nil {
        return fmt.Errorf("failed to write blockchain state: %w", err)
    }
    
    return nil
}

// Load loads the blockchain from disk
func (bc *Blockchain) Load() error {
    stateFile := filepath.Join(bc.DataDir, "blockchain.json")
    stateData, err := os.ReadFile(stateFile)
    if err != nil {
        return fmt.Errorf("failed to read blockchain state: %w", err)
    }
    
    if err := json.Unmarshal(stateData, bc); err != nil {
        return fmt.Errorf("failed to unmarshal blockchain state: %w", err)
    }
    
    return nil
}

// DisplayBlockchain displays the blockchain
func (bc *Blockchain) DisplayBlockchain() {
    bc.mu.RLock()
    defer bc.mu.RUnlock()
    
    fmt.Println("\n=== BLOCKCHAIN ===")
    fmt.Printf("Total Blocks: %d\n", len(bc.Blocks))
    fmt.Printf("Current Index: %d\n", bc.CurrentIndex)
    fmt.Printf("Difficulty: %d\n", bc.Difficulty)
    fmt.Printf("Pending Transactions: %d\n", len(bc.TransactionQueue))
    fmt.Printf("Total Wallets: %d\n", len(bc.Wallets))
    
    if len(bc.Blocks) > 0 {
        fmt.Println("\n=== BLOCKS ===")
        for i, block := range bc.Blocks {
            fmt.Printf("Block #%d:\n", i)
            fmt.Printf("  Hash: %s\n", block.Hash)
            fmt.Printf("  Previous: %s\n", block.PreviousHash)
            fmt.Printf("  Timestamp: %s\n", block.Timestamp.Format(time.RFC3339))
            fmt.Printf("  Transactions: %d\n", len(block.Transactions))
            fmt.Printf("  Nonce: %d\n", block.Nonce)
            fmt.Println()
        }
    }
    
    if len(bc.Wallets) > 0 {
        fmt.Println("=== WALLETS ===")
        for address, wallet := range bc.Wallets {
            fmt.Printf("Address: %s\n", address)
            fmt.Printf("  Balance: %.2f\n", wallet.Balance)
            fmt.Printf("  Transactions: %d\n", wallet.TransactionCount)
            fmt.Printf("  Created: %s\n", wallet.CreatedAt.Format(time.RFC3339))
            fmt.Println()
        }
    }
}

// Interactive menu functions
func runInteractiveMenu(bc *Blockchain) {
    scanner := bufio.NewScanner(os.Stdin)
    
    for {
        displayMenu()
        
        if !scanner.Scan() {
            break
        }
        
        choice := strings.TrimSpace(scanner.Text())
        
        switch choice {
        case "1":
            mineBlock(bc)
        case "2":
            addTransaction(bc, scanner)
        case "3":
            createWallet(bc)
        case "4":
            displayBlockchain(bc)
        case "5":
            startMining(bc)
        case "6":
            stopMining(bc)
        case "7":
            validateChain(bc)
        case "8":
            displayInfo(bc)
        case "9":
            fmt.Println("👋 Goodbye!")
            return
        default:
            fmt.Println("❌ Invalid choice. Please try again.")
        }
        
        fmt.Println()
    }
}

func displayMenu() {
    fmt.Println("=== BLOCKCHAIN MENU ===")
    fmt.Println("1. Mine next block")
    fmt.Println("2. Add transaction")
    fmt.Println("3. Create wallet")
    fmt.Println("4. Display blockchain")
    fmt.Println("5. Start mining")
    fmt.Println("6. Stop mining")
    fmt.Println("7. Validate chain")
    fmt.Println("8. Display info")
    fmt.Println("9. Exit")
    fmt.Print("Enter your choice: ")
}

func mineBlock(bc *Blockchain) {
    fmt.Println("⛏️  Mining next block...")
    
    if len(bc.TransactionQueue) == 0 {
        fmt.Println("❌ No transactions to mine. Add some transactions first.")
        return
    }
    
    startTime := time.Now()
    block, err := bc.MineNextBlock()
    if err != nil {
        fmt.Printf("❌ Mining failed: %v\n", err)
        return
    }
    
    miningTime := time.Since(startTime)
    fmt.Printf("✅ Block %d mined successfully in %v!\n", block.Index, miningTime)
    fmt.Printf("   Hash: %s\n", block.Hash)
    fmt.Printf("   Nonce: %d\n", block.Nonce)
}

func addTransaction(bc *Blockchain, scanner *bufio.Scanner) {
    if len(bc.Wallets) < 2 {
        fmt.Println("❌ Need at least 2 wallets. Create wallets first.")
        return
    }
    
    // Get sender
    fmt.Print("Enter sender address: ")
    if !scanner.Scan() {
        return
    }
    sender := strings.TrimSpace(scanner.Text())
    
    // Get recipient
    fmt.Print("Enter recipient address: ")
    if !scanner.Scan() {
        return
    }
    recipient := strings.TrimSpace(scanner.Text())
    
    // Get amount
    fmt.Print("Enter amount: ")
    if !scanner.Scan() {
        return
    }
    amountStr := strings.TrimSpace(scanner.Text())
    amount, err := strconv.ParseFloat(amountStr, 64)
    if err != nil {
        fmt.Println("❌ Invalid amount")
        return
    }
    
    // Get fee
    fmt.Print("Enter fee: ")
    if !scanner.Scan() {
        return
    }
    feeStr := strings.TrimSpace(scanner.Text())
    fee, err := strconv.ParseFloat(feeStr, 64)
    if err != nil {
        fmt.Println("❌ Invalid fee")
        return
    }
    
    // Create transaction
    tx := NewBankTransaction(sender, recipient, amount, fee)
    
    // Add to blockchain
    if err := bc.AddTransaction(tx); err != nil {
        fmt.Printf("❌ Failed to add transaction: %v\n", err)
        return
    }
    
    fmt.Printf("✅ Transaction added: %s\n", tx.ID)
}

func createWallet(bc *Blockchain) {
    fmt.Print("Enter wallet name: ")
    scanner := bufio.NewScanner(os.Stdin)
    if !scanner.Scan() {
        return
    }
    name := strings.TrimSpace(scanner.Text())
    
    wallet := NewWallet(name)
    bc.Wallets[wallet.Address] = wallet
    bc.Save()
    
    fmt.Printf("✅ Wallet created successfully!\n")
    fmt.Printf("   Address: %s\n", wallet.Address)
    fmt.Printf("   ID: %s\n", wallet.ID)
}

func displayBlockchain(bc *Blockchain) {
    bc.DisplayBlockchain()
}

func startMining(bc *Blockchain) {
    bc.StartMining()
}

func stopMining(bc *Blockchain) {
    bc.StopMining()
}

func validateChain(bc *Blockchain) {
    fmt.Println("🔍 Validating blockchain...")
    
    startTime := time.Now()
    err := bc.ValidateChain()
    validationTime := time.Since(startTime)
    
    if err != nil {
        fmt.Printf("❌ Blockchain validation failed: %v\n", err)
        return
    }
    
    fmt.Printf("✅ Blockchain is valid! (took %v)\n", validationTime)
}

func displayInfo(bc *Blockchain) {
    bc.mu.RLock()
    defer bc.mu.RUnlock()
    
    fmt.Println("=== BLOCKCHAIN INFO ===")
    fmt.Printf("Total Blocks: %d\n", len(bc.Blocks))
    fmt.Printf("Current Index: %d\n", bc.CurrentIndex)
    fmt.Printf("Next Index: %d\n", bc.NextIndex)
    fmt.Printf("Difficulty: %d\n", bc.Difficulty)
    fmt.Printf("Pending Transactions: %d\n", len(bc.TransactionQueue))
    fmt.Printf("Total Wallets: %d\n", len(bc.Wallets))
    fmt.Printf("Is Mining: %t\n", bc.isMining)
}

// StartMining starts the mining process
func (bc *Blockchain) StartMining() {
    if bc.isMining {
        return
    }
    
    bc.isMining = true
    go func() {
        for {
            select {
            case <-bc.stopMining:
                bc.isMining = false
                return
            default:
                if len(bc.TransactionQueue) > 0 {
                    if _, err := bc.MineNextBlock(); err != nil {
                        fmt.Printf("❌ Mining failed: %v\n", err)
                    }
                }
                time.Sleep(bc.BlockTime)
            }
        }
    }()
    
    fmt.Println("🚀 Mining started!")
}

// StopMining stops the mining process
func (bc *Blockchain) StopMining() {
    if !bc.isMining {
        return
    }
    
    close(bc.stopMining)
    fmt.Println("⏹️  Mining stopped!")
}

func main() {
    fmt.Println("🚀 Welcome to Go Basic Blockchain!")
    fmt.Println("Building your first working blockchain...\n")
    
    // Create blockchain
    blockchain := NewBlockchain("data/blockchain")
    
    // Display initial state
    blockchain.DisplayBlockchain()
    
    // Start interactive menu
    runInteractiveMenu(blockchain)
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

- **Excellent (90%+)**: 47+ points - You have mastered basic blockchain implementation
- **Good (80-89%)**: 42-46 points - You understand the concepts well with minor gaps
- **Satisfactory (70-79%)**: 36-41 points - You have a basic understanding but need more practice
- **Needs Improvement (<70%)**: <36 points - Review the section material and retake the quiz

### **Next Steps Based on Your Score:**

- **90%+**: Excellent! You're ready for Phase 2
- **80-89%**: Good work! Review any missed concepts before moving on
- **70-79%**: Spend more time on the hands-on exercises before proceeding
- **<70%**: Review the section material thoroughly and retake the quiz

---

**🎉 Congratulations! You've successfully completed Phase 1 and built your first working blockchain! You now have a complete, functional blockchain system that you can run, test, and extend. You're ready to move on to advanced features in [Phase 2](../../phase2/README.md)! 🚀**
