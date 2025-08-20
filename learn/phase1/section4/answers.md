# Section 4 Quiz Answers

## 📋 Answer Key

Here are the correct answers and explanations for the Section 4 quiz.

---

## **Multiple Choice Questions**

### **Question 1: Block Structure**
**Answer: B) To efficiently verify transaction inclusion**

**Explanation**: The Merkle root allows efficient verification that a transaction is included in a block without downloading all transactions. This is crucial for lightweight clients and SPV (Simplified Payment Verification).

### **Question 2: Transaction Interface**
**Answer: B) They allow for polymorphism and different transaction types**

**Explanation**: Interfaces in Go provide polymorphism, allowing different transaction types (Bank, Message, Coinbase) to be used interchangeably as long as they implement the required methods.

### **Question 3: Wallet System**
**Answer: B) To manage cryptographic keys and addresses**

**Explanation**: Wallets primarily manage cryptographic key pairs and derive addresses from public keys. They don't actually store cryptocurrency - that's recorded on the blockchain.

### **Question 4: PUID System**
**Answer: A) Persistent Unique Identifier**

**Explanation**: PUID stands for Persistent Unique Identifier, which provides unique identifiers that persist across blockchain operations and can be used to track entities.

### **Question 5: Merkle Trees**
**Answer: B) They allow efficient verification of transaction inclusion**

**Explanation**: Merkle trees enable efficient verification that a transaction is included in a block by providing a proof path, without needing to download all transactions.

### **Question 6: JSON Tags**
**Answer: B) It omits the field from JSON output if it's empty**

**Explanation**: The `omitempty` tag tells the JSON encoder to skip the field if it has a zero value (empty string, 0, nil, etc.).

### **Question 7: Block Validation**
**Answer: D) Check the block index**

**Explanation**: The first step in block validation is checking the block index to ensure it's in the correct sequence and not negative.

### **Question 8: Address Generation**
**Answer: C) Both SHA-256 and RIPEMD-160**

**Explanation**: Bitcoin-style addresses typically use both SHA-256 and RIPEMD-160. SHA-256 is used first, then RIPEMD-160 to create a shorter, more manageable address.

---

## **True/False Questions**

### **Question 9**
**Answer: True**

**Explanation**: A block header contains all the metadata needed to identify and validate a block, including index, timestamp, previous hash, Merkle root, nonce, difficulty, and version.

### **Question 10**
**Answer: False**

**Explanation**: Go uses implicit interface implementation. A type automatically implements an interface if it has all the required methods, without any explicit declaration.

### **Question 11**
**Answer: True**

**Explanation**: Merkle trees allow efficient verification of transaction inclusion by providing a proof path from the transaction to the root, without downloading all transactions.

### **Question 12**
**Answer: False**

**Explanation**: Wallet private keys should never be stored in plain text. They should be encrypted and stored securely to prevent theft and unauthorized access.

### **Question 13**
**Answer: True**

**Explanation**: PUIDs (Persistent Unique Identifiers) are designed to provide unique identifiers that persist across blockchain operations and can be used to track entities.

### **Question 14**
**Answer: True**

**Explanation**: Block size calculation is important for network transmission efficiency, storage optimization, and ensuring blocks don't exceed network limits.

---

## **Practical Questions**

### **Question 15: Block Structure Implementation**

```go
package blockchain

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
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
            Difficulty:   4,
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
```

### **Question 16: Transaction Interface Design**

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

// Implement interface methods for BaseTransaction
func (bt *BaseTransaction) GetID() string { return bt.ID }
func (bt *BaseTransaction) GetSender() string { return bt.Sender }
func (bt *BaseTransaction) GetRecipient() string { return bt.Recipient }
func (bt *BaseTransaction) GetAmount() float64 { return bt.Amount }
func (bt *BaseTransaction) GetFee() float64 { return bt.Fee }
func (bt *BaseTransaction) GetTimestamp() time.Time { return bt.Timestamp }
func (bt *BaseTransaction) GetType() string { return bt.Type }

