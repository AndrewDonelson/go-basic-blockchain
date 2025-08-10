// Package sdk is a software development kit for building blockchain applications.
// File sdk/blockchain.go - The main Blockchain file

package sdk

import (
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/helios/algorithm"
	"github.com/AndrewDonelson/go-basic-blockchain/internal/helios/difficulty"
	"github.com/AndrewDonelson/go-basic-blockchain/internal/helios/sidechain"
	"github.com/AndrewDonelson/go-basic-blockchain/internal/helios/validation"
	"github.com/AndrewDonelson/go-basic-blockchain/internal/progress"
)

// State represents the current state of the blockchain.
type State struct {
	// Add state-related fields here if needed
}

// BlockchainPersistData represents the data that is persisted for a blockchain to disk.
type BlockchainPersistData struct {
	TXLookup       *Index `json:"tx_lookup"`
	CurrBlockIndex *int   `json:"current_block_index"`
	NextBlockIndex *int   `json:"next_block_index"`
}

// String returns a string representation of the BlockchainPersistData.
func (b *BlockchainPersistData) String() string {
	return fmt.Sprintf("TXLookup: %v, CurrBlockIndex: %v, NextBlockIndex: %v", b.TXLookup, b.CurrBlockIndex, b.NextBlockIndex)
}

// Blockchain is the main struct that represents the blockchain.
type Blockchain struct {
	cfg               *Config          // Configuration for the blockchain
	Blocks            []*Block         // Slice of blocks in the blockchain
	TransactionQueue  []Transaction    // Queue of transactions to be added to the blockchain
	TXLookup          *TXLookupManager // Map of Block Number/Index (Key) and Transaction ID (Value)
	mux               sync.Mutex       // Mutex to protect concurrent access to the blockchain
	CurrentBlockIndex int              // Current block index
	NextBlockIndex    int              // Next block index
	AvgTxsPerBlock    float64          // Average number of transactions per block
	State             *State           // Current state of the blockchain

	// Helios integration
	heliosAlgorithm    *algorithm.HeliosAlgorithm     // Helios proof-of-work algorithm
	heliosValidator    *validation.ProofValidator     // Helios proof validator
	difficultyAdjuster *difficulty.DifficultyAdjuster // Helios difficulty adjustment
	sidechainRouter    *sidechain.ProtocolRouter      // Sidechain protocol router
	useHeliosMining    bool                           // Flag to enable/disable Helios mining

	// Progress indicator
	progressIndicator *progress.ProgressIndicator

	// Menu state
	menuActive bool
	menuMutex  sync.RWMutex
}

// NewBlockchain creates a new instance of the Blockchain struct with the provided configuration.
func NewBlockchain(cfg *Config) *Blockchain {

	// If no config is provided, create a default one
	if cfg == nil {
		cfg = NewConfig()
	}

	bc := &Blockchain{
		cfg:               cfg,
		Blocks:            []*Block{},
		TransactionQueue:  []Transaction{},
		TXLookup:          NewTXLookupManager(),
		CurrentBlockIndex: 0,
		NextBlockIndex:    1,
		AvgTxsPerBlock:    0,
		State:             &State{},
		useHeliosMining:   true, // Enable Helios mining by default
		progressIndicator: progress.NewProgressIndicator(),
	}

	// Initialize Helios components
	heliosConfig := algorithm.TestHeliosConfig() // Use test config for faster mining
	bc.heliosAlgorithm = algorithm.NewHeliosAlgorithm(heliosConfig)
	bc.heliosValidator = validation.NewProofValidator(bc.heliosAlgorithm)

	difficultyConfig := difficulty.DefaultDifficultyAdjustmentConfig()
	bc.difficultyAdjuster = difficulty.NewDifficultyAdjuster(difficultyConfig)

	bc.sidechainRouter = sidechain.NewProtocolRouter()

	// Set up sidechain router callbacks
	bc.sidechainRouter.SetCallbacks(
		bc.onTransactionValidated,
		bc.onTransactionFailed,
		bc.onRollupCreated,
	)

	// Ensure local storage is initialized
	if !LocalStorageAvailable() {
		err := NewLocalStorage(cfg.DataPath)
		if err != nil {
			log.Printf("Error initializing local storage: %v", err)
			return nil
		}
	}

	// Ensure the node is initialized
	if GetNode() == nil {
		log.Println("Creating default node for blockchain")
		nodeOpts := DefaultNodeOptions()
		nodeOpts.Config = cfg

		err := NewNode(nodeOpts)
		if err != nil {
			log.Printf("Error creating default node: %v", err)
			return nil
		}
	}

	err := bc.Load()
	if err != nil {
		log.Println("No existing blockchain state found, creating new blockchain")
		err = bc.createBlockchain()
		if err != nil {
			log.Printf("Error creating blockchain: %v", err)
			return nil
		}
	}

	// Load existing blocks from disk
	err = bc.LoadExistingBlocks()
	if err != nil {
		log.Printf("Error loading existing blocks: %v", err)
	}

	if len(bc.Blocks) == 0 {
		log.Println("No blocks found, creating genesis block")
		bc.GenerateGenesisBlock([]Transaction{})
	}

	log.Printf("Blockchain initialized with %d blocks", len(bc.Blocks))
	return bc
}

