// Package sdk is a software development kit for building blockchain applications.
// File sdk/blockchain.go - The main Blockchain file
package sdk

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type State struct {
}

// BlockchainPersistData is the data that is persisted for a blockchain to disk.
type BlockchainPersistData struct {
	TXLookup       *Index
	CurrBlockIndex *int
	NextBlockIndex *int
}

func (b *BlockchainPersistData) String() string {
	return fmt.Sprintf("TXLookup: %v, CurrBlockIndex: %v, NextBlockIndex: %v", b.TXLookup, b.CurrBlockIndex, b.NextBlockIndex)
}

// Blockchain is a blockchain.
type Blockchain struct {
	cfg               *Config          // Config is the configuration for the blockchain.
	Blocks            []*Block         // Blocks is a slice of blocks in the blockchain.
	TransactionQueue  []Transaction    // TransactionQueue is a queue of transactions to be added to the blockchain.
	TXLookup          *TXLookupManager // TXLookup is a map of Block Number/Index (Key) and Transaction ID (Value) that is stored in memory and persisted to disk.
	mux               sync.Mutex       // mux is a mutex to protect concurrent access to the blockchain.
	CurrentBlockIndex int              // CurrentBlockIndex is the current block index.
	NextBlockIndex    int              // NextBlockIndex is the next block index.
	AvgTxsPerBlock    float64          // AvgTxsPerBlock is the average number of transactions per block.
}

// NewBlockchain returns a new blockchain.
func NewBlockchain(cfg *Config) *Blockchain {
	localStorage = NewLocalStorage(cfg.DataPath)

	bc := &Blockchain{
		cfg:               cfg,
		Blocks:            []*Block{},
		TransactionQueue:  []Transaction{},
		TXLookup:          NewTXLookupManager(),
		CurrentBlockIndex: 0,
		NextBlockIndex:    1,
		AvgTxsPerBlock:    0,
	}

	err := bc.Load()
	if err != nil {
		fmt.Printf("Error loading blockchain...\n%v\n. Creating new blockchain.\n", err)
		bc.createBLockchain()
	}

	// err = bc.LoadExistingBlocks()
	// if err != nil {
	// 	log.Fatalln(err)
	// }

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

func (bc *Blockchain) Load() error {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	data := &BlockchainPersistData{}

	// Load the state from disk
	err := localStorage.Get("state", data)
	if err != nil {
		return err
	}

	bc.TXLookup.index.Set(data.TXLookup)
	bc.CurrentBlockIndex = *data.CurrBlockIndex
	bc.NextBlockIndex = *data.NextBlockIndex

	return err
}

func (bc *Blockchain) Save() error {
	data := &BlockchainPersistData{
		TXLookup:       bc.TXLookup.index.Get(),
		CurrBlockIndex: &bc.CurrentBlockIndex,
		NextBlockIndex: &bc.NextBlockIndex,
	}

	bc.mux.Lock()
	defer bc.mux.Unlock()

	// Save the state to disk
	err := localStorage.Set("state", data)
	return err
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
	fmt.Printf("A Blockchain project Dev wallet was created for you with address [%s] and password [%s] (you can change this later)\n", bc.cfg.DevAddress, devWalletPW)

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
	fmt.Printf("A Node miner wallet was created for you with address [%s] and password [%s] (you can change this later)\n", bc.cfg.MinerAddress, minerWalletPW)

	// Create the Coinbase Transaction
	cbTX, err := NewCoinbaseTransaction(devWallet, devWallet, bc.cfg)
	if err != nil {
		return err
	}

	// Set the Dev Wallet balance to config TokenCount
	err = devWallet.SetData("balance", bc.cfg.TokenCount)
	if err != nil {
		return err
	}

	// Sign the Coinbase Transaction with the DEV Wallet
	cbTX.Signature, err = cbTX.Sign([]byte(devWallet.PrivatePEM()))
	if err != nil {
		return err
	}
	fmt.Printf("A Coinbase Transaction was created and set Dev wallet Balance to [%d] tokens)\n", cbTX.TokenCount)

	// Add the Coinbase Transaction to the genesis block
	genesisTxs = append(genesisTxs, cbTX)

	// Create a Bank Transaction and send fundWalletAmount to the Miner Wallet
	bankTX, err := NewBankTransaction(devWallet, minerWallet, bc.cfg.FundWalletAmount)
	if err != nil {
		return err
	}

	// Sign the Bank Transaction with the DEV Wallet
	bankTX.Signature, err = bankTX.Sign([]byte(devWallet.PrivatePEM()))
	if err != nil {
		return err
	}
	fmt.Printf("A Bank Transaction was created sent [%0.4f] tokens to the miner wallet)\n", bankTX.Amount)

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
			Index:        *big.NewInt(0),
			Timestamp:    time.Now(),
			Transactions: []Transaction{}, // Genesis block usually does not contain transactions
			Nonce:        "",
			Hash:         "",
			PreviousHash: "", // There is no previous block for the genesis block
		}

		// if we have any transactions, add them to the genesis block
		if len(txs) > 0 {
			genesisBlock.Transactions = txs
			// clear the transaction queue
			bc.TransactionQueue = nil // clear the transaction queue
		}

		genesisBlock.Hash = bc.generateHash(genesisBlock)

		bc.Mine(genesisBlock, 1)

		//genesisBlock.save()

		// add the block to the blockchain
		bc.Blocks = append(bc.Blocks, genesisBlock)

		// add the block transactions to the transaction lookup
		err := bc.TXLookup.Add(genesisBlock)
		if err != nil {
			fmt.Printf("[%s] Error adding block to TXLookup: %v\n", time.Now().Format(logDateTimeFormat), err)
		}

		fmt.Printf("[%s] Genesis Block created with Hash [%s]\n", time.Now().Format(logDateTimeFormat), genesisBlock.Hash)

		err = bc.Save()
		if err != nil {
			fmt.Printf("[%s] Error saving blockchain state: %v\n", time.Now().Format(logDateTimeFormat), err)
		}
	}
}

