// Package sdk is a software development kit for building blockchain applications.
// File sdk/blockchain.go - The main Blockchain file

package sdk

import (
	"fmt"
	"log"
	"sync"

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

	// metrics records what the node is doing. Nothing used to be measurable: the
	// status struct reported a hardcoded `HashRate: 0` and there were no counters
	// at all.
	metrics     *Metrics
	metricsOnce sync.Once

	// Progress indicator
	progressIndicator *progress.ProgressIndicator

	// Menu state
	menuActive bool
	menuMutex  sync.RWMutex

	// utxos is the authoritative record of who owns what, derived from the blocks.
	// It replaces both the O(chain) rescan in GetBalance and the per-wallet
	// balance that nothing kept in step with the chain.
	utxos *UTXOSet

	// blockIndex holds every block we have seen, on the main chain or not, keyed
	// by hash. Fork choice walks it to assemble a candidate branch, so a block
	// arriving out of order is retained rather than discarded.
	blockIndex map[string]*Block
	// indexedBlocks is how many leading entries of Blocks are already in
	// blockIndex, and indexedTipHash is the hash that was at that position.
	// The hash is what makes the count trustworthy: a reorganisation can replace
	// blocks below the old length, so a count alone would skip them.
	indexedBlocks  int
	indexedTipHash string
	// mainChainHeight maps a hash to its position on the main chain. Unlike
	// blockIndex it holds main-chain blocks only, so it is cleared whenever the
	// chain is rebuilt rather than extended.
	mainChainHeight map[string]int

	// txIDIndex answers "have I seen this transaction?" without scanning every
	// block. txIndexedBlocks and txIndexedTipHash track how much of the chain it
	// covers, on the same basis as blockIndex above.
	txIDIndex        map[string]struct{}
	txIndexedBlocks  int
	txIndexedTipHash string

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
		utxos:             NewUTXOSet(),
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

	// A blockchain does not create a node.
	//
	// This used to call NewNode when the global node was nil, while NewNode
	// itself builds a blockchain -- a cycle that only terminated because NewNode
	// assigned the global halfway through its own construction. Separating
	// construction from registration exposed it as unbounded recursion.
	//
	// Nothing here needs a node: every other GetNode() call site already treats
	// nil as "not running inside a node" and degrades accordingly.

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

	// Rebuild the UTXO set from the blocks we have. The set is derived state, so
	// it is never loaded from disk -- replaying the chain is what guarantees it
	// matches the blocks rather than some stale snapshot.
	if err := bc.RebuildUTXOSet(); err != nil {
		log.Printf("Error rebuilding the UTXO set: %v", err)
		return nil
	}

	log.Printf("Blockchain initialized with %d blocks", len(bc.Blocks))
	return bc
}

// GetConfig returns the configuration used to create the Blockchain instance.
func (bc *Blockchain) GetConfig() *Config {
	return bc.cfg
}

// Metrics returns the node's metrics recorder, creating it on first use.
//
// Lazily initialised because Blockchain is constructed directly in several
// places, including tests, and a nil recorder would otherwise have to be checked
// at every call site. Every Metrics method also tolerates a nil receiver, so this
// is belt and braces.
func (bc *Blockchain) Metrics() *Metrics {
	bc.metricsOnce.Do(func() {
		if bc.metrics == nil {
			bc.metrics = NewMetrics()
		}
	})
	return bc.metrics
}

// UpdateConfig updates the blockchain configuration.
func (bc *Blockchain) UpdateConfig(newConfig *Config) error {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	// Validate the new configuration
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Update the configuration
	bc.cfg = newConfig

	// Save the updated configuration. This must be saveLocked: bc.Save() takes
	// bc.mux, which we already hold, and sync.Mutex is not reentrant -- calling
	// Save() here deadlocked the caller permanently.
	return bc.saveLocked()
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
