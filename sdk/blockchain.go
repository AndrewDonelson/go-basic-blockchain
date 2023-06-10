// Package sdk is a software development kit for building blockchain applications.
// File sdk/blockchain.go - The main Blockchain file
package sdk

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"log"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type State struct {
}

// Blockchain is a blockchain.
type Blockchain struct {
	cfg               *Config
	Blocks            []*Block
	TransactionQueue  []Transaction
	mux               sync.Mutex
	CurrentBlockIndex int
	NextBlockIndex    int
	AvgTxsPerBlock    float64
}

// NewBlockchain returns a new blockchain.
func NewBlockchain(cfg *Config) *Blockchain {
	bc := &Blockchain{
		cfg:               cfg,
		Blocks:            []*Block{},
		TransactionQueue:  []Transaction{},
		CurrentBlockIndex: 0,
		NextBlockIndex:    1,
		AvgTxsPerBlock:    0,
	}

	err := bc.LoadExistingBlocks()
	if err != nil {
		log.Fatalln(err)
	}

	return bc
}

// DisplayStatus displays the status of the blockchain.
func (bc *Blockchain) DisplayStatus() {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	staticBlocksLen := len(bc.Blocks)
	staticTransactionQueueLen := len(bc.TransactionQueue)

	// Check if the length of Blocks or TransactionQueue has changed
	if staticBlocksLen != len(bc.Blocks) || staticTransactionQueueLen != len(bc.TransactionQueue) {
		fmt.Printf("[%s] Blockchain Activity: Blocks: %d, Transaction Queue: %d\n",
			time.Now().Format(logDateTimeFormat), len(bc.Blocks), len(bc.TransactionQueue))
	}
}

// CreateBLockchain creates a new blockchain.
func (bc *Blockchain) createBLockchain() error {

	genesisTxs := []Transaction{}

	// Create two wallets. One for the DEV and one for the Miner
	devWalletPW, err := GenerateRandomPassword()
	if err != nil {
		return err
	}
	devWallet, err := NewWallet("Dev", devWalletPW, []string{"blockchain", "master"})
	if err != nil {
		return err
	}
	devWallet.Close(devWalletPW)
	devWallet.Open(devWalletPW)
	bc.cfg.DevAddress = devWallet.GetAddress()

	minerWalletPW, err := GenerateRandomPassword()
	if err != nil {
		return err
	}
	minerWallet, err := NewWallet("Wallet2", minerWalletPW, []string{"tag3", "tag4"})
	if err != nil {
		return err
	}
	minerWallet.Close(minerWalletPW)
	minerWallet.Open(minerWalletPW)
	bc.cfg.MinerAddress = minerWallet.GetAddress()

	// Create the Coinbase Transaction
	cbTX, err := NewCoinbaseTransaction(devWallet, devWallet, bc.cfg)
	if err != nil {
		return err
	}

	// Sign the Coinbase Transaction with the DEV Wallet
	err = devWallet.SignTransaction(cbTX)
	if err != nil {
		return err
	}

	// Add the Coinbase Transaction to the genesis block
	genesisTxs = append(genesisTxs, cbTX)

	// Create a Bank Transaction and send fundWalletAmount to the Miner Wallet
	bankTX, err := NewBankTransaction(devWallet, minerWallet, bc.cfg.FundWalletAmount)
	if err != nil {
		return err
	}

	// Sign the Banke Transaction with the DEV Wallet
	err = devWallet.SignTransaction(bankTX)
	if err != nil {
		return err
	}

	// Add the Coinbase Transaction to the genesis block
	genesisTxs = append(genesisTxs, bankTX)

	// Create the genesis block
	bc.GenerateGenesisBlock(genesisTxs)

	return nil
}

// GenerateGenesisBlock generates the genesis block if there are no existing blocks.
func (bc *Blockchain) GenerateGenesisBlock(txs []Transaction) {
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

		// if we have any transactions, add them to the genesis block
		if len(txs) > 0 {
			genesisBlock.Transactions = txs
		}

		genesisBlock.Hash = bc.generateHash(genesisBlock)

		genesisBlock.save()

		bc.Blocks = append(bc.Blocks, genesisBlock)

		fmt.Printf("[%s] Genesis Block created with Hash [%s]\n", time.Now().Format(logDateTimeFormat), genesisBlock.Hash)
	}
}

// createDataFolders creates the data folders if they dont exist
func (bc *Blockchain) createDataFolders() {
	createFolder(dataFolder)
	createFolder(blockFolder)
}

// LoadExistingBlocks loads existing blocks from disk.
func (bc *Blockchain) LoadExistingBlocks() error {
	bc.createDataFolders()

	files, _ := filepath.Glob(fmt.Sprintf("%s/*.json", blockFolder))
	if len(files) == 0 {
		fmt.Printf("[%s] No existing Blocks\n", time.Now().Format(logDateTimeFormat))

		// If no blocks loaded, generate Genesis Block.
		//bc.GenerateGenesisBlock([]Transaction{})
		bc.createBLockchain()

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

// AddTransaction adds a transaction to the transaction queue. All transactions in the queue will be added to the next block based ont he Blockchain's BlockInterval.
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

			fmt.Printf("[%s] Mined a new Block with [%d] TXs & Hash [%s]\n", time.Now().Format(logDateTimeFormat), len(block.Transactions), block.Hash)
			break
		}
	}

	return block
}

// VerifySignature verifies the signature of a transaction using the sender's public key.
func (bc *Blockchain) VerifySignature(tx Transaction) error {
	senderPublicKey, err := tx.GetSenderWallet().PublicKey()
	if err != nil {
		return err
	}

	if !VerifySignature(tx.GetHash(), tx.GetSignature(), senderPublicKey) {
		return fmt.Errorf("failed to verify signature")
	}

	return nil
}

// Errors/Issues:
// - cannot use tx.GetSenderWallet().PublicKey (variable of type *ecdsa.PublicKey) as []byte value in argument to VerifySignature
// - invalid operation: cannot compare err != nil (mismatched types bool and untyped nil)

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