func (bt *BaseTransaction) GetHash() string {
    data := fmt.Sprintf("%s%s%s%f%f%s", 
        bt.ID, bt.Sender, bt.Recipient, bt.Amount, bt.Fee, bt.Type)
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

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

func (bt *BaseTransaction) Sign(privateKey []byte) error {
    data := bt.GetHash()
    bt.Signature = []byte(data) // Simplified signature
    return nil
}

func (bt *BaseTransaction) VerifySignature() bool {
    return len(bt.Signature) > 0
}

func (bt *BaseTransaction) ToJSON() ([]byte, error) {
    return json.MarshalIndent(bt, "", "  ")
}

func (bt *BaseTransaction) FromJSON(data []byte) error {
    return json.Unmarshal(data, bt)
}
```

### **Question 17: Wallet System Implementation**

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
    return hex.EncodeToString(hash[:16])
}

// generateAddress generates an address from a public key
func generateAddress(publicKey ecdsa.PublicKey) string {
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

### **Question 18: Merkle Tree Implementation**

```go
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

// verifyMerkleProof verifies that a transaction is included in a block
func verifyMerkleProof(transactionHash string, merkleRoot string, proof []string, proofIndex int) bool {
    currentHash := transactionHash
    
    for i, siblingHash := range proof {
        if proofIndex%2 == 0 {
            // Current hash is left child
            combined := currentHash + siblingHash
            hash := sha256.Sum256([]byte(combined))
            currentHash = hex.EncodeToString(hash[:])
        } else {
            // Current hash is right child
            combined := siblingHash + currentHash
            hash := sha256.Sum256([]byte(combined))
            currentHash = hex.EncodeToString(hash[:])
        }
        proofIndex = proofIndex / 2
    }
    
    return currentHash == merkleRoot
}

// generateMerkleProof generates a proof for a transaction
func generateMerkleProof(leaves []string, targetIndex int) ([]string, error) {
    if targetIndex >= len(leaves) {
        return nil, fmt.Errorf("target index out of range")
    }
    
    var proof []string
    currentIndex := targetIndex
    
    // Build the tree and collect proof
    currentLevel := leaves
    for len(currentLevel) > 1 {
        if len(currentLevel)%2 != 0 {
            currentLevel = append(currentLevel, currentLevel[len(currentLevel)-1])
        }
        
        // Find sibling
        if currentIndex%2 == 0 {
            // Even index, sibling is next
            if currentIndex+1 < len(currentLevel) {
                proof = append(proof, currentLevel[currentIndex+1])
            }
        } else {
            // Odd index, sibling is previous
            proof = append(proof, currentLevel[currentIndex-1])
        }
        
        // Move to parent level
        currentIndex = currentIndex / 2
        
        // Create parent level
        parentLevel := make([]string, len(currentLevel)/2)
        for i := 0; i < len(currentLevel); i += 2 {
            combined := currentLevel[i] + currentLevel[i+1]
            hash := sha256.Sum256([]byte(combined))
            parentLevel[i/2] = hex.EncodeToString(hash[:])
        }
        currentLevel = parentLevel
    }
    
    return proof, nil
}
```

---

## **Bonus Challenge**

### **Question 19: Complete Data Structure System**

```go
package main

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

// PUID represents a persistent unique identifier
type PUID struct {
    ID        string                 `json:"id"`
    Type      string                 `json:"type"`
    CreatedAt time.Time              `json:"created_at"`
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
    bytes := make([]byte, 16)
    rand.Read(bytes)
    timestamp := time.Now().UnixNano()
    timestampBytes := []byte(fmt.Sprintf("%d", timestamp))
    combined := append(bytes, timestampBytes...)
    hash := sha256.Sum256(combined)
    return hex.EncodeToString(hash[:])
}

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
    PUID         *PUID        `json:"puid"`
    Header       BlockHeader  `json:"header"`
    Transactions []Transaction `json:"transactions"`
    Hash         string       `json:"hash"`
    Size         int          `json:"size"`
}

// Transaction interface
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
    PUID       *PUID    `json:"puid"`
    ID         string   `json:"id"`
    Timestamp  time.Time `json:"timestamp"`
    Sender     string   `json:"sender"`
    Recipient  string   `json:"recipient"`
    Amount     float64  `json:"amount"`
    Fee        float64  `json:"fee"`
    Signature  []byte   `json:"signature"`
    Type       string   `json:"type"`
}

// BankTransaction represents a bank transfer
type BankTransaction struct {
    BaseTransaction
    Currency string `json:"currency"`
}

// MessageTransaction represents a message transaction
type MessageTransaction struct {
    BaseTransaction
    Message string `json:"message"`
}

// Wallet represents a blockchain wallet
type Wallet struct {
    PUID             *PUID             `json:"puid"`
    ID               string            `json:"id"`
    Address          string            `json:"address"`
    PublicKey        []byte            `json:"public_key"`
    PrivateKey       []byte            `json:"private_key,omitempty"`
    Balance          float64           `json:"balance"`
    CreatedAt        time.Time         `json:"created_at"`
    LastUpdated      time.Time         `json:"last_updated"`
    TransactionCount int               `json:"transaction_count"`
    Metadata         map[string]string `json:"metadata"`
}

