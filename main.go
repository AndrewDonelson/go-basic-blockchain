package main

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
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/pborman/uuid"
	"golang.org/x/crypto/pbkdf2"
)

const (
	TransactionFee   = 0.05   // transaction fee is 5 hunderths of a coin (a nickle-ish)
	saltSize         = 16     // salt size is 16 bytes
	fundWalletAmount = 100.0  // default Amount to fund new wallets is 100 coins
	blockTimeInSec   = 5      // block time is 5 seconds
	dataFolder       = "data" // data folder is the folder where the blockchain data is stored
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
	return &Wallet{
		ID:         uuid.New(),
		Name:       name,
		Tags:       tags,
		PrivateKey: privateKeyBytes,
		PublicKey:  publicKeyBytes,
		Balance:    fundWalletAmount,
	}, nil
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
	// Check if the from wallet has enough balance
	if from.Balance < amount+TransactionFee {
		return nil, fmt.Errorf("insufficient balance in the wallet")
	}

	return &Bank{
		Tx: Tx{
			From: from,
			To:   to,
			Fee:  TransactionFee,
		},
		Amount: amount,
	}, nil
}

// Process processes the bank transaction.
func (b *Bank) Process() string {
	// Check if From wallet has enough balance for the transaction
	if b.From.Balance < b.Amount {
		return fmt.Sprintf("Insufficient balance in wallet %s", b.From.Address)
	}

	// Subtract the amount from the From wallet and add it to the To wallet
	b.From.Balance -= b.Amount
	b.To.Balance += b.Amount

	return fmt.Sprintf("Transferred %f from %s to %s", b.Amount, b.From.Address, b.To.Address)
}

// Message is a transaction that represents a message sent from one user to another.
type Message struct {
	Tx
	Message string
}

// NewMessageTransaction creates a new message transaction.
func NewMessageTransaction(from *Wallet, to *Wallet, message string) (*Message, error) {
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
			Fee:  TransactionFee,
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

// Blockchain is a blockchain.
type Blockchain struct {
	Blocks           []*Block
	TransactionQueue []Transaction
	mux              sync.Mutex
}

// NewBlockchain returns a new blockchain.
func NewBlockchain() *Blockchain {
	bc := &Blockchain{}
	bc.LoadExistingBlocks()
	return bc
}

func (bc *Blockchain) DisplayStatus() {
	fmt.Println("Blockchain Status:")
	fmt.Println("- Blocks:", len(bc.Blocks))
	fmt.Println("- Transaction Queue:", len(bc.TransactionQueue))
}

// LoadExistingBlocks loads existing blocks from disk.
func (bc *Blockchain) LoadExistingBlocks() {
	files, _ := filepath.Glob("*.json")
	for _, file := range files {
		blockData, _ := ioutil.ReadFile(file)
		var block Block
		json.Unmarshal(blockData, &block)
		bc.Blocks = append(bc.Blocks, &block)
	}
}

// AddTransaction adds a transaction to the transaction queue.
func (bc *Blockchain) AddTransaction(transaction Transaction) {
	bc.mux.Lock()
	bc.TransactionQueue = append(bc.TransactionQueue, transaction)
	bc.mux.Unlock()
}

// Run runs the blockchain.
func (bc *Blockchain) Run(difficulty int) {
	for {
		// check the queue for transactions
		bc.mux.Lock()
		if len(bc.TransactionQueue) == 0 {
			bc.mux.Unlock()
			time.Sleep(5 * time.Second)
			continue
		}

		// create a new block
		index := len(bc.Blocks)
		previousHash := ""
		if index > 0 {
			previousHash = bc.Blocks[index-1].Hash
		}

		block := &Block{index, time.Now(), bc.TransactionQueue, "", "", previousHash}
		block = bc.Mine(block, difficulty)

		// add the block to the blockchain
		bc.Blocks = append(bc.Blocks, block)
		bc.TransactionQueue = nil // clear the transaction queue

		// save the block to disk
		filename := fmt.Sprintf("%s/%010d.json", dataFolder, index)
		file, _ := json.MarshalIndent(block, "", " ")

		_ = ioutil.WriteFile(filename, file, 0644)
		fmt.Printf("Block [%d] saved to disk.\n", block.Index)

		// unlock the mutex and wait for 5 seconds
		bc.mux.Unlock()
		time.Sleep(5 * time.Second) // wait for 5 seconds
	}
}

// Mine mines a block.
func (bc *Blockchain) Mine(block *Block, difficulty int) *Block {
	prefix := strings.Repeat("0", difficulty)
	for i := 0; ; i++ {
		block.Nonce = strconv.Itoa(i)
		block.Hash = bc.generateHash(block)
		if strings.HasPrefix(block.Hash, prefix) {
			fmt.Printf("Block [%d] mined with Hash [%s]\n", block.Index, block.Hash)
			return block
		}
	}
}

// generateHash generates a hash for a block.
func (bc *Blockchain) generateHash(block *Block) string {
	record := strconv.Itoa(block.Index) + block.Timestamp.String() + block.Nonce + block.PreviousHash
	h := sha512.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

func main() {
	// Create a blockchain instance
	bc := NewBlockchain()

	go bc.Run(1)

	// This is to keep the main goroutine alive. Remove it if not necessary
	select {}
}
