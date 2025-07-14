// Package sdk is a software development kit for building blockchain applications.
// File  sdk/node.go - Node for all Node related Protocol based transactions
package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/progress"
	"github.com/pborman/uuid"
)

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
	initialized       bool
	LastSeen          time.Time
	Status            string
	ID                string
	Config            *Config
	Blockchain        *Blockchain
	API               *API
	P2P               *P2P
	Wallet            *Wallet
	ProgressIndicator *progress.ProgressIndicator
}

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

	node = &Node{
		ID:                uuid.New(),
		Config:            opts.Config,
		Status:            "initializing",
		ProgressIndicator: progress.NewProgressIndicator(),
	}

	// Initialize blockchain
	blockchain := NewBlockchain(opts.Config)
	if blockchain == nil {
		return errors.New("failed to create blockchain")
	}
	node.Blockchain = blockchain

	// Initialize API
	if opts.Config.EnableAPI {
		api := NewAPI(blockchain)
		if api == nil {
			return errors.New("failed to create API")
		}
		node.API = api
	}

	// Initialize P2P
	p2p := NewP2P()
	if p2p == nil {
		return errors.New("failed to create P2P")
	}
	node.P2P = p2p

	// Initialize wallet with a strong password
	strongPassword, err := GenerateRandomPassword()
	if err != nil {
		return fmt.Errorf("failed to generate strong password: %v", err)
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
		return fmt.Errorf("failed to create wallet: %v", err)
	}
	node.Wallet = wallet

	// Load existing data
	err = node.load()
	if err != nil {
		log.Printf("No existing node state found, creating new node")
	}

	node.initialized = true
	node.Status = "ready"

	log.Printf("Node initialized: %s", node.ID)
	return nil
}