// NewBlock creates a new block
func NewBlock(index int, transactions []Transaction, previousHash string) *Block {
    block := &Block{
        PUID: NewPUID("block"),
        Header: BlockHeader{
            Index:        index,
            Timestamp:    time.Now(),
            PreviousHash: previousHash,
            Nonce:        0,
            Difficulty:   4,
            Version:      1,
        },
        Transactions: transactions,
    }
    
    block.Header.MerkleRoot = block.calculateMerkleRoot()
    block.Hash = block.calculateHash()
    block.Size = block.calculateSize()
    
    return block
}

// calculateHash calculates the hash of the block
func (b *Block) calculateHash() string {
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
    
    leaves := make([]string, len(b.Transactions))
    for i, tx := range b.Transactions {
        leaves[i] = tx.GetHash()
    }
    
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
    
    if len(leaves)%2 != 0 {
        leaves = append(leaves, leaves[len(leaves)-1])
    }
    
    parents := make([]string, len(leaves)/2)
    for i := 0; i < len(leaves); i += 2 {
        combined := leaves[i] + leaves[i+1]
        hash := sha256.Sum256([]byte(combined))
        parents[i/2] = hex.EncodeToString(hash[:])
    }
    
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

// Validate validates the block
func (b *Block) Validate() error {
    if b.Header.Index < 0 {
        return fmt.Errorf("block index cannot be negative")
    }
    
    if len(b.Transactions) == 0 {
        return fmt.Errorf("block must contain at least one transaction")
    }
    
    expectedHash := b.calculateHash()
    if b.Hash != expectedHash {
        return fmt.Errorf("invalid block hash")
    }
    
    expectedMerkleRoot := b.calculateMerkleRoot()
    if b.Header.MerkleRoot != expectedMerkleRoot {
        return fmt.Errorf("invalid Merkle root")
    }
    
    for i, tx := range b.Transactions {
        if err := tx.Validate(); err != nil {
            return fmt.Errorf("invalid transaction %d: %w", i, err)
        }
    }
    
    return nil
}

// NewBankTransaction creates a new bank transaction
func NewBankTransaction(sender, recipient string, amount, fee float64) *BankTransaction {
    return &BankTransaction{
        BaseTransaction: BaseTransaction{
            PUID:      NewPUID("transaction"),
            ID:        generateTransactionID(),
            Timestamp: time.Now(),
            Sender:    sender,
            Recipient: recipient,
            Amount:    amount,
            Fee:       fee,
            Type:      "BANK",
        },
        Currency: "USD",
    }
}

// NewMessageTransaction creates a new message transaction
func NewMessageTransaction(sender, recipient, message string, fee float64) *MessageTransaction {
    return &MessageTransaction{
        BaseTransaction: BaseTransaction{
            PUID:      NewPUID("transaction"),
            ID:        generateTransactionID(),
            Timestamp: time.Now(),
            Sender:    sender,
            Recipient: recipient,
            Amount:    0,
            Fee:       fee,
            Type:      "MESSAGE",
        },
        Message: message,
    }
}

// generateTransactionID generates a unique transaction ID
func generateTransactionID() string {
    data := fmt.Sprintf("%d", time.Now().UnixNano())
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

// Implement Transaction interface for BaseTransaction
func (bt *BaseTransaction) GetID() string { return bt.ID }
func (bt *BaseTransaction) GetSender() string { return bt.Sender }
func (bt *BaseTransaction) GetRecipient() string { return bt.Recipient }
func (bt *BaseTransaction) GetAmount() float64 { return bt.Amount }
func (bt *BaseTransaction) GetFee() float64 { return bt.Fee }
func (bt *BaseTransaction) GetTimestamp() time.Time { return bt.Timestamp }
func (bt *BaseTransaction) GetType() string { return bt.Type }

func (bt *BaseTransaction) GetHash() string {
    data := fmt.Sprintf("%s%s%s%f%f%s", 
        bt.ID, bt.Sender, bt.Recipient, bt.Amount, bt.Fee, bt.Type)
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

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

func (bt *BaseTransaction) Sign(privateKey []byte) error {
    data := bt.GetHash()
    bt.Signature = []byte(data)
    return nil
}

func (bt *BaseTransaction) VerifySignature() bool {
    return len(bt.Signature) > 0
}

func (bt *BaseTransaction) ToJSON() ([]byte, error) {
    return json.MarshalIndent(bt, "", "  ")
}

func (bt *BaseTransaction) FromJSON(data []byte) error {
    return json.Unmarshal(data, bt)
}

// NewWallet creates a new wallet
func NewWallet() (*Wallet, error) {
    privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
    if err != nil {
        return nil, fmt.Errorf("failed to generate key pair: %w", err)
    }
    
    address := generateAddress(privateKey.PublicKey)
    
    wallet := &Wallet{
        PUID:             NewPUID("wallet"),
        ID:               generateWalletID(),
        Address:          address,
        PublicKey:        publicKeyToBytes(privateKey.PublicKey),
        PrivateKey:       privateKeyToBytes(privateKey),
        Balance:          0.0,
        CreatedAt:        time.Now(),
        LastUpdated:      time.Now(),
        TransactionCount: 0,
        Metadata:         make(map[string]string),
    }
    
    return wallet, nil
}

// generateWalletID generates a unique wallet ID
func generateWalletID() string {
    data := fmt.Sprintf("%d", time.Now().UnixNano())
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:16])
}

// generateAddress generates an address from a public key
func generateAddress(publicKey ecdsa.PublicKey) string {
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

func main() {
    fmt.Println("🧪 Testing Complete Data Structure System...\n")
    
    // Test 1: Create wallet
    fmt.Println("Test 1: Creating wallet...")
    wallet, err := NewWallet()
    if err != nil {
        fmt.Printf("❌ Failed to create wallet: %v\n", err)
        return
    }
    fmt.Printf("✅ Wallet created: %s\n", wallet.Address[:8])
    
    // Test 2: Create transactions
    fmt.Println("\nTest 2: Creating transactions...")
    bankTx := NewBankTransaction("user1", "user2", 100, 1)
    messageTx := NewMessageTransaction("user2", "user3", "Hello Blockchain!", 0.5)
    
    fmt.Printf("✅ Bank transaction created: %s\n", bankTx.ID[:8])
    fmt.Printf("✅ Message transaction created: %s\n", messageTx.ID[:8])
    
    // Test 3: Create block
    fmt.Println("\nTest 3: Creating block...")
    transactions := []Transaction{bankTx, messageTx}
    block := NewBlock(1, transactions, "previous_hash")
    
    fmt.Printf("✅ Block created: %s\n", block.Hash[:8])
    fmt.Printf("   Merkle root: %s\n", block.Header.MerkleRoot[:8])
    fmt.Printf("   Size: %d bytes\n", block.Size)
    
    // Test 4: Validate block
    fmt.Println("\nTest 4: Validating block...")
    if err := block.Validate(); err != nil {
        fmt.Printf("❌ Block validation failed: %v\n", err)
        return
    }
    fmt.Println("✅ Block validation passed!")
    
    // Test 5: JSON serialization
    fmt.Println("\nTest 5: JSON serialization...")
    blockJSON, err := json.MarshalIndent(block, "", "  ")
    if err != nil {
        fmt.Printf("❌ JSON serialization failed: %v\n", err)
        return
    }
    fmt.Printf("✅ Block serialized to JSON (%d bytes)\n", len(blockJSON))
    
    // Test 6: PUID system
    fmt.Println("\nTest 6: PUID system...")
    fmt.Printf("Block PUID: %s\n", block.PUID.ID[:8])
    fmt.Printf("Transaction PUID: %s\n", bankTx.PUID.ID[:8])
    fmt.Printf("Wallet PUID: %s\n", wallet.PUID.ID[:8])
    
    fmt.Println("\n🎉 All tests passed! Data structure system is working!")
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

- **Excellent (90%+)**: 47+ points - You have mastered core data structures
- **Good (80-89%)**: 42-46 points - You understand the concepts well with minor gaps
- **Satisfactory (70-79%)**: 36-41 points - You have a basic understanding but need more practice
- **Needs Improvement (<70%)**: <36 points - Review the section material and retake the quiz

### **Next Steps Based on Your Score:**

- **90%+**: Excellent! You're ready for Section 5
- **80-89%**: Good work! Review any missed concepts before moving on
- **70-79%**: Spend more time on the hands-on exercises before proceeding
- **<70%**: Review the section material thoroughly and retake the quiz

---

**Great job completing Section 4! 🎉**

Ready for the next challenge? Move on to [Section 5: Basic Blockchain Implementation](../section5/README.md)!
