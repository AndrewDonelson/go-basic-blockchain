// Package sdk is a software development kit for building blockchain applications.
// File  sdk/node.go - Node for all Node related Protocol based transactions
package sdk

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/pborman/uuid"
)

// NodeOptions is the options for a node.
type NodeOptions struct {
	EnvName  string
	DataPath string
	Config   *Config
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
	NodeID string
	Status string
	// Add other status fields as needed
}

// Node is a node in the blockchain network.
type Node struct {
	sync.Mutex
	initialized bool
	LastSeen    time.Time
	Status      string
	ID          string
	Config      *Config
	Blockchain  *Blockchain
	API         *API
	P2P         *P2P
	Wallet      *Wallet
}

// node is the node instance
var node *Node

func GetNode() *Node {
	return node
}

// NewNode returns a new node instance.
func NewNode(opts *NodeOptions) error {
	node = &Node{}
	node.Lock()
	defer node.Unlock()

	if opts != nil {
		node.Config = opts.Config
	} else {
		node.Config = NewConfig()
	}

	err := NewLocalStorage(node.Config.DataPath)
	if err != nil {
		return fmt.Errorf("error initializing local storage: %w", err)
	}

	err = node.load()
	if err != nil {
		fmt.Printf("No existing node state found: %s\n", err)
		node.ID = uuid.New()
	}

	err = node.save()
	if err != nil {
		return fmt.Errorf("error saving node state: %w", err)
	}

	node.Blockchain = NewBlockchain(node.Config)
	node.API = NewAPI(node.Blockchain)
	node.P2P = NewP2P()

	// Initialize wallet
	walletOptions := &WalletOptions{
		OrganizationID: NewBigInt(1), // Example value, adjust as needed
		AppID:          NewBigInt(1), // Example value, adjust as needed
		UserID:         NewBigInt(1), // Example value, adjust as needed
		AssetID:        NewBigInt(1), // Example value, adjust as needed
		Name:           "NodeWallet",
		Passphrase:     generateRandomPassphrase(),
		Tags:           []string{"node", "wallet"},
	}
	wallet, err := NewWallet(walletOptions)
	if err != nil {
		return fmt.Errorf("error creating node wallet: %w", err)
	}
	node.Wallet = wallet

	err = node.Register()
	if err != nil {
		return fmt.Errorf("error registering node: %w", err)
	}

	node.Config.Show()
	node.initialized = true

	return nil
}

func DefaultNodeOptions() *NodeOptions {
	return &NodeOptions{
		EnvName:  "chaind",
		DataPath: "./chaind_data",
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

	fmt.Printf("Loaded node state: %s\n", n.ID)
	return nil
}

// Run runs the node.
func (n *Node) Run() {
	go n.P2P.Start()
	go n.Blockchain.Run(1)

	if n.Config.EnableAPI {
		go n.API.Start()
	}

	// Keep the main goroutine alive
	select {}
}

// ProcessP2PTransaction processes a P2PTransaction received from the P2P network.
func (n *Node) ProcessP2PTransaction(tx P2PTransaction) error {
	n.Lock()
	defer n.Unlock()

	if n.Wallet == nil {
		return errors.New("node wallet is nil")
	}

	fmt.Printf("Processing P2P transaction: %s (%s)\n", tx.ID, tx.Protocol)

	switch tx.Action {
	case "validate":
		return n.validateTransaction(tx)
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
	if n.Wallet == nil {
		return errors.New("node wallet is nil")
	}

	n.P2P.RegisterNode(n)

	jsonNodeData, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("error marshaling node data: %w", err)
	}

	tx, err := NewTransaction("chain", n.Wallet, n.Wallet)
	if err != nil {
		return fmt.Errorf("error creating transaction: %w", err)
	}

	p2pTx := P2PTransaction{
		Tx:     *tx,
		Target: "node",
		Action: "register",
		Data:   jsonNodeData,
	}

	n.P2P.AddTransaction(p2pTx)
	n.P2P.Broadcast(p2pTx)

	return nil
}

func (n *Node) validateTransaction(tx P2PTransaction) error {
	isValid, err := tx.Tx.Verify([]byte(tx.Tx.From.PublicPEM()), tx.Tx.GetSignature())
	if err != nil {
		return fmt.Errorf("error validating transaction: %w", err)
	}

	if isValid {
		fmt.Printf("Transaction %s is valid\n", tx.ID)
		n.Blockchain.AddTransaction(&tx.Tx)
	} else {
		fmt.Printf("Transaction %s is invalid\n", tx.ID)
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

	for i, node := range n.P2P.nodes {
		if node.ID == status.NodeID {
			n.P2P.nodes[i].LastSeen = time.Now()
			n.P2P.nodes[i].Status = status.Status
			fmt.Printf("Updated status of node %s: %s\n", status.NodeID, status.Status)
			return nil
		}
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

	for _, node := range n.P2P.nodes {
		if node.ID == newNode.ID {
			return fmt.Errorf("node %s already exists in the network", newNode.ID)
		}
	}

	n.P2P.nodes = append(n.P2P.nodes, &newNode)
	fmt.Printf("Added new node to the network: %s\n", newNode.ID)
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

	for i, node := range n.P2P.nodes {
		if node.ID == nodeID {
			n.P2P.nodes = append(n.P2P.nodes[:i], n.P2P.nodes[i+1:]...)
			fmt.Printf("Removed node from the network: %s\n", nodeID)
			return nil
		}
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

	for _, node := range n.P2P.nodes {
		if node.ID == newNode.ID {
			return fmt.Errorf("node %s is already registered in the network", newNode.ID)
		}
	}

	n.P2P.nodes = append(n.P2P.nodes, &newNode)
	fmt.Printf("Registered new node in the network: %s\n", newNode.ID)

	n.P2P.Broadcast(P2PTransaction{
		Tx:     tx.Tx,
		Target: "all",
		Action: "add",
		Data:   tx.Data,
	})

	return nil
}

func generateRandomPassphrase() string {
	b := make([]byte, 32)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", b)
}