// DisplayStatus displays the current status of the blockchain.
func (bc *Blockchain) DisplayStatus() {
	// Check if menu is active - if so, don't display ANY status at all
	bc.menuMutex.RLock()
	menuActive := bc.menuActive
	bc.menuMutex.RUnlock()

	if menuActive {
		return // Exit early - don't display ANY status when menu is active
	}

	bc.mux.Lock()
	defer bc.mux.Unlock()

	staticBlocksLen := len(bc.Blocks)
	staticTransactionQueueLen := len(bc.TransactionQueue)

	if staticBlocksLen != len(bc.Blocks) || staticTransactionQueueLen != len(bc.TransactionQueue) {
		log.Printf("Blockchain Activity: Blocks: %d, Transaction Queue: %d\n",
			len(bc.Blocks), len(bc.TransactionQueue))
	}

	// Update progress indicator only if not paused
	if bc.progressIndicator != nil {
		// Check if progress indicator is paused (menu is active)
		if !bc.progressIndicator.IsPaused() {
			status := progress.BlockchainStatus{
				IsMining:    true, // Assume mining is active
				BlockCount:  len(bc.Blocks),
				TxQueueSize: len(bc.TransactionQueue),
				Difficulty:  bc.cfg.Difficulty,
				HashRate:    0, // TODO: Calculate actual hash rate
				LastBlock:   "",
				Peers:       0, // TODO: Get actual peer count
				IsSynced:    true,
				Uptime:      0, // TODO: Calculate uptime
			}

			if len(bc.Blocks) > 0 {
				status.LastBlock = bc.Blocks[len(bc.Blocks)-1].Hash
			}

			bc.progressIndicator.UpdateStatus(status)
		}
	}
}

// GetConfig returns the configuration used to create the Blockchain instance.
func (bc *Blockchain) GetConfig() *Config {
	return bc.cfg
}

// Load loads the blockchain state from disk.
func (bc *Blockchain) Load() error {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	data := &BlockchainPersistData{}

	err := localStorage.Get("state", data)
	if err != nil {
		return err
	}

	if bc.TXLookup == nil {
		bc.TXLookup = NewTXLookupManager()
	}

	if data.TXLookup != nil {
		if err := bc.TXLookup.Set(data.TXLookup); err != nil {
			log.Printf("Error setting TX lookup: %v", err)
		}
	}

	if data.CurrBlockIndex != nil {
		bc.CurrentBlockIndex = *data.CurrBlockIndex
	}

	if data.NextBlockIndex != nil {
		bc.NextBlockIndex = *data.NextBlockIndex
	}

	return nil
}

// Save saves the blockchain state to disk.
func (bc *Blockchain) Save() error {
	data := &BlockchainPersistData{
		TXLookup:       bc.TXLookup.index.Get(),
		CurrBlockIndex: &bc.CurrentBlockIndex,
		NextBlockIndex: &bc.NextBlockIndex,
	}

	bc.mux.Lock()
	defer bc.mux.Unlock()

	return localStorage.Set("state", data)
}