func DefaultNodeOptions() *NodeOptions {
	return &NodeOptions{
		EnvName:  "chaind",
		DataPath: "./data",
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
func (n *Node) save() error {
	data := &NodePersistData{
		ID:     n.ID,
		Config: n.Config,
	}

	err := localStorage.Set("state", data)
	if err != nil {
		return fmt.Errorf("error saving node state: %w", err)
	}

	fmt.Printf("Saved node state: %s\n", n.ID)
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

	log.Printf("Loaded node state: %s\n", n.ID)
	return nil
}

// LogEvent is a custom logger function that prints a message with a newline
// after clearing the current line where the spinner is displayed
func LogEvent(format string, args ...interface{}) {
	// Clear the current line containing the spinner
	fmt.Print("\r                                                    \r")
	log.Printf(format, args...)
}

// Run runs the node.
func (n *Node) Run() {
	log.Println("Starting node...")

	// Start progress indicator
	if n.ProgressIndicator != nil {
		n.ProgressIndicator.Start()
		n.ProgressIndicator.ShowInfo("Node starting up...")
	}

	// Start P2P network
	go n.P2P.Start()
	log.Println("P2P network starting on :8101")

	if n.Blockchain == nil {
		log.Println("Error: Blockchain is not initialized")
		n.ProgressIndicator.ShowError("Blockchain not initialized")
		return
	}

	n.ProgressIndicator.ShowSuccess("Blockchain initialized successfully")
	go n.Blockchain.Run(n.Config.Difficulty)

	if n.Config.EnableAPI {
		go n.API.Start()
		n.ProgressIndicator.ShowInfo("API server started")
	}

	// Set up a channel to capture log output
	logCh := make(chan bool, 10)

	// Blockchain-themed spinner animation
	spinnerFrames := []string{
		"[⬛⬜⬜⬜⬛] >>",
		"[⬛⬜⬜⬛⬜] >>",
		"[⬛⬜⬛⬜⬜] >>",
		"[⬛⬛⬜⬜⬜] >>",
		"[⬜⬛⬜⬜⬛] >>",
		"[⬜⬜⬛⬜⬛] >>",
		"[⬜⬜⬜⬛⬛] >>",
	}
	frameIndex := 0

	// Create a ticker for updating the spinner
	spinnerTick := time.NewTicker(150 * time.Millisecond)
	defer spinnerTick.Stop()

	// Create a ticker for updating blockchain stats
	statsTick := time.NewTicker(2 * time.Second)
	defer statsTick.Stop()

	// Create a ticker for network status updates
	networkTick := time.NewTicker(5 * time.Second)
	defer networkTick.Stop()

	// Check if we're running in a terminal that supports ANSI escape sequences
	// This is a simple check and might not work in all environments
	_, isTerminal := os.LookupEnv("TERM")

	// Keep the main goroutine alive
	for {
		select {
		case <-spinnerTick.C:
			if isTerminal {
				// Update the spinner animation
				frameIndex = (frameIndex + 1) % len(spinnerFrames)
				blockCount := 0
				txCount := 0

				if n.Blockchain != nil {
					blockCount = n.Blockchain.GetBlockCount()
					txCount = len(n.Blockchain.TransactionQueue)
				}

				// Display the spinner with blockchain stats using fixed-width formatting
				fmt.Printf("\r%s Node: %-8s | Blocks: %-4d | TXs: %-3d",
					spinnerFrames[frameIndex],
					n.ID[:8],
					blockCount,
					txCount)
			}
		case <-statsTick.C:
			// Periodically update blockchain status
			// This helps ensure we don't miss important state changes
			if n.Blockchain != nil {
				n.Blockchain.DisplayStatus()
			}
		case <-networkTick.C:
			// Update network status
			if n.P2P != nil {
				peerCount := len(n.P2P.nodes)
				isSynced := true // TODO: Implement actual sync status
				n.ProgressIndicator.ShowNetworkStatus(peerCount, isSynced)
			}
		case <-logCh:
			// This would be triggered by a significant event
			// The actual logging happens in event handlers
		}
	}
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
	n.P2P.RegisterNode(n)
	LogEvent("Node registered with P2P network")

	LogEvent("Marshaling node data to JSON")
	jsonNodeData, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("error marshaling node data: %w", err)
	}
	LogEvent("Node data marshaled to JSON")

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
	isValid, err := tx.Tx.Verify([]byte(tx.Tx.From.PublicPEM()), tx.Tx.GetSignature())
	if err != nil {
		return fmt.Errorf("error validating transaction: %w", err)
	}

	if isValid {
		LogEvent("Transaction %s is valid", tx.ID)
		n.Blockchain.AddTransaction(&tx.Tx)
	} else {
		LogEvent("Transaction %s is invalid", tx.ID)
	}

	return nil
}

func (n *Node) updateStatus(tx P2PTransaction) error {
	var status NodeStatus
	data, ok := tx.Data.([]byte)
	if !ok {
		return errors.New("error asserting tx.Data to []byte")
	}
	err := json.Unmarshal(data, &status)
	if err != nil {
		return fmt.Errorf("error unmarshaling node status: %w", err)
	}

	if node, exists := n.P2P.nodes[status.NodeID]; exists {
		node.LastSeen = time.Now()
		node.Status = status.Status
		LogEvent("Updated status of node %s: %s", status.NodeID, status.Status)
		return nil
	}

	return fmt.Errorf("node %s not found in the network", status.NodeID)
}

func (n *Node) addNode(tx P2PTransaction) error {
	var newNode Node
	data, ok := tx.Data.([]byte)
	if !ok {
		return errors.New("error asserting tx.Data to []byte")
	}
	err := json.Unmarshal(data, &newNode)
	if err != nil {
		return fmt.Errorf("error unmarshaling new node data: %w", err)
	}

	if _, exists := n.P2P.nodes[newNode.ID]; exists {
		return fmt.Errorf("node %s already exists in the network", newNode.ID)
	}

	n.P2P.nodes[newNode.ID] = &newNode
	LogEvent("Added new node to the network: %s", newNode.ID)
	return nil
}

func (n *Node) removeNode(tx P2PTransaction) error {
	var nodeID string
	data, ok := tx.Data.([]byte)
	if !ok {
		return errors.New("error asserting tx.Data to []byte")
	}
	err := json.Unmarshal(data, &nodeID)
	if err != nil {
		return fmt.Errorf("error unmarshaling node ID: %w", err)
	}

	if _, exists := n.P2P.nodes[nodeID]; exists {
		delete(n.P2P.nodes, nodeID)
		LogEvent("Removed node from the network: %s", nodeID)
		return nil
	}

	return fmt.Errorf("node %s not found in the network", nodeID)
}

func (n *Node) registerNode(tx P2PTransaction) error {
	var newNode Node
	data, ok := tx.Data.([]byte)
	if !ok {
		return errors.New("error asserting tx.Data to []byte")
	}
	err := json.Unmarshal(data, &newNode)
	if err != nil {
		return fmt.Errorf("error unmarshaling new node data: %w", err)
	}

	if _, exists := n.P2P.nodes[newNode.ID]; exists {
		return fmt.Errorf("node %s is already registered in the network", newNode.ID)
	}

	n.P2P.nodes[newNode.ID] = &newNode
	LogEvent("Registered new node in the network: %s", newNode.ID)

	n.P2P.Broadcast(P2PTransaction{
		Tx:     tx.Tx,
		Target: "all",
		Action: "add",
		Data:   tx.Data,
	})

	return nil
}
