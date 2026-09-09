// Package sdk is a software development kit for building blockchain applications.
// File  sdk/node.go - Node for all Node related Protocol based transactions
package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/progress"
)

// envNodeWalletPassphrase names the environment variable holding the node
// wallet's passphrase.
const envNodeWalletPassphrase = "NODE_WALLET_PASSPHRASE"

// nodeShutdownTimeout bounds the graceful shutdown of the HTTP server.
const nodeShutdownTimeout = 10 * time.Second

// NodeOptions is the options for a node.
type NodeOptions struct {
	EnvName     string
	DataPath    string
	Config      *Config
	IsSeed      bool
	SeedAddress string
}

// NewNodeOptions creates a new NodeOptions instance.
func NewNodeOptions(envName string, path string, cfg *Config) *NodeOptions {
	nodeOptions := &NodeOptions{
		EnvName:  envName,
		DataPath: path,
		Config:   cfg,
	}

	nodeOptions.Config.DataPath = path
	return nodeOptions
}

// NodePersistData is the data that is persisted for a node to disk.
type NodePersistData struct {
	ID     string
	Config *Config
}

type NodeStatus struct {
	NodeID string `json:"node_id"`
	Status string `json:"status"`
}

// Node is a node in the blockchain network.
type Node struct {
	sync.Mutex
	cancel            context.CancelFunc
	shutdownOnce      sync.Once
	initialized       bool
	StartedAt         time.Time
	LastSeen          time.Time
	Status            string
	ID                string
	Config            *Config
	Blockchain        *Blockchain
	API               *API
	P2P               *P2P
	Wallet            *Wallet
	ProgressIndicator *progress.ProgressIndicator

	// Syncer pulls blocks from peers with longer chains.
	Syncer *Syncer

	// Identity is this node's long-term keypair. Node.ID is derived from it.
	Identity *PeerIdentity
}

// (Node embeds sync.Mutex, so Lock/Unlock are already promoted; the hand-written
// wrappers that used to sit here just called the embedded methods.)

// node is the node instance
var node *Node

func GetNode() *Node {
	return node
}

// NewNode creates a new node with the given options.
func NewNode(opts *NodeOptions) error {
	if node != nil {
		return errors.New("node already exists")
	}

	built, err := newNode(opts)
	if err != nil {
		return err
	}

	node = built
	LogInfof("Node initialized: %s", node.ID)
	return nil
}

