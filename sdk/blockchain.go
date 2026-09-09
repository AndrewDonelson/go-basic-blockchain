// Package sdk is a software development kit for building blockchain applications.
// File sdk/blockchain.go - The main Blockchain file

package sdk

import (
	"context"
	"encoding/json"
	"errors"
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

	// Network announcement hooks. These let the P2P layer relay blocks and
	// transactions without the chain package depending on it.
	announceBlock func(*Block)
	announceTx    func(Transaction)
	announceMu    sync.RWMutex
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
	if n := GetNode(); n != nil && n.ProgressIndicator != nil {
		bc.progressIndicator = n.ProgressIndicator
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

	currentAction := ""
	peerCount := 0
	uptime := time.Duration(0)
	if bc.progressIndicator != nil {
		currentAction = bc.progressIndicator.CurrentStatus().Action
	}
	if n := GetNode(); n != nil && n.P2P != nil {
		peerCount = n.P2P.PeerCount()
		if !n.StartedAt.IsZero() {
			uptime = time.Since(n.StartedAt)
		}
	}

	// (A comparison of len(bc.Blocks) against a copy taken two lines earlier used
	// to live here. It could never be true.)

	// Update progress indicator only if not paused
	if bc.progressIndicator != nil {
		// Check if progress indicator is paused (menu is active)
		if !bc.progressIndicator.IsPaused() {
			status := progress.BlockchainStatus{
				Action:      currentAction,
				IsMining:    true, // Assume mining is active
				BlockCount:  len(bc.Blocks),
				TxQueueSize: len(bc.TransactionQueue),
				Difficulty:  bc.cfg.Difficulty,
				HashRate:    0, // TODO: Calculate actual hash rate
				LastBlock:   "",
				Peers:       peerCount,
				IsSynced:    true,
				Uptime:      uptime,
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
	bc.mux.Lock()
	defer bc.mux.Unlock()
	return bc.saveLocked()
}

func (bc *Blockchain) saveLocked() error {
	data := &BlockchainPersistData{
		TXLookup:       bc.TXLookup.index.Get(),
		CurrBlockIndex: &bc.CurrentBlockIndex,
		NextBlockIndex: &bc.NextBlockIndex,
	}

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

	if err = minerWallet.Close(minerWalletPW); err != nil {
		return fmt.Errorf("failed to close miner wallet: %v", err)
	}

	if err = minerWallet.Open(minerWalletPW); err != nil {
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

		mined, err := bc.Mine(genesisBlock, 1)
		if err != nil {
			LogInfof("Failed to mine genesis block: %v", err)
			return
		}
		genesisBlock = mined

		err = genesisBlock.save()
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

// LoadExistingBlocks loads any existing blocks from disk and appends them to the
// blockchain, in block-index order, verifying the hash chain as it goes.
//
// Two problems with the previous version: it ordered files with sort.Strings, so
// "10.json" sorted before "2.json" and blocks were appended in lexical order --
// bc.Blocks[i] was not block i, and the previousHash chain was scrambled. And it
// logged and skipped every decode failure, which (given Block could not be
// decoded at all) silently discarded the entire history on every restart.
func (bc *Blockchain) LoadExistingBlocks() error {
	blocksPath := filepath.Join(bc.cfg.DataPath, "blocks")

	// Look for both .json and .jso files (in case of truncated names)
	jsonFiles, _ := filepath.Glob(filepath.Join(blocksPath, "*.json"))
	jsoFiles, _ := filepath.Glob(filepath.Join(blocksPath, "*.jso"))
	files := append(jsonFiles, jsoFiles...)
	if len(files) == 0 {
		LogVerbosef("No existing blocks found in %s", blocksPath)
		return nil
	}

	LogInfof("Loading %d existing blocks...", len(files))

	type blockFile struct {
		index int
		path  string
	}

	ordered := make([]blockFile, 0, len(files))
	for _, file := range files {
		filename := filepath.Base(file)
		blockIndexStr := strings.TrimSuffix(strings.TrimSuffix(filename, ".json"), ".jso")
		blockIndex, err := strconv.Atoi(blockIndexStr)
		if err != nil {
			LogInfof("Skipping file with non-numeric block index: %s", filename)
			continue
		}
		ordered = append(ordered, blockFile{index: blockIndex, path: file})
	}

	// Sort numerically, not lexically.
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].index < ordered[j].index })

	loaded := make([]*Block, 0, len(ordered))
	for _, bf := range ordered {
		fileData, err := os.ReadFile(bf.path)
		if err != nil {
			return fmt.Errorf("error reading block file %s: %w", bf.path, err)
		}

		block := &Block{}
		if err := json.Unmarshal(fileData, block); err != nil {
			// A corrupt block is fatal, not skippable: continuing would silently
			// serve a chain with a hole in it.
			return fmt.Errorf("error parsing block %d (%s): %w", bf.index, bf.path, err)
		}

		if got := int(block.Index.Int64()); got != bf.index {
			return fmt.Errorf("block file %s contains block %d", bf.path, got)
		}

		// Verify the chain links as we load, so a gap or a tampered block is
		// detected at startup rather than at some arbitrary later point.
		if len(loaded) > 0 {
			previous := loaded[len(loaded)-1]
			if block.Header.PreviousHash != previous.Hash {
				return fmt.Errorf("block %d does not link to block %d (previous hash mismatch)",
					bf.index, int(previous.Index.Int64()))
			}
		}

		loaded = append(loaded, block)
		if err := bc.TXLookup.Add(block); err != nil {
			LogInfof("Error adding block %d to TXLookup: %v", bf.index, err)
		}
	}

	bc.mux.Lock()
	defer bc.mux.Unlock()

	bc.Blocks = loaded
	LogInfof("Successfully loaded %d blocks", len(bc.Blocks))

	if len(bc.Blocks) > 0 {
		lastBlockIndex := int(bc.Blocks[len(bc.Blocks)-1].Index.Int64())
		bc.CurrentBlockIndex = lastBlockIndex
		bc.NextBlockIndex = lastBlockIndex + 1
		LogVerbosef("Updated blockchain state: current_block_index=%d, next_block_index=%d",
			bc.CurrentBlockIndex, bc.NextBlockIndex)
	}

	return nil
}

// AddTransaction adds a transaction to the blockchain's transaction queue.
//
// The mempool is the single source of truth. Previously a BANK or MESSAGE
// transaction was handed to the sidechain router and *not* queued, and the router
// output was later reconstituted by convertToBankTransaction /
// convertToMessageTransaction -- placeholder builders that hardcoded
// `Amount: 0.0` and `Message: "Rollup message"` around wallet stubs with no keys.
// Every routed transfer therefore reached its block with its value zeroed and its
// signature unverifiable. The router now observes transactions for its rollup
// accounting while the mempool keeps the real thing.
func (bc *Blockchain) AddTransaction(transaction Transaction) {
	if !bc.AddTransactionLocal(transaction) {
		return
	}

	// Locally originated, so relay it to peers. Transactions that arrive *from* a
	// peer use AddTransactionLocal directly: re-announcing one would bounce it
	// back and forth between nodes indefinitely.
	bc.announceTransaction(transaction)
}

// AddTransactionLocal queues a transaction without relaying it to peers.
//
// It reports whether the transaction was newly queued, so a relayed duplicate is
// not counted or re-announced.
func (bc *Blockchain) AddTransactionLocal(transaction Transaction) bool {
	if transaction == nil {
		return false
	}

	id := transaction.GetID()

	bc.mux.Lock()
	for _, queued := range bc.TransactionQueue {
		if queued.GetID() == id {
			bc.mux.Unlock()
			return false
		}
	}
	bc.TransactionQueue = append(bc.TransactionQueue, transaction)
	bc.mux.Unlock()

	if bc.progressIndicator != nil {
		// "pending" is accurate here. Reporting "confirmed" immediately after
		// queueing, as this used to, told the caller a transaction was final while
		// it was still sitting in the mempool.
		bc.progressIndicator.ShowTransactionProgress(id, "pending")
	}

	bc.routeToSidechain(transaction)
	return true
}

// routeToSidechain mirrors a transaction into the protocol router for rollup
// accounting. A routing failure never loses the transaction: it is already in the
// mempool.
func (bc *Blockchain) routeToSidechain(transaction Transaction) {
	protocol := transaction.GetProtocol()
	if protocol != BankProtocolID && protocol != MessageProtocolID {
		return
	}
	if bc.sidechainRouter == nil {
		return
	}

	txData, err := json.Marshal(transaction)
	if err != nil {
		LogVerbosef("Failed to marshal transaction for sidechain: %v", err)
		return
	}

	sender, recipient := "", ""
	if w := transaction.GetSenderWallet(); w != nil {
		sender = w.GetAddress()
	}
	if w := transaction.GetRecipientWallet(); w != nil {
		recipient = w.GetAddress()
	}

	if _, err := bc.sidechainRouter.RouteTransaction(protocol, txData, sender, recipient); err != nil {
		LogVerbosef("Failed to route transaction through sidechain: %v", err)
		return
	}

	LogVerbosef("Transaction mirrored to sidechain: %s (protocol: %s)", transaction.GetID(), protocol)
}

// difficultyTarget converts an integer difficulty into a 256-bit target.
//
// A hash is valid when it is <= target, so a larger difficulty means a smaller
// target. The bounds matter: uint(256-difficulty) underflows for difficulty > 256
// and produces an astronomically large shift.
func difficultyTarget(difficulty int) *big.Int {
	if difficulty < 1 {
		difficulty = 1
	}
	if difficulty > 255 {
		difficulty = 255
	}
	return new(big.Int).Lsh(big.NewInt(1), uint(256-difficulty))
}

// Mine attempts to mine a block, returning an error if no valid proof was found.
//
// Mine no longer mutates chain state. The simple-PoW path used to append to
// bc.Blocks and clear bc.TransactionQueue itself, without holding bc.mux and in
// addition to the append its caller already performed -- so every block was added
// twice and the mempool was cleared from an unsynchronised goroutine.
func (bc *Blockchain) Mine(block *Block, difficulty int) (*Block, error) {
	if bc.useHeliosMining {
		return bc.mineWithHelios(block, difficulty)
	}
	return bc.mineWithSimplePoW(block, difficulty)
}

// mineWithHelios mines a block using the Helios three-stage algorithm
func (bc *Blockchain) mineWithHelios(block *Block, difficulty int) (*Block, error) {
	LogVerbosef("Mining block [#%s] with Helios algorithm...", block.Index.String())

	targetDifficulty := difficultyTarget(difficulty)

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
		// A mining failure (including the timeout) must abandon the block. The old
		// code logged the error and returned the *unmined* block, which the caller
		// then appended to the chain and persisted.
		return nil, fmt.Errorf("helios mining failed: %w", err)
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

	// Verify our own work before publishing it. bc.heliosValidator was constructed
	// and then never called anywhere, so nothing ever checked a proof.
	if err := bc.verifyHeliosProof(block, difficulty); err != nil {
		return nil, fmt.Errorf("self-verification of freshly mined block failed: %w", err)
	}

	LogVerbosef("Helios mining successful: nonce=%d, hash=%s", proof.Nonce, proof.FinalHash)
	return block, nil
}

// verifyHeliosProof checks a block's stored Helios proof against its header and
// the target difficulty.
func (bc *Blockchain) verifyHeliosProof(block *Block, difficulty int) error {
	if block.HeliosProof == nil {
		return errors.New("block carries no Helios proof")
	}
	if bc.heliosAlgorithm == nil {
		return errors.New("helios algorithm is not initialized")
	}

	target := difficultyTarget(difficulty)

	// Recompute the proof hash from the block header: this is what makes the
	// proof binding rather than self-asserted.
	if err := bc.heliosAlgorithm.ValidateProof(block.HeliosProof, block.createBlockHeaderForMining(), target); err != nil {
		return err
	}

	if bc.heliosValidator != nil {
		if err := bc.heliosValidator.ValidateFullProof(block.HeliosProof, target); err != nil {
			return err
		}
	}
	return nil
}

// mineWithSimplePoW mines a block using the original simple proof-of-work.
//
// It searches the nonce space and returns an error when it is exhausted. It does
// not touch chain state: appending the block and clearing the mempool is the
// caller's job, done once, under the lock.
func (bc *Blockchain) mineWithSimplePoW(block *Block, difficulty int) (*Block, error) {
	prefix := strings.Repeat("0", difficulty)

	LogVerbosef("Mining a new Block [#%s] with [%d] Txs...", block.Index.String(), len(block.Transactions))

	// Show mining progress
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowMiningProgress(int(block.Index.Int64()), difficulty, block.Hash)
	}

	for i := 0; i < maxMiningNonce; i++ {
		block.Header.Nonce = uint32(i)
		block.Hash = block.CalculateHash()

		if strings.HasPrefix(block.Hash, prefix) {
			LogVerbosef("Mined a new Block [#%s] with [%d] TXs & Hash [%s]",
				block.Index.String(), len(block.Transactions), block.Hash)
			return block, nil
		}
	}

	return nil, fmt.Errorf("exhausted the nonce space without finding a proof at difficulty %d", difficulty)
}

// VerifySignature verifies the signature of the given transaction.
func (bc *Blockchain) VerifySignature(tx Transaction) error {
	_, err := tx.Verify([]byte(tx.GetSenderWallet().PublicPEM()), tx.GetSignature())
	return err
}

// Run is a long-running function that manages the blockchain.
//
// It is a thin wrapper over RunContext for callers that do not manage a context.
func (bc *Blockchain) Run(difficulty int) {
	bc.RunContext(context.Background(), difficulty)
}

// RunContext starts the mining and status loops, stopping when ctx is cancelled.
//
// Both tickers used to run in goroutines with no exit path at all: they were
// never stopped, so every Blockchain leaked two goroutines and two tickers for
// the life of the process, and there was no way to shut mining down cleanly.
func (bc *Blockchain) RunContext(ctx context.Context, difficulty int) {
	LogInfof("Blockchain.Run started")

	// Start progress indicator
	if bc.progressIndicator != nil {
		bc.progressIndicator.Start()
	}

	blockTime := bc.cfg.BlockTime
	if blockTime <= 0 {
		blockTime = blockTimeInSec
	}

	go func() {
		statusTicker := time.NewTicker(time.Second)
		defer statusTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-statusTicker.C:
				bc.DisplayStatus()
			}
		}
	}()

	go func() {
		blockTicker := time.NewTicker(time.Duration(blockTime) * time.Second)
		defer blockTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-blockTicker.C:
				if bc.IsMenuActive() {
					continue // Skip block creation when the menu is open
				}

				LogVerbosef("Block ticker fired, creating new block (current=%d next=%d total=%d)",
					bc.CurrentBlockIndex, bc.NextBlockIndex, bc.GetBlockCount())
				bc.createNewBlock(difficulty)
			}
		}
	}()
}

