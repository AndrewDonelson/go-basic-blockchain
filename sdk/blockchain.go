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

// State is the state of the blockchain.
type State struct {
}

// BlockchainPersistData represents the data that is persisted for a blockchain to disk.
// It contains an index for transactions (TXLookup), the current block index (CurrBlockIndex),
// and the next block index (NextBlockIndex).
type BlockchainPersistData struct {
	TXLookup       *Index
	CurrBlockIndex *int
	NextBlockIndex *int
}

// String returns a string representation of the BlockchainPersistData, including the TXLookup, CurrBlockIndex, and NextBlockIndex.
func (b *BlockchainPersistData) String() string {
	return fmt.Sprintf("TXLookup: %v, CurrBlockIndex: %v, NextBlockIndex: %v", b.TXLookup, b.CurrBlockIndex, b.NextBlockIndex)
}

// Blockchain is the main struct that represents the blockchain. It contains the configuration, blocks, transaction queue, transaction lookup, a mutex for concurrency control, the current and next block indices, and the average number of transactions per block.
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

// NewBlockchain creates a new instance of the Blockchain struct with the provided configuration.
// It attempts to load the blockchain data from disk, and if that fails, it creates a new blockchain.
// The returned Blockchain instance has the following fields initialized:
// - cfg: the provided configuration
// - Blocks: an empty slice of Block pointers
// - TransactionQueue: an empty slice of Transactions
// - TXLookup: a new TXLookupManager instance
// - CurrentBlockIndex: 0
// - NextBlockIndex: 1
// - AvgTxsPerBlock: 0
func NewBlockchain(cfg *Config) *Blockchain {
	//localStorage = NewLocalStorage(cfg.DataPath)

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

	return bc
}

// DisplayStatus displays the current status of the blockchain, including the number of blocks and the size
// of the transaction queue. It acquires a lock on the blockchain's mutex before accessing the blockchain data,
// and releases the lock before returning.
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

// GetConfig returns the configuration used to create the Blockchain instance.
func (bc *Blockchain) GetConfig() *Config {
	return bc.cfg
}

// Load loads the blockchain state from disk. It acquires a lock on the blockchain's mutex before loading
// the state, and releases the lock before returning. It loads the blockchain's transaction lookup index,
// current block index, and next block index from persistent storage.
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

// Save saves the blockchain state to disk. It acquires a lock on the blockchain's mutex before saving the
// state, and releases the lock before returning. It saves the blockchain's transaction lookup index, current
// block index, and next block index to persistent storage.
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