// newNode builds a Node without touching the package-level singleton.
//
// NewNode used to do both, which made a node impossible to construct twice in
// one process: the second call returned "node already exists". That is why every
// meaningful test in node_test.go was skipped. Construction and registration are
// now separate concerns, so a test can build as many independent nodes as it
// needs while the CLI keeps its single global one.
func newNode(opts *NodeOptions) (*Node, error) {
	if opts == nil {
		return nil, errors.New("node options cannot be nil")
	}
	if opts.Config == nil {
		opts.Config = NewConfig()
	}

	// Carry the seed settings onto the Config, which is what the running node
	// actually reads. They used to be set on NodeOptions and then dropped, so
	// --seed and --seed-address had no effect on a running node.
	opts.Config.IsSeed = opts.IsSeed || opts.Config.IsSeed
	if opts.SeedAddress != "" {
		opts.Config.SeedAddress = opts.SeedAddress
	}
	if opts.DataPath != "" && opts.Config.DataPath == "" {
		opts.Config.DataPath = opts.DataPath
	}

	// The node's identity keypair. Its ID is the hash of the public key, so the
	// ID is self-certifying: it used to be a random UUID the node asserted about
	// itself, which any host could claim.
	identity, err := LoadOrCreatePeerIdentity(opts.Config.DataPath)
	if err != nil {
		return nil, fmt.Errorf("failed to establish node identity: %w", err)
	}

	n := &Node{
		Identity:          identity,
		ID:                identity.NodeID,
		Config:            opts.Config,
		Status:            "initializing",
		ProgressIndicator: progress.NewProgressIndicator(),
	}

	// Initialize blockchain
	blockchain := NewBlockchain(opts.Config)
	if blockchain == nil {
		return nil, errors.New("failed to create blockchain")
	}
	n.Blockchain = blockchain

	// Initialize API
	if opts.Config.EnableAPI {
		api := NewAPI(blockchain)
		if api == nil {
			return nil, errors.New("failed to create API")
		}
		n.API = api
	}

	// Initialize P2P and connect it to the chain, so this node can both serve
	// sync requests and apply what peers send it.
	p2p := NewP2P()
	if p2p == nil {
		return nil, errors.New("failed to create P2P")
	}
	n.P2P = p2p

	p2p.SetChain(blockchain)
	p2p.SetIdentity(identity)
	p2p.SetSelfInfo(n.ID, opts.Config.P2PHostName)
	p2p.SetAllowedPeers(opts.Config.AllowedPeers)
	if opts.IsSeed {
		p2p.SetAsSeedNode()
	}

	// Relay locally produced blocks and transactions. The chain calls these
	// hooks; it does not import the P2P layer.
	blockchain.SetBlockAnnouncer(p2p.AnnounceBlock)
	blockchain.SetTransactionAnnouncer(p2p.AnnounceTransaction)

	syncer, err := NewSyncer(SyncerOptions{
		Chain:     blockchain,
		Transport: p2p,
		Peers:     p2p,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create chain syncer: %w", err)
	}
	n.Syncer = syncer

	// Initialize the node wallet.
	//
	// The passphrase comes from configuration (NODE_WALLET_PASSPHRASE). It used to
	// be generated with GenerateRandomPassword() and then dropped on the floor, so
	// the wallet was encrypted with a key nobody had: the node could never sign
	// with it again, and every restart orphaned another wallet file on disk.
	strongPassword, err := nodeWalletPassphrase()
	if err != nil {
		return nil, fmt.Errorf("failed to resolve node wallet passphrase: %w", err)
	}

	walletOptions := NewWalletOptions(
		NewBigInt(1),               // organizationID
		NewBigInt(1),               // appID
		NewBigInt(1),               // userID
		NewBigInt(1),               // assetID
		"NodeWallet",               // name
		strongPassword,             // passphrase
		[]string{"node", "wallet"}, // tags
	)
	wallet, err := NewWallet(walletOptions)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet: %w", err)
	}
	n.Wallet = wallet

	// Load existing data
	err = n.load()
	if err != nil {
		LogVerbosef("No existing node state found, creating new node")
	}

	n.initialized = true
	n.Status = "ready"

	return n, nil
}

// nodeWalletPassphrase resolves the node wallet passphrase from the environment.
//
// When NODE_WALLET_PASSPHRASE is unset a passphrase is generated and reported
// once, prominently, so the operator can persist it. Silently generating an
// unrecoverable one is what made the node wallet useless.
func nodeWalletPassphrase() (string, error) {
	if pass := getEnv(envNodeWalletPassphrase, ""); pass != "" {
		if err := testPasswordStrength(pass); err != nil {
			return "", fmt.Errorf("%s is too weak: %w", envNodeWalletPassphrase, err)
		}
		return pass, nil
	}

	generated, err := GenerateRandomPassword()
	if err != nil {
		return "", err
	}

	LogInfof("No %s configured; generated one for this node wallet. "+
		"Save it now -- the wallet cannot be recovered without it: %s",
		envNodeWalletPassphrase, generated)

	return generated, nil
}

func DefaultNodeOptions() *NodeOptions {
	home, err := os.UserHomeDir()
	dataPath := "./data"
	if err == nil {
		dataPath = filepath.Join(home, "gbb-data")
	}
	return &NodeOptions{
		EnvName:  "chaind",
		DataPath: dataPath,
		Config:   NewConfig(),
	}
}

// IsReady returns true if the node is ready for use.
func (n *Node) IsReady() bool {
	n.Lock()
	defer n.Unlock()
	return n.initialized
}

// save saves the node state to disk.
// This function is currently unused but kept for potential future use
//
//nolint:unused
func (n *Node) save() error {
	data := &NodePersistData{
		ID:     n.ID,
		Config: n.Config,
	}

	err := localStorage.Set("state", data)
	if err != nil {
		return fmt.Errorf("error saving node state: %w", err)
	}

	LogVerbosef("Saved node state: %s", n.ID)
	return nil
}

// load loads the node state from disk.
func (n *Node) load() error {
	data := &NodePersistData{}
	err := localStorage.Get("state", data)
	if err != nil {
		return fmt.Errorf("error loading node state: %w", err)
	}

	n.ID = data.ID
	n.Config = data.Config

	LogVerbosef("Loaded node state: %s", n.ID)
	return nil
}