func (bc *Blockchain) createNewBlock(difficulty int) {
	bc.mux.Lock()
	previousHash := ""
	if len(bc.Blocks) > 0 {
		previousHash = bc.Blocks[len(bc.Blocks)-1].Hash
	}

	queuedTransactions := append([]Transaction(nil), bc.TransactionQueue...)
	bc.TransactionQueue = []Transaction{}
	nextBlockIndex := bc.NextBlockIndex
	bc.mux.Unlock()

	// The mempool already holds the real transactions, including the ones mirrored
	// to the sidechain, so there is nothing to merge in from the router.
	allTransactions := queuedTransactions

	newBlock := NewBlock(allTransactions, previousHash)
	newBlock.Index = *big.NewInt(int64(nextBlockIndex))

	// Show block progress
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowBlockProgress(int(newBlock.Index.Int64()), len(allTransactions))
	}

	minedBlock, err := bc.Mine(newBlock, difficulty)
	if err != nil {
		// Mining failed, so there is no block. Return the transactions to the
		// mempool rather than losing them: previously the unmined block was
		// appended and persisted regardless.
		LogInfof("Failed to mine block #%d: %v", nextBlockIndex, err)
		bc.requeueTransactions(queuedTransactions)
		return
	}
	newBlock = minedBlock

	if err := bc.TXLookup.Add(newBlock); err != nil {
		LogInfof("Error adding block to TXLookup: %v", err)
	}

	if err := newBlock.save(); err != nil {
		LogInfof("Error saving block: %v", err)
	}

	bc.commitMinedBlock(newBlock, len(queuedTransactions))

	// Relay the block we just mined. This happens after the lock is released, so
	// network I/O never blocks the chain.
	//
	// Only *mined* blocks are announced, never accepted ones. Re-announcing an
	// accepted block would echo it straight back to the peer that sent it; peers
	// that are further behind close the gap through the periodic Syncer, which
	// fetches the intervening blocks too. One-hop announcement plus periodic sync
	// is simpler than gossip with deduplication, and has no loop to get wrong.
	bc.announce(newBlock)
}

