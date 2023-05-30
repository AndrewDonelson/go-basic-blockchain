package sdk

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

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
