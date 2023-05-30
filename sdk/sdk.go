package sdk

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pborman/uuid"
	"golang.org/x/crypto/pbkdf2"
)

const (
	// proofOfWorkDifficulty is the number of leading zeros that must be found in the hash of a block
	proofOfWorkDifficulty = 4

	// transaction fee is 5 hunderths of a coin (a nickle-ish)
	transactionFee = 0.05

	// miner reward is 50% of the transaction fee
	minerRewardPCT = 50.0

	// minerAddress is the address of the miner (will be supplied by the environment)
	minerAddress = "MINER"

	// devreward is 50% of the transaction fee
	devRewardPCT = 50.0

	// devAddress is the address of the developer
	devAddress = "DEV" // will be supplied by the genesis block

	// salt size is 16 bytes
	saltSize = 16

	// default Amount to fund new wallets is 100 coins
	fundWalletAmount = 100.0

	// block time is 5 seconds
	blockTimeInSec = 5

	// data folder is the folder where the blockchain data is stored
	dataFolder = "../data"

	// Log Date/Time format
	logDateTimeFormat = "2006-01-02 15:04:05"

	// maxNonce is the maximum value for a nonce
	maxNonce = math.MaxInt64
)

// Wallet represents a user's wallet.
type Wallet struct {
	ID         string
	Name       string
	Tags       []string
	PrivateKey []byte
	PublicKey  []byte
	Address    string
	Balance    float64
}

// NewWallet creates a new wallet with a unique ID, name, and set of tags
func NewWallet(name string, tags []string) (*Wallet, error) {
	// Generate a new private key.
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	// Convert the private key to PEM format
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)

	// Generate a new public key.
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	// Create a new wallet with a unique ID, name, and set of tags.
	wallet := &Wallet{
		ID:         uuid.New(),
		Name:       name,
		Tags:       tags,
		PrivateKey: privateKeyBytes,
		PublicKey:  publicKeyBytes,
		Balance:    fundWalletAmount,
	}

	if err != nil {
		return nil, err
	}

	wallet.GetAddress()
	fmt.Printf("[%s] Created new Wallet: %+v\n", time.Now().Format(logDateTimeFormat), wallet)

	return wallet, nil
}

// GetAddress generates and returns the wallet address.
func (w *Wallet) GetAddress() string {
	// If the address is already generated, return it.
	if w.Address != "" {
		return w.Address
	}

	// Generate an address by hashing the public key and encoding it in hexadecimal.
	hash := sha256.Sum256(w.PublicKey)
	w.Address = hex.EncodeToString(hash[:])

	return w.Address
}

// EncryptPrivateKey encrypts the wallet's private key using the passphrase.
func (w *Wallet) EncryptPrivateKey(passphrase string) error {
	// Generate a new salt
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}

	// Derive a new key from the passphrase
	key := pbkdf2.Key([]byte(passphrase), salt, 4096, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	// Encrypt the private key
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	// Generate a new nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	// Encrypt the private key and prepend the salt and nonce to it
	w.PrivateKey = append(salt, append(nonce, gcm.Seal(nil, nonce, w.PrivateKey, nil)...)...)
	return nil
}

// DecryptPrivateKey decrypts the wallet's private key using the passphrase.
func (w *Wallet) DecryptPrivateKey(passphrase string) error {
	// Extract the salt from the encrypted private key
	salt := w.PrivateKey[:saltSize]

	// Derive the key from the passphrase
	key := pbkdf2.Key([]byte(passphrase), salt, 4096, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	// Create a new GCM instance
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	// Extract the nonce from the encrypted private key
	nonceSize := gcm.NonceSize()

	// Extract the nonce and ciphertext from the encrypted private key
	nonce, ciphertext := w.PrivateKey[saltSize:saltSize+nonceSize], w.PrivateKey[saltSize+nonceSize:]

	// Decrypt the private key
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return err
	}

	// Set the plaintext private key
	w.PrivateKey = plaintext
	return nil
}

// Transaction is an interface that defines the Processes for the different types of protocol transactions.
type Transaction interface {
	Process() string
}

// Tx is a transaction that represents a generic transaction.
type Tx struct {
	From *Wallet
	To   *Wallet
	Fee  float64
}

// Bank is a transaction that represents a bank transfer.
type Bank struct {
	Tx
	Amount float64
}

// NewBankTransaction creates a new bank transaction.
func NewBankTransaction(from *Wallet, to *Wallet, amount float64) (*Bank, error) {
	fmt.Printf("[%s] Creating TX (BANK) - FROM: %s, TO: %s, Amount: %f\n", time.Now().Format(logDateTimeFormat), from.Address, to.Address, amount)

	// Validate if wallets exist
	if from == nil || to == nil {
		return nil, fmt.Errorf("wallets can't be nil")
	}

	// Check if the from wallet has enough balance
	if from.Balance < amount+transactionFee {
		return nil, fmt.Errorf("insufficient balance in the wallet")
	}

	return &Bank{
		Tx: Tx{
			From: from,
			To:   to,
			Fee:  transactionFee,
		},
		Amount: amount,
	}, nil
}