// createBlockchain initializes a new blockchain with a genesis block and sets up
// the necessary wallets and transactions. It performs the following steps:
// 1. Initializes blockchain organization, application, admin user, developer asset, and miner asset IDs.
// 2. Creates a developer wallet with a randomly generated password and assigns it to the blockchain configuration.
// 3. Creates a miner wallet with a randomly generated password and assigns it to the blockchain configuration.
// 4. Generates a coinbase transaction to set the initial balance of the developer wallet.
// 5. Generates a bank transaction to fund the miner wallet with a specified amount.
// 6. Generates the genesis block with the created transactions.
//
// Returns an error if any step in the process fails.
func (bc *Blockchain) createBlockchain() error {
	LogInfof("Initializing new blockchain...")
	ThisBlockchainOrganizationID = NewBigInt(BlockhainOrganizationID)
	ThisBlockchainAppID = NewBigInt(BlockchainAppID)
	ThisBlockchainAdminUserID = NewBigInt(BlockchainAdminUserID)
	ThisBlockchainDevAssetID = NewBigInt(BlockchainDevAssetID)
	ThisBlockchainMinerID = NewBigInt(BlockchainMinerAssetID)

	genesisTxs := []Transaction{}

	devWalletPW, err := GenerateRandomPassword()
	if err != nil {
		return err
	}

	devWallet, err := NewWallet(NewWalletOptions(ThisBlockchainOrganizationID, ThisBlockchainAppID, ThisBlockchainAdminUserID, ThisBlockchainDevAssetID, "Dev", devWalletPW, []string{"blockchain", "master"}))
	if err != nil {
		return fmt.Errorf("failed to create dev wallet: %v", err)
	}

	err = devWallet.Close(devWalletPW)
	if err != nil {
		return fmt.Errorf("failed to close dev wallet: %v", err)
	}

	err = devWallet.Open(devWalletPW)
	if err != nil {
		return fmt.Errorf("failed to open dev wallet: %v", err)
	}

	bc.cfg.DevAddress = devWallet.GetAddress()
	LogVerbosef("Dev wallet created: %s (password: %s)", bc.cfg.DevAddress, devWalletPW)

	minerWalletPW, err := GenerateRandomPassword()
	if err != nil {
		return err
	}

	minerWallet, err := NewWallet(NewWalletOptions(ThisBlockchainOrganizationID, ThisBlockchainAppID, ThisBlockchainAdminUserID, ThisBlockchainMinerID, "Miner", minerWalletPW, []string{"blockchain", "node", "miner"}))
	if err != nil {
		return fmt.Errorf("failed to create miner wallet: %v", err)
	}

	minerWallet.Close(minerWalletPW)
	if err != nil {
		return fmt.Errorf("failed to close miner wallet: %v", err)
	}

	if err := minerWallet.Open(minerWalletPW); err != nil {
		log.Printf("Error opening miner wallet: %v", err)
	}
	if err != nil {
		return fmt.Errorf("failed to open miner wallet: %v", err)
	}

	bc.cfg.MinerAddress = minerWallet.GetAddress()
	LogVerbosef("Miner wallet created: %s (password: %s)", bc.cfg.MinerAddress, minerWalletPW)

	cbTX, err := NewCoinbaseTransaction(devWallet, devWallet, bc.cfg)
	if err != nil {
		return err
	}

	err = devWallet.SetData("balance", bc.cfg.TokenCount)
	if err != nil {
		return err
	}

	cbTX.Signature, err = cbTX.Sign([]byte(devWallet.PrivatePEM()))
	if err != nil {
		return err
	}
	LogVerbosef("Coinbase transaction created: %d tokens allocated", cbTX.TokenCount)

	genesisTxs = append(genesisTxs, cbTX)

	bankTX, err := NewBankTransaction(devWallet, minerWallet, bc.cfg.FundWalletAmount)
	if err != nil {
		return err
	}

	bankTX.Signature, err = bankTX.Sign([]byte(devWallet.PrivatePEM()))
	if err != nil {
		return err
	}
	LogVerbosef("Bank transaction created: %.2f tokens transferred to miner", bankTX.Amount)

	genesisTxs = append(genesisTxs, bankTX)

	bc.GenerateGenesisBlock(genesisTxs)

	return nil
}