// commitMinedBlock appends a freshly mined block and persists chain state.
func (bc *Blockchain) commitMinedBlock(newBlock *Block, txCount int) {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	bc.Blocks = append(bc.Blocks, newBlock)
	bc.CurrentBlockIndex = int(newBlock.Index.Int64())
	bc.NextBlockIndex = bc.CurrentBlockIndex + 1

	if err := bc.saveLocked(); err != nil {
		LogInfof("Error saving blockchain state: %v", err)
	}

	LogVerbosef("New block created: [#%s] Hash: %s with %d transactions",
		newBlock.Index.String(), newBlock.Hash, txCount)
	LogVerbosef("Blockchain state updated: CurrentBlockIndex=%d, NextBlockIndex=%d",
		bc.CurrentBlockIndex, bc.NextBlockIndex)
}

// requeueTransactions puts transactions back at the front of the mempool after a
// failed block, preserving their original ordering.
func (bc *Blockchain) requeueTransactions(txs []Transaction) {
	if len(txs) == 0 {
		return
	}

	bc.mux.Lock()
	defer bc.mux.Unlock()
	bc.TransactionQueue = append(txs, bc.TransactionQueue...)
}

// HasTransactionID reports whether a transaction ID is already known, in the
// mempool or in any block.
func (bc *Blockchain) HasTransactionID(id string) bool {
	if id == "" {
		return false
	}

	bc.mux.Lock()
	defer bc.mux.Unlock()

	for _, tx := range bc.TransactionQueue {
		if tx.GetID() == id {
			return true
		}
	}
	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.GetID() == id {
				return true
			}
		}
	}
	return false
}