// Process processes the bank transaction.
func (b *Bank) Process() string {
	// Check if From wallet has enough balance for the transaction + fee
	if b.From.Balance < (b.Amount * transactionFee) {
		return fmt.Sprintf("Insufficient balance in wallet %s", b.From.GetAddress())
	}

	// Subtract the amount from the From wallet and add it to the To wallet
	b.From.Balance -= b.Amount
	b.To.Balance += b.Amount

	//TODO: Disperse fee to the miner & dev wallet's (if applicable)

	return fmt.Sprintf("Transferred %f from %s to %s", b.Amount, b.From.Address, b.To.Address)
}

// Message is a transaction that represents a message sent from one user to another.
type Message struct {
	Tx
	Message string
}

// NewMessageTransaction creates a new message transaction.
func NewMessageTransaction(from *Wallet, to *Wallet, message string) (*Message, error) {
	fmt.Printf("[%s] Creating TX (MESSAGE) - FROM: %s, TO: %s, Message: %s\n", time.Now().Format(logDateTimeFormat), from.GetAddress(), to.GetAddress(), message)

	// Validate if wallets exist
	if from == nil || to == nil {
		return nil, fmt.Errorf("wallets can't be nil")
	}

	// Validate if there's a message
	if message == "" {
		return nil, fmt.Errorf("message can't be empty")
	}

	// Create the new Message transaction
	messageTx := &Message{
		Tx: Tx{
			From: from,
			To:   to,
			Fee:  transactionFee,
		},
		Message: message,
	}

	return messageTx, nil
}

// Process returns a string representation of the message.
func (m *Message) Process() string {
	return fmt.Sprintf("Message from %s to %s: %s", m.From.Name, m.To.Name, m.Message)
}

// Block is a block in the blockchain.
type Block struct {
	Index        int
	Timestamp    time.Time
	Transactions []Transaction
	Nonce        string
	Hash         string
	PreviousHash string
}

func (b *Block) String() string {
	return fmt.Sprintf("Index: %d, Timestamp: %s, Transactions: %d, Nonce: %s, Hash: %s, PreviousHash: %s", b.Index, b.Timestamp.Format(logDateTimeFormat), len(b.Transactions), b.Nonce, b.Hash, b.PreviousHash)
}

func (b *Block) calculateHash() string {
	// Convert the block to a string
	blockString := fmt.Sprintf("%d%s%s%s%s", b.Index, b.Timestamp.Format(logDateTimeFormat), b.Transactions, b.Nonce, b.PreviousHash)

	// Hash the string
	hash := sha256.Sum256([]byte(blockString))

	// Return the hash as a string

	return hex.EncodeToString(hash[:])
}

func (b *Block) save() error {
	filename := fmt.Sprintf("%s/%010d.json", dataFolder, b.Index)
	file, _ := json.MarshalIndent(b, "", " ")

	_ = ioutil.WriteFile(filename, file, 0644)
	fmt.Printf("[%s] Block [%d] saved to disk.\n", time.Now().Format(logDateTimeFormat), b.Index)

	return nil
}

func (b *Block) load(file string) error {
	blockData, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	json.Unmarshal(blockData, &b)

	return nil
}

// Blockchain is a blockchain.
type Blockchain struct {
	Blocks           []*Block
	TransactionQueue []Transaction
	mux              sync.Mutex
}

// NewBlockchain returns a new blockchain.
func NewBlockchain() *Blockchain {
	bc := &Blockchain{}

	err := bc.LoadExistingBlocks()
	if err != nil {
		log.Fatalln(err)
	}

	return bc
}

// DisplayStatus displays the status of the blockchain.
func (bc *Blockchain) DisplayStatus() {
	fmt.Printf("[%s] Blockchain Status: Blocks: %d, Transaction Queue: %d\n",
		time.Now().Format(logDateTimeFormat), len(bc.Blocks), len(bc.TransactionQueue))
}

// GenerateGenesisBlock generates the genesis block if there are no existing blocks.
func (bc *Blockchain) GenerateGenesisBlock() {
	if len(bc.Blocks) == 0 {
		fmt.Printf("[%s] Generating Genesis Block...\n", time.Now().Format(logDateTimeFormat))

		genesisBlock := &Block{
			Index:        0,
			Timestamp:    time.Now(),
			Transactions: []Transaction{}, // Genesis block usually does not contain transactions
			Nonce:        "",
			Hash:         "",
			PreviousHash: "", // There is no previous block for the genesis block
		}

		genesisBlock.Hash = bc.generateHash(genesisBlock)

		genesisBlock.save()

		bc.Blocks = append(bc.Blocks, genesisBlock)

		fmt.Printf("[%s] Genesis Block created with Hash [%s]\n", time.Now().Format(logDateTimeFormat), genesisBlock.Hash)
	}
}