// GenerateGenesisBlock generates the genesis block if there are no existing blocks.
func (bc *Blockchain) GenerateGenesisBlock(txs []Transaction) {
	if len(bc.Blocks) == 0 {
		log.Println("Generating Genesis Block...")

		genesisBlock := NewBlock(txs, "")
		genesisBlock.Index = *big.NewInt(0)
		genesisBlock.Hash = bc.generateHash(genesisBlock)

		bc.Mine(genesisBlock, 1)

		err := genesisBlock.save()
		if err != nil {
			log.Printf("Error saving genesis block: %v\n", err)
		}

		bc.Blocks = append(bc.Blocks, genesisBlock)

		err = bc.TXLookup.Add(genesisBlock)
		if err != nil {
			log.Printf("Error adding block to TXLookup: %v\n", err)
		}

		log.Printf("Genesis Block created with Hash [%s]\n", genesisBlock.Hash)

		err = bc.Save()
		if err != nil {
			log.Printf("Error saving blockchain state: %v\n", err)
		}
	}
}

// HasTransaction checks if a transaction with the given ID exists in the blockchain.
func (bc *Blockchain) HasTransaction(id *PUID) bool {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	for _, tx := range bc.TransactionQueue {
		if tx.GetID() == id.String() {
			return true
		}
	}

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
func (bc *Blockchain) LoadExistingBlocks() error {
	// Use the configured data path instead of the relative blockFolder
	blocksPath := filepath.Join(bc.cfg.DataPath, "blocks")

	// Look for both .json and .jso files (in case of truncated names)
	jsonFiles, _ := filepath.Glob(filepath.Join(blocksPath, "*.json"))
	jsoFiles, _ := filepath.Glob(filepath.Join(blocksPath, "*.jso"))
	files := append(jsonFiles, jsoFiles...)
	if len(files) == 0 {
		log.Printf("No existing blocks found in %s\n", blocksPath)
		return nil
	}

	log.Printf("Loading %d existing blocks...", len(files))

	// Sort files by block index to load them in order
	sort.Strings(files)

	for _, file := range files {
		log.Printf("Processing block file: %s", file)
		// Extract block index from filename (e.g., "0.json" -> 0)
		filename := filepath.Base(file)
		if !strings.HasSuffix(filename, ".json") && !strings.HasSuffix(filename, ".jso") {
			log.Printf("Skipping file (not .json/.jso): %s", filename)
			continue
		}

		blockIndexStr := strings.TrimSuffix(strings.TrimSuffix(filename, ".json"), ".jso")
		blockIndex, err := strconv.Atoi(blockIndexStr)
		if err != nil {
			log.Printf("Invalid block filename: %s", filename)
			continue
		}

		// Read the JSON file directly
		fileData, err := os.ReadFile(file)
		if err != nil {
			log.Printf("Error reading block file %s: %v", file, err)
			continue
		}

		// Parse the JSON into a Block
		block := &Block{}
		err = json.Unmarshal(fileData, block)
		if err != nil {
			log.Printf("Error parsing block JSON %s: %v", file, err)
			continue
		}

		// Add block to blockchain
		bc.Blocks = append(bc.Blocks, block)
		log.Printf("Successfully loaded block %d: %s", blockIndex, block.Hash)

		// Add block to TXLookup
		err = bc.TXLookup.Add(block)
		if err != nil {
			log.Printf("Error adding block %d to TXLookup: %v", blockIndex, err)
		}
	}

	log.Printf("Successfully loaded %d blocks", len(bc.Blocks))

	// Update blockchain state to reflect loaded blocks
	if len(bc.Blocks) > 0 {
		lastBlockIndex := int(bc.Blocks[len(bc.Blocks)-1].Index.Int64())
		bc.CurrentBlockIndex = lastBlockIndex
		bc.NextBlockIndex = lastBlockIndex + 1
		log.Printf("Updated blockchain state: current_block_index=%d, next_block_index=%d", bc.CurrentBlockIndex, bc.NextBlockIndex)
	}

	return nil
}

// AddTransaction adds a transaction to the blockchain's transaction queue.
func (bc *Blockchain) AddTransaction(transaction Transaction) {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	// Show transaction progress
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowTransactionProgress(transaction.GetID(), "pending")
	}

	// Route transaction through sidechain router if it's a supported protocol
	protocol := transaction.GetProtocol()
	if protocol == BankProtocolID || protocol == MessageProtocolID {
		// Convert transaction to sidechain format
		txData, err := json.Marshal(transaction)
		if err != nil {
			log.Printf("Failed to marshal transaction for sidechain: %v", err)
			// Fall back to direct addition
			bc.TransactionQueue = append(bc.TransactionQueue, transaction)
			return
		}

		// Route through sidechain router
		_, err = bc.sidechainRouter.RouteTransaction(
			protocol,
			txData,
			transaction.GetSenderWallet().GetAddress(),
			transaction.GetRecipientWallet().GetAddress(),
		)
		if err != nil {
			log.Printf("Failed to route transaction through sidechain: %v", err)
			// Fall back to direct addition
			bc.TransactionQueue = append(bc.TransactionQueue, transaction)
			return
		}

		log.Printf("Transaction routed through sidechain: %s (protocol: %s)",
			transaction.GetID(), protocol)
	} else {
		// For other protocols, add directly to queue
		bc.TransactionQueue = append(bc.TransactionQueue, transaction)
	}

	// Show transaction confirmed
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowTransactionProgress(transaction.GetID(), "confirmed")
	}
}