// AcceptBlock validates a block received from a peer and appends it if it extends
// the current head.
//
// Fork choice and reorganisation are deliberately out of scope here: a block that
// does not extend the current head is refused rather than silently accepted. The
// /consensus/block endpoint previously answered {"accepted":true} for any payload
// at all, without inspecting it.
func (bc *Blockchain) AcceptBlock(block *Block) error {
	if block == nil {
		return errors.New("block is nil")
	}

	bc.mux.Lock()
	head := (*Block)(nil)
	if len(bc.Blocks) > 0 {
		head = bc.Blocks[len(bc.Blocks)-1]
	}
	expectedIndex := int64(bc.NextBlockIndex)
	bc.mux.Unlock()

	if head == nil {
		return errors.New("cannot accept a block before the genesis block exists")
	}

	if block.Index.Int64() != expectedIndex {
		return fmt.Errorf("block index %s does not extend the current head (expected %d)",
			block.Index.String(), expectedIndex)
	}

	if err := block.Validate(head); err != nil {
		return fmt.Errorf("block validation failed: %w", err)
	}

	if bc.useHeliosMining {
		if err := bc.verifyHeliosProof(block, bc.cfg.Difficulty); err != nil {
			return fmt.Errorf("proof-of-work validation failed: %w", err)
		}
	}

	if err := bc.TXLookup.Add(block); err != nil {
		LogInfof("Error adding accepted block to TXLookup: %v", err)
	}
	if err := block.save(); err != nil {
		return fmt.Errorf("failed to persist accepted block: %w", err)
	}

	bc.mux.Lock()
	defer bc.mux.Unlock()
	bc.Blocks = append(bc.Blocks, block)
	bc.CurrentBlockIndex = int(block.Index.Int64())
	bc.NextBlockIndex = bc.CurrentBlockIndex + 1
	return bc.saveLocked()
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

	// Match on the block's own Index rather than its slice position: the two
	// diverge as soon as blocks are loaded from disk out of order, or a block is
	// ever pruned.
	for _, block := range bc.Blocks {
		if block.Index.Int64() == index {
			return block
		}
	}
	return nil
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
			sender := ""
			if w := tx.GetSenderWallet(); w != nil {
				sender = w.GetAddress()
			}
			recipient := ""
			if w := tx.GetRecipientWallet(); w != nil {
				recipient = w.GetAddress()
			}

			switch concrete := tx.(type) {
			case *Coinbase:
				// Coinbase output was never credited to anyone, so the minted
				// supply existed in CalculateTotalSupply but in nobody's balance.
				if recipient == address {
					balance += float64(concrete.TokenCount)
				}
			case *Bank:
				if sender == address {
					balance -= concrete.Amount + concrete.GetFee()
				}
				if recipient == address {
					balance += concrete.Amount
				}
			default:
				if sender == address {
					balance -= tx.GetFee()
				}
			}

			// Fees are paid out rather than destroyed.
			balance += bc.feeCreditFor(tx, address)
		}
	}
	return balance
}