// createDataFolders creates the data folders if they dont exist
// func (bc *Blockchain) createDataFolders() {
// 	createFolder(dataFolder)
// 	createFolder(blockFolder)
// }

// LoadExistingBlocks loads existing blocks from disk.
func (bc *Blockchain) LoadExistingBlocks() error {
	//bc.createDataFolders()

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
	// for _, file := range files {
	// 	block := Block{}
	// 	err := block.load(file)
	// 	//err := block.load(file)
	// 	if err != nil {
	// 		return err
	// 	}

	// 	bc.Blocks = append(bc.Blocks, &block)
	// }

	fmt.Printf("[%s] Done\n", time.Now().Format(logDateTimeFormat))

	return nil
}

// AddTransaction adds a transaction to the transaction queue. All transactions in the queue will be added to the next block based ont he Blockchain's BlockInterval.
func (bc *Blockchain) AddTransaction(transaction Transaction) {
	bc.mux.Lock()
	transaction.Hash()
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
		block.Hash = block.hash()

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
	_, err := tx.Verify([]byte(tx.GetSenderWallet().PublicPEM()), tx.GetSignature())
	return err
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

			// Mine a new block with the current transaction queue and given difficulty
			block := &Block{Index: *big.NewInt(int64(index)), Timestamp: time.Now(), Transactions: bc.TransactionQueue, Nonce: "", Hash: "", PreviousHash: previousHash}
			bc.Mine(block, difficulty)

			// add the block to the blockchain
			//bc.Blocks = append(bc.Blocks, block)

			// clear the transaction queue
			//bc.TransactionQueue = nil // clear the transaction queue

			// add the block transactions to the transaction lookup
			err := bc.TXLookup.Add(block)
			if err != nil {
				fmt.Printf("[%s] Error adding block to TXLookup: %v\n", time.Now().Format(logDateTimeFormat), err)
			}

			// save the block to disk
			block.save()

			err = bc.Save()
			if err != nil {
				fmt.Printf("[%s] Error saving blockchain state: %v\n", time.Now().Format(logDateTimeFormat), err)
			}

			// unlock the mutex
			bc.mux.Unlock()
		}
	}()
}

// generateHash generates a hash for a block.
func (bc *Blockchain) generateHash(block *Block) string {
	record := block.Index.Text(10) + block.Timestamp.String() + block.Nonce + block.PreviousHash
	h := sha512.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}