// Mine attempts to mine a new block for the blockchain.
func (bc *Blockchain) Mine(block *Block, difficulty int) *Block {
	if bc.useHeliosMining {
		return bc.mineWithHelios(block, difficulty)
	}
	return bc.mineWithSimplePoW(block, difficulty)
}

// mineWithHelios mines a block using the Helios three-stage algorithm
func (bc *Blockchain) mineWithHelios(block *Block, difficulty int) *Block {
	LogInfof("Mining block [#%s] with Helios algorithm...", block.Index.String())

	// Convert difficulty to big.Int target
	targetDifficulty := new(big.Int).Lsh(big.NewInt(1), uint(256-difficulty))

	// Create block header for mining
	blockHeader := block.createBlockHeaderForMining()

	// Show Helios Stage 1: Proof Generation
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowHeliosProgress(1, "Proof Generation")
	}

	// Show mining progress
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowMiningProgress(int(block.Index.Int64()), difficulty, block.Hash)
	}

	// Mine using Helios algorithm
	proof, err := bc.heliosAlgorithm.Mine(blockHeader, targetDifficulty)
	if err != nil {
		// Only log if menu is not active
		bc.menuMutex.RLock()
		menuActive := bc.menuActive
		bc.menuMutex.RUnlock()

		if !menuActive {
			log.Printf("Helios mining failed: %v", err)
		}
		return block
	}

	// Show Helios Stage 2: Sidechain Routing
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowHeliosProgress(2, "Sidechain Routing")
	}

	// Show Helios Stage 3: Block Finalization
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowHeliosProgress(3, "Block Finalization")
	}

	// Update block with Helios proof
	block.updateWithHeliosProof(proof)

	// Only log if menu is not active
	bc.menuMutex.RLock()
	menuActive := bc.menuActive
	bc.menuMutex.RUnlock()

	if !menuActive {
		log.Printf("Helios mining successful: nonce=%d, hash=%s", proof.Nonce, proof.FinalHash)
	}
	return block
}

// mineWithSimplePoW mines a block using the original simple proof-of-work
func (bc *Blockchain) mineWithSimplePoW(block *Block, difficulty int) *Block {
	prefix := strings.Repeat("0", difficulty)

	// Only log if menu is not active
	bc.menuMutex.RLock()
	menuActive := bc.menuActive
	bc.menuMutex.RUnlock()

	if !menuActive {
		log.Printf("Mining a new Block [#%s] with [%d] Txs...", block.Index.String(), len(block.Transactions))
	}

	// Show mining progress
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowMiningProgress(int(block.Index.Int64()), difficulty, block.Hash)
	}

	for i := 0; i < maxNonce; i++ {
		block.Header.Nonce = uint32(i)
		block.Hash = block.CalculateHash()

		if strings.HasPrefix(block.Hash, prefix) {
			err := block.save()
			if err != nil {
				log.Printf("Error saving block: %v\n", err)
			}
			bc.Blocks = append(bc.Blocks, block)
			bc.TransactionQueue = []Transaction{}

			// Only log if menu is not active
			bc.menuMutex.RLock()
			menuActive := bc.menuActive
			bc.menuMutex.RUnlock()

			if !menuActive {
				log.Printf("Mined a new Block [#%s] with [%d] TXs & Hash [%s]\n",
					block.Index.String(),
					len(block.Transactions),
					block.Hash)
			}
			break
		}
	}

	return block
}