// feeCreditFor returns the share of a transaction's fee credited to address.
//
// Fees used to be subtracted from the sender and credited to nobody, so every
// transaction quietly destroyed value. They are now split between the miner and
// the developer per MinerRewardPCT / DevRewardPCT, which the config has always
// described but nothing implemented.
func (bc *Blockchain) feeCreditFor(tx Transaction, address string) float64 {
	fee := tx.GetFee()
	if fee <= 0 || bc.cfg == nil {
		return 0
	}

	credit := 0.0
	if bc.cfg.MinerAddress == address {
		credit += fee * bc.cfg.MinerRewardPCT / 100.0
	}
	if bc.cfg.DevAddress == address {
		credit += fee * bc.cfg.DevRewardPCT / 100.0
	}
	return credit
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

		// Verify the proof of work. ValidateChain previously checked hash linkage
		// and transaction validity but never that any work had been done.
		if bc.useHeliosMining {
			if err := bc.verifyHeliosProof(currentBlock, bc.cfg.Difficulty); err != nil {
				return fmt.Errorf("invalid proof of work at block %d: %v", i, err)
			}
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

	// Return a copy. Handing out the live slice let callers read (and append to)
	// mutex-guarded state after the lock was released.
	out := make([]Transaction, len(bc.TransactionQueue))
	copy(out, bc.TransactionQueue)
	return out
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

	// Save the updated configuration. This must be saveLocked: bc.Save() takes
	// bc.mux, which we already hold, and sync.Mutex is not reentrant -- calling
	// Save() here deadlocked the caller permanently.
	return bc.saveLocked()
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

// Height returns the index of the head block, or -1 for an empty chain.
//
// Height is the block's own Index, not its slice position, so it stays correct
// if the two ever diverge.
func (bc *Blockchain) Height() int {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	return bc.heightLocked()
}

func (bc *Blockchain) heightLocked() int {
	if len(bc.Blocks) == 0 {
		return -1
	}
	return int(bc.Blocks[len(bc.Blocks)-1].Index.Int64())
}

// HeadHash returns the hash of the head block, or "" for an empty chain.
func (bc *Blockchain) HeadHash() string {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	if len(bc.Blocks) == 0 {
		return ""
	}
	return bc.Blocks[len(bc.Blocks)-1].Hash
}

// GenesisHash returns the hash of block 0, or "" for an empty chain.
//
// Peers compare this before syncing: two nodes with different genesis blocks are
// on different networks, and pulling blocks across that boundary would be
// meaningless.
func (bc *Blockchain) GenesisHash() string {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	if len(bc.Blocks) == 0 {
		return ""
	}
	return bc.Blocks[0].Hash
}

// ChainStatus summarises this node's chain for a peer.
func (bc *Blockchain) ChainStatus() ChainStatus {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	status := ChainStatus{Height: bc.heightLocked()}
	if len(bc.Blocks) > 0 {
		status.HeadHash = bc.Blocks[len(bc.Blocks)-1].Hash
		status.GenesisHash = bc.Blocks[0].Hash
	}
	if n := GetNode(); n != nil {
		status.NodeID = n.ID
	}
	return status
}

// GetBlocksFrom returns up to count blocks starting at block index startIndex.
//
// Selection is by the block's own Index rather than its slice position, so a
// sync request for "everything from height N" means what it says.
func (bc *Blockchain) GetBlocksFrom(startIndex, count int) []*Block {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	if count <= 0 {
		return []*Block{}
	}

	out := make([]*Block, 0, count)
	for _, block := range bc.Blocks {
		if int(block.Index.Int64()) < startIndex {
			continue
		}
		out = append(out, block)
		if len(out) == count {
			break
		}
	}
	return out
}

// SetBlockAnnouncer registers a callback invoked after a block is added locally,
// so the networking layer can relay it without the chain importing the P2P code.
func (bc *Blockchain) SetBlockAnnouncer(fn func(*Block)) {
	bc.announceMu.Lock()
	defer bc.announceMu.Unlock()
	bc.announceBlock = fn
}

// SetTransactionAnnouncer registers a callback invoked when a transaction enters
// the local mempool.
func (bc *Blockchain) SetTransactionAnnouncer(fn func(Transaction)) {
	bc.announceMu.Lock()
	defer bc.announceMu.Unlock()
	bc.announceTx = fn
}

// announce relays a newly added block to the network, if an announcer is set.
func (bc *Blockchain) announce(block *Block) {
	bc.announceMu.RLock()
	fn := bc.announceBlock
	bc.announceMu.RUnlock()

	if fn != nil {
		fn(block)
	}
}

// announceTransaction relays a newly queued transaction, if an announcer is set.
func (bc *Blockchain) announceTransaction(tx Transaction) {
	bc.announceMu.RLock()
	fn := bc.announceTx
	bc.announceMu.RUnlock()

	if fn != nil {
		fn(tx)
	}
}

// GetBlockRange returns a copy of the blocks in [start, end).
//
// Callers (notably the API handlers) used to index bc.Blocks directly while the
// mining goroutine appended to it. Returning a copy taken under the lock removes
// that race without handing out a reference into guarded state.
func (bc *Blockchain) GetBlockRange(start, end int) []*Block {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	if start < 0 {
		start = 0
	}
	if end > len(bc.Blocks) {
		end = len(bc.Blocks)
	}
	if start >= end {
		return []*Block{}
	}

	out := make([]*Block, end-start)
	copy(out, bc.Blocks[start:end])
	return out
}

// GetAllTransactions returns every transaction in the chain plus the mempool.
func (bc *Blockchain) GetAllTransactions() []Transaction {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	all := make([]Transaction, 0, len(bc.TransactionQueue))
	for _, block := range bc.Blocks {
		all = append(all, block.Transactions...)
	}
	return append(all, bc.TransactionQueue...)
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

// Cleanup releases the blockchain's resources and flushes state to disk.
//
// This used to be an empty function with the comment "No cleanup needed", so
// SIGINT left the sidechain rollup goroutine running and the blockchain state
// unflushed -- the last block's index was lost on every shutdown.
func (bc *Blockchain) Cleanup() {
	if bc.sidechainRouter != nil {
		bc.sidechainRouter.Stop()
	}
	if bc.progressIndicator != nil {
		bc.progressIndicator.Stop()
	}
	if err := bc.Save(); err != nil {
		LogInfof("Error saving blockchain state during shutdown: %v", err)
	}
}