// createBLockchain creates a new blockchain. It sets the blockchain organization ID, app ID, admin user ID,
// dev asset ID, and miner asset ID. It then creates two wallets, one for the dev and one for the miner. It
// generates a coinbase transaction and a bank transaction, and adds them to the genesis block. Finally, it
// generates the genesis block for the new blockchain.
func (bc *Blockchain) createBLockchain() error {

	// Set the Blockchain Organization ID, App ID and Admin User ID as defined in the constants
	ThisBlockchainOrganizationID = NewBigInt(BlockhainOrganizationID)
	ThisBlockchainAppID = NewBigInt(BlockchainAppID)
	ThisBlockchainAdminUserID = NewBigInt(BlockchainAdminUserID)
	ThisBlockchainDevAssetID = NewBigInt(BlockchainDevAssetID)
	ThisBlockchainMinerID = NewBigInt(BlockchainMinerAssetID)

	genesisTxs := []Transaction{}

	// Create two wallets. One for the DEV and one for the Miner
	devWalletPW, err := GenerateRandomPassword()
	if err != nil {
		return err
	}

	devWallet, err := NewWallet(NewWalletOptions(ThisBlockchainOrganizationID, ThisBlockchainAppID, ThisBlockchainAdminUserID, ThisBlockchainDevAssetID, "Dev", devWalletPW, []string{"blockchain", "master"}))
	if err != nil {
		return err
	}

	// Creates a new blockchain dev wallet and sets the dev address in the blockchain configuration.
	// The dev wallet is created with a randomly generated password, which is printed to the console.
	// The dev wallet is then opened using the generated password.
	devWallet.Close(devWalletPW)
	devWallet.Open(devWalletPW)
	bc.cfg.DevAddress = devWallet.GetAddress()
	fmt.Printf("A Blockchain project Dev wallet was created for you with address [%s] and password [%s] (you can change this later)\n", bc.cfg.DevAddress, devWalletPW)

	minerWalletPW, err := GenerateRandomPassword()
	if err != nil {
		return err
	}

	// Creates a new miner wallet for the blockchain. The wallet is created with the specified
	// organization ID, app ID, admin user ID, miner asset ID, wallet name, and password. The
	// wallet is then closed and reopened, and the miner address is stored in the blockchain
	// configuration.
	minerWallet, err := NewWallet(NewWalletOptions(ThisBlockchainOrganizationID, ThisBlockchainAppID, ThisBlockchainAdminUserID, ThisBlockchainMinerID, "Miner", minerWalletPW, []string{"blockchain", "node", "miner"}))
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
// It creates a new block with the provided transactions and adds it to the blockchain.
// If there are no transactions, the genesis block will not contain any. The genesis block
// is the first block in the blockchain and has no previous hash.
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

// HasTransaction checks if a transaction with the given ID exists in the blockchain.
func (bc *Blockchain) HasTransaction(id *PUID) bool {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	// First, check the transaction queue
	for _, tx := range bc.TransactionQueue {
		if tx.GetID() == id.String() {
			return true
		}
	}

	// Then, check all blocks
	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.GetID() == id.String() {
				return true
			}
		}
	}

	return false
}

// LoadExistingBlocks loads any existing blocks from disk and appends them to the blockchain.
// If no existing blocks are found, it creates a new blockchain by generating a genesis block.
// This function returns an error if there is a problem loading the existing blocks.
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

// AddTransaction adds a new transaction to the transaction queue. This method is thread-safe.
// The transaction is hashed and appended to the TransactionQueue slice. A message is printed
// to the log with the current time and the added transaction.
func (bc *Blockchain) AddTransaction(transaction Transaction) {
	bc.mux.Lock()
	transaction.Hash()
	bc.TransactionQueue = append(bc.TransactionQueue, transaction)
	bc.mux.Unlock()
	fmt.Printf("[%s] Added TX to que: %v\n", time.Now().Format(logDateTimeFormat), transaction)
}

// Mine attempts to mine a new block for the blockchain. It takes a block and a difficulty
// parameter, and tries different nonces until it finds a hash with the required number of
// leading zeros. Once a valid hash is found, the block is saved to disk and appended to
// the blockchain. The transaction queue is also cleared after a successful mining operation.
// This function returns the mined block.
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

// VerifySignature verifies the signature of the given transaction. It takes the transaction
// and returns an error if the signature is invalid.
func (bc *Blockchain) VerifySignature(tx Transaction) error {
	_, err := tx.Verify([]byte(tx.GetSenderWallet().PublicPEM()), tx.GetSignature())
	return err
}

// Errors/Issues:
// - cannot use tx.GetSenderWallet().PublicKey (variable of type *ecdsa.PublicKey) as []byte value in argument to VerifySignature
// - invalid operation: cannot compare err != nil (mismatched types bool and untyped nil)

// Run is a long-running function that manages the blockchain. It creates two tickers, one that fires every
// second and one that fires every 5 seconds. The first ticker calls DisplayStatus() to display the current
// status of the blockchain. The second ticker checks the transaction queue and creates a new block if there
// are any transactions. The new block is mined, added to the blockchain, and the transaction queue is cleared.
// The block is also saved to disk.
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

// generateHash generates a SHA-512 hash for the given block. The hash is calculated
// by concatenating the block's index, timestamp, nonce, and previous hash, and then
// hashing the resulting string using the SHA-512 algorithm.
func (bc *Blockchain) generateHash(block *Block) string {
	record := block.Index.Text(10) + block.Timestamp.String() + block.Nonce + block.PreviousHash
	h := sha512.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}