// LogEvent is a custom logger function that prints a message with a newline
// after clearing the current line where the spinner is displayed
func LogEvent(format string, args ...interface{}) {
	// Clear the current line containing the spinner
	fmt.Print("\r                                                    \r")
	LogInfof(format, args...)
}

// Run runs the node until the process is terminated.
func (n *Node) Run() {
	n.RunContext(context.Background())
}

// RunContext runs the node until ctx is cancelled, then shuts it down cleanly.
//
// The old Run ended in a bare `select {}` with no exit path: there was no way to
// stop the node, drain the HTTP server, close the P2P listener, or flush state.
func (n *Node) RunContext(ctx context.Context) {
	LogInfof("Starting node...")

	ctx, cancel := context.WithCancel(ctx)
	n.Lock()
	n.cancel = cancel
	if n.StartedAt.IsZero() {
		n.StartedAt = time.Now()
	}
	n.LastSeen = n.StartedAt
	n.Unlock()

	// Start progress indicator
	if n.ProgressIndicator != nil {
		n.ProgressIndicator.Start()
		n.ProgressIndicator.ShowInfo("Node starting up...")
	}

	if n.Blockchain == nil {
		LogInfof("Error: Blockchain is not initialized")
		if n.ProgressIndicator != nil {
			n.ProgressIndicator.ShowError("Blockchain not initialized")
		}
		cancel()
		return
	}

	// Start P2P network
	if n.P2P != nil {
		go func() {
			if err := n.P2P.Start(); err != nil {
				LogInfof("P2P network failed to start: %v", err)
			}
		}()
		LogInfof("P2P network starting on %s", n.Config.P2PHostName)
	}

	if n.ProgressIndicator != nil {
		n.ProgressIndicator.ShowSuccess("Blockchain initialized successfully")
	}
	go n.Blockchain.RunContext(ctx, n.Config.Difficulty)

	// Connect to the configured seed node, then start pulling from peers.
	if n.Config.SeedAddress != "" && n.P2P != nil {
		go func() {
			// The node's context, so a shutdown during the initial seed dial is
			// not held open for the full dial timeout.
			if err := n.P2P.ConnectToSeedNode(ctx, n.Config.SeedAddress); err != nil {
				LogInfof("Could not connect to seed node %s: %v", n.Config.SeedAddress, err)
			}
		}()
	}

	if n.Syncer != nil {
		//nolint:gosec // G118: ctx is the node's own context and is passed through
		go n.Syncer.Run(ctx, defaultSyncInterval)
		LogInfof("Chain sync started (every %s)", defaultSyncInterval)
	}

	if n.Config.EnableAPI && n.API != nil {
		go func() {
			if err := n.API.Start(); err != nil {
				LogInfof("API server stopped: %v", err)
			}
		}()
		if n.ProgressIndicator != nil {
			n.ProgressIndicator.ShowInfo("API server started")
		}
	}

	<-ctx.Done()
	n.shutdown()
}

// Stop signals the node to shut down.
func (n *Node) Stop() {
	n.Lock()
	cancel := n.cancel
	n.Unlock()

	if cancel != nil {
		cancel()
	}
}

// shutdown releases the node's resources in dependency order.
func (n *Node) shutdown() {
	n.shutdownOnce.Do(func() {
		LogInfof("Shutting down node...")

		if n.API != nil {
			shutdownCtx, cancel := context.WithTimeout(context.Background(), nodeShutdownTimeout)
			defer cancel()
			if err := n.API.Stop(shutdownCtx); err != nil {
				LogInfof("Error stopping API server: %v", err)
			}
		}

		if n.P2P != nil {
			if err := n.P2P.Stop(); err != nil {
				LogVerbosef("Error stopping P2P network: %v", err)
			}
		}

		if n.Blockchain != nil {
			n.Blockchain.Cleanup()
		}

		if n.ProgressIndicator != nil {
			n.ProgressIndicator.Stop()
		}

		n.Lock()
		n.Status = "stopped"
		n.Unlock()

		LogInfof("Node shutdown complete")
	})
}