// LoadExistingBlocks loads existing blocks from disk.
func (bc *Blockchain) LoadExistingBlocks() error {
	// Check if the dataFolder exists, if not, create it
	if _, err := os.Stat(dataFolder); os.IsNotExist(err) {
		err := os.MkdirAll(dataFolder, 0755)
		if err != nil {
			return fmt.Errorf("[%s] Error creating directory: %s", time.Now().Format(logDateTimeFormat), err)
		}
		fmt.Printf("[%s] Data directory '%s' created.\n", time.Now().Format(logDateTimeFormat), dataFolder)
	}

	files, _ := filepath.Glob(fmt.Sprintf("%s/*.json", dataFolder))
	if len(files) == 0 {
		fmt.Printf("[%s] No existing Blocks\n", time.Now().Format(logDateTimeFormat))

		// If no blocks loaded, generate Genesis Block.
		bc.GenerateGenesisBlock()

		return nil
	}

	fmt.Printf("[%s] Loading Blockchain [%d]...\n", time.Now().Format(logDateTimeFormat), len(files))

	// Load all the blocks
	for _, file := range files {
		block := Block{}
		err := block.load(file)
		if err != nil {
			return err
		}

		bc.Blocks = append(bc.Blocks, &block)
	}

	fmt.Printf("[%s] Done\n", time.Now().Format(logDateTimeFormat))

	return nil
}

// AddTransaction adds a transaction to the transaction queue.
func (bc *Blockchain) AddTransaction(transaction Transaction) {
	bc.mux.Lock()
	bc.TransactionQueue = append(bc.TransactionQueue, transaction)
	bc.mux.Unlock()
	fmt.Printf("[%s] Added TX to que: %v\n", time.Now().Format(logDateTimeFormat), transaction)
}

// Mine mines a new block with the given transactions and difficulty.
func (bc *Blockchain) Mine(block *Block, difficulty int) *Block {
	// Prepare difficulty string for comparison. It is a string consisting of `difficulty` number of zeros.
	prefix := strings.Repeat("0", difficulty)

	// Try different nonces until we get a hash with `difficulty` leading zeros.
	for i := 0; i < maxNonce; i++ {
		block.Nonce = strconv.Itoa(i)
		block.Hash = block.calculateHash()

		// Compare the prefix of the hash and our difficulty string.
		// If they are equal, we've mined a block.
		if strings.HasPrefix(block.Hash, prefix) {
			block.save() // Save the block to the disk
			bc.Blocks = append(bc.Blocks, block)
			bc.TransactionQueue = []Transaction{} // Clear the transaction queue

			fmt.Printf("[%s] Mined a new Block with Hash [%s]\n", time.Now().Format(logDateTimeFormat), block.Hash)
			break
		}
	}

	return block
}

// Mine mines a block.
func (bc *Blockchain) oldMine(block *Block, difficulty int) *Block {
	prefix := strings.Repeat("0", difficulty)
	for i := 0; ; i++ {
		block.Nonce = strconv.Itoa(i)
		block.Hash = bc.generateHash(block)
		if strings.HasPrefix(block.Hash, prefix) {
			fmt.Printf("[%s] Block [%d] mined with Hash [%s]\n", time.Now().Format(logDateTimeFormat), block.Index, block.Hash)
			return block
		}
	}
}

// Run runs the blockchain.
func (bc *Blockchain) Run(difficulty int) {
	// Create a ticker that fires every second
	statusTicker := time.NewTicker(time.Second)

	// Create a ticker that fires every 5 seconds
	blockTicker := time.NewTicker(blockTimeInSec * time.Second)

	// Run a goroutine that calls DisplayStatus() every second
	go func() {
		for range statusTicker.C {
			bc.DisplayStatus()
		}
	}()

	// Run a goroutine that checks for transactions every 5 seconds
	go func() {
		for range blockTicker.C {
			// check the queue for transactions
			bc.mux.Lock()
			if len(bc.TransactionQueue) == 0 {
				bc.mux.Unlock()
				continue
			}

			// create a new block
			index := len(bc.Blocks)
			previousHash := ""
			if index > 0 {
				previousHash = bc.Blocks[index-1].Hash
			}

			// block := &Block{index, time.Now(), bc.TransactionQueue, "", "", previousHash}
			// block = bc.Mine(block, difficulty)

			// Mine a new block with the current transaction queue and given difficulty
			block := &Block{Index: index, Timestamp: time.Now(), Transactions: bc.TransactionQueue, Nonce: "", Hash: "", PreviousHash: previousHash}
			bc.Mine(block, difficulty)

			// add the block to the blockchain
			bc.Blocks = append(bc.Blocks, block)
			bc.TransactionQueue = nil // clear the transaction queue

			// save the block to disk
			block.save()

			// unlock the mutex
			bc.mux.Unlock()
		}
	}()
}

// generateHash generates a hash for a block.
func (bc *Blockchain) generateHash(block *Block) string {
	record := strconv.Itoa(block.Index) + block.Timestamp.String() + block.Nonce + block.PreviousHash
	h := sha512.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}