// VerifySignature verifies the signature of the given transaction.
func (bc *Blockchain) VerifySignature(tx Transaction) error {
	_, err := tx.Verify([]byte(tx.GetSenderWallet().PublicPEM()), tx.GetSignature())
	return err
}

// Run is a long-running function that manages the blockchain.
func (bc *Blockchain) Run(difficulty int) {
	log.Println("Blockchain.Run started")

	// Start progress indicator
	if bc.progressIndicator != nil {
		bc.progressIndicator.Start()
	}

	statusTicker := time.NewTicker(time.Second)
	blockTicker := time.NewTicker(time.Duration(bc.cfg.BlockTime) * time.Second)

	go func() {
		for range statusTicker.C {
			bc.DisplayStatus()
		}
	}()

	go func() {
		for range blockTicker.C {
			// Check if menu is active - if so, skip block creation and logging
			bc.menuMutex.RLock()
			menuActive := bc.menuActive
			bc.menuMutex.RUnlock()

			if menuActive {
				continue // Skip block creation when menu is active
			}

			now := time.Now()
			log.Printf("Block ticker fired at %s, creating new block...", now.Format("15:04:05"))
			log.Printf("Current blockchain state before creating new block: CurrentBlockIndex=%d, NextBlockIndex=%d, TotalBlocks=%d",
				bc.CurrentBlockIndex, bc.NextBlockIndex, len(bc.Blocks))
			bc.createNewBlock(difficulty)
		}
	}()
}

func (bc *Blockchain) createNewBlock(difficulty int) {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	previousHash := ""
	if len(bc.Blocks) > 0 {
		previousHash = bc.Blocks[len(bc.Blocks)-1].Hash
	}

	// Collect sidechain rollup transactions
	sidechainTxs := bc.collectSidechainRollups()

	// Combine main chain and sidechain transactions
	allTransactions := append(bc.TransactionQueue, sidechainTxs...)

	newBlock := NewBlock(allTransactions, previousHash)
	newBlock.Index = *big.NewInt(int64(bc.NextBlockIndex))

	// Show block progress
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowBlockProgress(int(newBlock.Index.Int64()), len(allTransactions))
	}

	bc.Mine(newBlock, difficulty)

	err := bc.TXLookup.Add(newBlock)
	if err != nil {
		log.Printf("Error adding block to TXLookup: %v\n", err)
	}

	err = newBlock.save()
	if err != nil {
		log.Printf("Error saving block: %v\n", err)
	}

	bc.Blocks = append(bc.Blocks, newBlock)
	bc.CurrentBlockIndex = int(newBlock.Index.Int64())
	bc.NextBlockIndex = bc.CurrentBlockIndex + 1
	bc.TransactionQueue = []Transaction{} // Clear the queue

	err = bc.Save()
	if err != nil {
		log.Printf("Error saving blockchain state: %v\n", err)
	}

	// Only log if menu is not active
	bc.menuMutex.RLock()
	menuActive := bc.menuActive
	bc.menuMutex.RUnlock()

	if !menuActive {
		log.Printf("New block created: [#%s] Hash: %s with %d main chain + %d sidechain transactions",
			newBlock.Index.String(), newBlock.Hash, len(bc.TransactionQueue), len(sidechainTxs))
		log.Printf("Blockchain state updated: CurrentBlockIndex=%d, NextBlockIndex=%d", bc.CurrentBlockIndex, bc.NextBlockIndex)
	}
}

// collectSidechainRollups collects validated sidechain transactions for inclusion in the main block
func (bc *Blockchain) collectSidechainRollups() []Transaction {
	var rollupTxs []Transaction

	// Get validated transactions from sidechain router
	if bc.sidechainRouter != nil {
		// Get bank protocol rollups
		bankTxs := bc.sidechainRouter.GetValidatedTransactions("BANK")
		for _, tx := range bankTxs {
			// Convert ProtocolTransaction back to Bank transaction
			if bankTx, err := bc.convertToBankTransaction(tx); err == nil {
				rollupTxs = append(rollupTxs, bankTx)
			}
		}

		// Get message protocol rollups
		messageTxs := bc.sidechainRouter.GetValidatedTransactions("MESSAGE")
		for _, tx := range messageTxs {
			// Convert ProtocolTransaction back to Message transaction
			if messageTx, err := bc.convertToMessageTransaction(tx); err == nil {
				rollupTxs = append(rollupTxs, messageTx)
			}
		}

		// Only log if menu is not active
		bc.menuMutex.RLock()
		menuActive := bc.menuActive
		bc.menuMutex.RUnlock()

		if !menuActive {
			log.Printf("Collected %d sidechain rollup transactions (%d bank, %d message)",
				len(rollupTxs), len(bankTxs), len(messageTxs))
		}
	}

	return rollupTxs
}