// ProcessP2PTransaction processes a P2PTransaction received from the P2P network.
func (n *Node) ProcessP2PTransaction(tx P2PTransaction) error {
	n.Lock()
	defer n.Unlock()

	if n.Wallet == nil {
		return errors.New("node wallet is nil")
	}

	LogEvent("Processing P2P transaction: %s (%s)", tx.ID, tx.Protocol)

	// Show transaction progress
	if n.ProgressIndicator != nil {
		n.ProgressIndicator.ShowTransactionProgress(tx.ID.String(), "validating")
	}

	switch tx.Action {
	case "validate":
		err := n.validateTransaction(tx)
		if err != nil {
			n.ProgressIndicator.ShowError(fmt.Sprintf("Transaction validation failed: %v", err))
		} else {
			n.ProgressIndicator.ShowTransactionProgress(tx.ID.String(), "confirmed")
		}
		return err
	case "status":
		return n.updateStatus(tx)
	case "add":
		return n.addNode(tx)
	case "remove":
		return n.removeNode(tx)
	case "register":
		return n.registerNode(tx)
	default:
		return fmt.Errorf("unknown transaction action: %s", tx.Action)
	}
}

// Register registers the node with the P2P network.
func (n *Node) Register() error {
	LogEvent("Starting node registration")
	if n.Wallet == nil {
		return errors.New("node wallet is nil")
	}

	LogEvent("Registering node with P2P network")
	if err := n.P2P.RegisterNode(n); err != nil {
		// Log error but continue
		_ = err // Suppress unused variable warning
	}
	LogEvent("Node registered with P2P network")

	// Announce only this node's identity and address.
	//
	// This used to marshal the entire Node -- which transitively includes the
	// whole Blockchain (every block) and the Wallet's ciphertext -- and pushed all
	// of it into a broadcast message.
	address := ""
	if n.Config != nil {
		address = n.Config.P2PHostName
	}
	jsonNodeData, err := json.Marshal(NodeInfo{ID: n.ID, Address: address})
	if err != nil {
		return fmt.Errorf("error marshaling node data: %w", err)
	}

	LogEvent("Creating new transaction")
	tx, err := NewTransaction("chain", n.Wallet, n.Wallet)
	if err != nil {
		return fmt.Errorf("error creating transaction: %w", err)
	}
	LogEvent("New transaction created")

	p2pTx := P2PTransaction{
		Tx:     *tx,
		Target: "node",
		Action: "register",
		Data:   jsonNodeData,
	}

	LogEvent("Adding transaction to P2P network")
	n.P2P.AddTransaction(p2pTx)
	LogEvent("Transaction added to P2P network")

	LogEvent("Broadcasting transaction to P2P network")
	err = n.P2P.Broadcast(p2pTx)
	if err != nil {
		return fmt.Errorf("error broadcasting transaction: %w", err)
	}
	LogEvent("Transaction broadcast completed")

	return nil
}

func (n *Node) validateTransaction(tx P2PTransaction) error {
	if tx.Tx.From == nil {
		return errors.New("transaction has no sender wallet")
	}

	isValid, err := tx.Tx.Verify([]byte(tx.Tx.From.PublicPEM()), tx.Tx.GetSignature())
	if err != nil {
		return fmt.Errorf("error validating transaction: %w", err)
	}

	if !isValid {
		LogEvent("Transaction %s is invalid", tx.GetID())
		return nil
	}

	LogEvent("Transaction %s is valid", tx.GetID())
	if n.Blockchain != nil {
		n.Blockchain.AddTransaction(&tx.Tx)
	}
	return nil
}

// The node-management handlers (updateStatus/addNode/removeNode/registerNode) used
// to be duplicated here, mutating n.P2P.nodes directly and without P2P's mutex --
// a concurrent map write, which is an unrecoverable fatal error rather than a
// catchable panic. They now delegate to the P2P implementations, which hold the
// correct lock, so there is one implementation instead of two divergent ones.

func (n *Node) updateStatus(tx P2PTransaction) error {
	if n.P2P == nil {
		return errors.New("p2p network is not initialized")
	}
	return n.P2P.updateNodeStatus(tx)
}

func (n *Node) addNode(tx P2PTransaction) error {
	if n.P2P == nil {
		return errors.New("p2p network is not initialized")
	}
	return n.P2P.addNode(tx)
}

func (n *Node) removeNode(tx P2PTransaction) error {
	if n.P2P == nil {
		return errors.New("p2p network is not initialized")
	}
	return n.P2P.removeNode(tx)
}

func (n *Node) registerNode(tx P2PTransaction) error {
	if n.P2P == nil {
		return errors.New("p2p network is not initialized")
	}
	return n.P2P.registerNode(tx)
}