// convertToBankTransaction converts a ProtocolTransaction back to a Bank transaction
func (bc *Blockchain) convertToBankTransaction(ptx *sidechain.ProtocolTransaction) (*Bank, error) {
	// This is a simplified conversion - in a real implementation, you'd properly deserialize
	// For now, we'll create a placeholder transaction
	fromWallet := &Wallet{Address: ptx.Sender}
	toWallet := &Wallet{Address: ptx.Recipient}

	bankTx := &Bank{
		Tx: Tx{
			From: fromWallet,
			To:   toWallet,
			Fee:  0.05,
		},
		Amount: 0.0, // Would be extracted from ptx.Data
	}
	return bankTx, nil
}

// convertToMessageTransaction converts a ProtocolTransaction back to a Message transaction
func (bc *Blockchain) convertToMessageTransaction(ptx *sidechain.ProtocolTransaction) (*Message, error) {
	// This is a simplified conversion - in a real implementation, you'd properly deserialize
	// For now, we'll create a placeholder transaction
	fromWallet := &Wallet{Address: ptx.Sender}
	toWallet := &Wallet{Address: ptx.Recipient}

	messageTx := &Message{
		Tx: Tx{
			From: fromWallet,
			To:   toWallet,
			Fee:  0.01,
		},
		Message: "Rollup message", // Would be extracted from ptx.Data
	}
	return messageTx, nil
}

// generateHash generates a SHA-512 hash for the given block.
func (bc *Blockchain) generateHash(block *Block) string {
	record := block.Index.Text(10) + block.Header.Timestamp.String() + strconv.FormatUint(uint64(block.Header.Nonce), 10) + block.Header.PreviousHash
	h := sha512.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// GetLatestBlock returns the latest block in the blockchain.
func (bc *Blockchain) GetLatestBlock() *Block {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	if len(bc.Blocks) == 0 {
		return nil
	}
	return bc.Blocks[len(bc.Blocks)-1]
}

// GetBlockByHash returns a block with the given hash.
func (bc *Blockchain) GetBlockByHash(hash string) *Block {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	for _, block := range bc.Blocks {
		if block.Hash == hash {
			return block
		}
	}
	return nil
}

// GetBlockByIndex returns a block at the given index.
func (bc *Blockchain) GetBlockByIndex(index int64) *Block {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	if index < 0 || int(index) >= len(bc.Blocks) {
		return nil
	}
	return bc.Blocks[index]
}

// GetTransactionByID returns a transaction with the given ID.
func (bc *Blockchain) GetTransactionByID(id string) Transaction {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	// First, check the transaction queue
	for _, tx := range bc.TransactionQueue {
		if tx.GetID() == id {
			return tx
		}
	}

	// Then, check all blocks
	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.GetID() == id {
				return tx
			}
		}
	}

	return nil
}

// GetBalance returns the balance of a given wallet address.
func (bc *Blockchain) GetBalance(address string) float64 {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	balance := 0.0
	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.GetSenderWallet().GetAddress() == address {
				balance -= tx.GetFee()
				if bankTx, ok := tx.(*Bank); ok {
					balance -= bankTx.Amount
				}
			}
			if tx.GetProtocol() == BankProtocolID {
				if bankTx, ok := tx.(*Bank); ok {
					if bankTx.To.GetAddress() == address {
						balance += bankTx.Amount
					}
				}
			}
		}
	}
	return balance
}

// CalculateTotalSupply calculates the total supply of tokens in the blockchain.
func (bc *Blockchain) CalculateTotalSupply() float64 {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	totalSupply := 0.0
	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.GetProtocol() == CoinbaseProtocolID {
				if coinbaseTx, ok := tx.(*Coinbase); ok {
					totalSupply += float64(coinbaseTx.TokenCount)
				}
			}
		}
	}
	return totalSupply
}

// ValidateChain validates the entire blockchain.
func (bc *Blockchain) ValidateChain() error {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	for i := 1; i < len(bc.Blocks); i++ {
		currentBlock := bc.Blocks[i]
		previousBlock := bc.Blocks[i-1]

		if currentBlock.Header.PreviousHash != previousBlock.Hash {
			return fmt.Errorf("invalid previous hash at block %d", i)
		}

		if currentBlock.Hash != currentBlock.CalculateHash() {
			return fmt.Errorf("invalid hash at block %d", i)
		}

		if err := currentBlock.Validate(previousBlock); err != nil {
			return fmt.Errorf("invalid block at index %d: %v", i, err)
		}

		for _, tx := range currentBlock.Transactions {
			if err := tx.Validate(); err != nil {
				return fmt.Errorf("invalid transaction %s in block %d: %v", tx.GetID(), i, err)
			}
		}
	}

	return nil
}

// GetTransactionHistory returns the transaction history for a given wallet address.
func (bc *Blockchain) GetTransactionHistory(address string) []Transaction {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	var history []Transaction

	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.GetSenderWallet().GetAddress() == address || (tx.GetProtocol() == BankProtocolID && tx.(*Bank).To.GetAddress() == address) {
				history = append(history, tx)
			}
		}
	}

	return history
}

// GetPendingTransactions returns all pending transactions in the queue.
func (bc *Blockchain) GetPendingTransactions() []Transaction {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	return bc.TransactionQueue
}

// RemoveTransaction removes a transaction from the pending queue.
func (bc *Blockchain) RemoveTransaction(id string) bool {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	for i, tx := range bc.TransactionQueue {
		if tx.GetID() == id {
			bc.TransactionQueue = append(bc.TransactionQueue[:i], bc.TransactionQueue[i+1:]...)
			return true
		}
	}

	return false
}

// UpdateConfig updates the blockchain configuration.
func (bc *Blockchain) UpdateConfig(newConfig *Config) error {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	// Validate the new configuration
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %v", err)
	}

	// Update the configuration
	bc.cfg = newConfig

	// Save the updated configuration
	return bc.Save()
}

// GetBlockchainInfo returns general information about the blockchain.
func (bc *Blockchain) GetBlockchainInfo() BlockchainInfo {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	return BlockchainInfo{
		Version:    BlockchainVersion,
		Name:       bc.cfg.BlockchainName,
		Symbol:     bc.cfg.BlockchainSymbol,
		BlockTime:  bc.cfg.BlockTime,
		Difficulty: bc.cfg.Difficulty,
		Fee:        bc.cfg.TransactionFee,
	}
}

// GetMempoolSize returns the number of transactions in the mempool (transaction queue).
func (bc *Blockchain) GetMempoolSize() int {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	return len(bc.TransactionQueue)
}

// GetBlockCount returns the number of blocks in the blockchain.
func (bc *Blockchain) GetBlockCount() int {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	return len(bc.Blocks)
}

// Sidechain router callback methods
func (bc *Blockchain) onTransactionValidated(tx *sidechain.ProtocolTransaction) error {
	log.Printf("Transaction validated: %s (protocol: %s)", tx.ID, tx.Protocol)
	return nil
}

func (bc *Blockchain) onTransactionFailed(tx *sidechain.ProtocolTransaction, errorMsg string) error {
	log.Printf("Transaction failed: %s (protocol: %s) - %s", tx.ID, tx.Protocol, errorMsg)
	return nil
}

func (bc *Blockchain) onRollupCreated(rollup *sidechain.RollupBlock) error {
	log.Printf("Rollup block created: %s (protocol: %s) with %d transactions",
		rollup.ID, rollup.Protocol, len(rollup.Transactions))
	return nil
}

// GetProgressIndicator returns the progress indicator instance
func (bc *Blockchain) GetProgressIndicator() *progress.ProgressIndicator {
	return bc.progressIndicator
}

// SetMenuActive sets the menu active state
func (bc *Blockchain) SetMenuActive(active bool) {
	bc.menuMutex.Lock()
	defer bc.menuMutex.Unlock()
	bc.menuActive = active
}

// IsMenuActive returns true if the menu is currently active
func (bc *Blockchain) IsMenuActive() bool {
	bc.menuMutex.RLock()
	defer bc.menuMutex.RUnlock()
	return bc.menuActive
}

// Cleanup performs any necessary cleanup
func (bc *Blockchain) Cleanup() {
	// No cleanup needed for menu state
}
