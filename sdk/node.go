// Package sdk is a software development kit for building blockchain applications.
// File  sdk/node.go - Node for all Node related Protocol based transactions
package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"sync"
	"time"

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

func NewNode(opts *NodeOptions) error {
	log.Println("Starting NewNode function")
	node = &Node{}
	node.Lock()
	defer node.Unlock()

	if opts != nil {
		node.Config = opts.Config
	} else {
		node.Config = NewConfig()
	}
	log.Println("Config initialized")

	err := NewLocalStorage(node.Config.DataPath)
	if err != nil {
		return fmt.Errorf("error initializing local storage: %w", err)
	}
	log.Println("Local storage initialized")

	nodePath := filepath.Join("./", "node.json")
	if fileExists(nodePath) {
		err := node.load()
		if err != nil {
			return fmt.Errorf("error loading existing node: %w", err)
		}
		log.Println("Loaded existing node configuration")
	} else {
		node.ID = uuid.New()
		err := node.save()
		if err != nil {
			return fmt.Errorf("error saving new node: %w", err)
		}
		log.Println("Created and saved new node configuration")
	}

	node.Blockchain = NewBlockchain(node.Config)
	log.Println("Blockchain initialized")

	node.API = NewAPI(node.Blockchain)
	log.Println("API initialized")

	node.P2P = NewP2P()
	log.Println("P2P initialized")

	// Initialize wallet
	password, err := GenerateRandomPassword()
	if err != nil {
		return fmt.Errorf("error generating random password: %w", err)
	}
	log.Println("Random password generated")

	walletOptions := &WalletOptions{
		OrganizationID: NewBigInt(1),
		AppID:          NewBigInt(1),
		UserID:         NewBigInt(1),
		AssetID:        NewBigInt(1),
		Name:           "NodeWallet",
		Passphrase:     password,
		Tags:           []string{"node", "wallet"},
	}
	wallet, err := NewWallet(walletOptions)
	if err != nil {
		return fmt.Errorf("error creating node wallet: %w", err)
	}
	node.Wallet = wallet
	log.Println("Node wallet created")

	if opts.IsSeed {
		log.Println("Initializing as seed node")
		node.P2P.SetAsSeedNode()
		log.Println("Node set as seed node")
	} else if opts.SeedAddress != "" {
		log.Println("Attempting to connect to seed node")
		err := node.P2P.ConnectToSeedNode(opts.SeedAddress)
		if err != nil {
			return fmt.Errorf("failed to connect to seed node: %w", err)
		}
		log.Println("Connected to seed node")

		log.Println("Registering node with P2P network")
		err = node.Register()
		if err != nil {
			return fmt.Errorf("error registering node: %w", err)
		}
		log.Println("Node registered with P2P network")
	} else {
		log.Println("Warning: Node is neither a seed node nor connected to a seed node")
	}

	node.Config.Show()
	node.initialized = true

	err = node.save()
	if err != nil {
		return fmt.Errorf("error saving node state: %w", err)
	}
	log.Println("Node state saved")

	log.Println("Node initialization complete")

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

	log.Printf("Loaded node state: %s\n", n.ID)
	return nil
}

// Run runs the node.
func (n *Node) Run() {
	log.Println("Starting node...")
	go n.P2P.Start()
	go n.Blockchain.Run(n.Config.Difficulty)

	if n.Config.EnableAPI {
		go n.API.Start()
	}

	// Keep the main goroutine alive
	for {
		time.Sleep(time.Second)
		log.Printf(".")
	}
}

// ProcessP2PTransaction processes a P2PTransaction received from the P2P network.
func (n *Node) ProcessP2PTransaction(tx P2PTransaction) error {
	n.Lock()
	defer n.Unlock()

	if n.Wallet == nil {
		return errors.New("node wallet is nil")
	}

	log.Printf("Processing P2P transaction: %s (%s)\n", tx.ID, tx.Protocol)

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
	log.Println("Starting node registration")
	if n.Wallet == nil {
		return errors.New("node wallet is nil")
	}

	log.Println("Registering node with P2P network")
	n.P2P.RegisterNode(n)
	log.Println("Node registered with P2P network")

	log.Println("Marshaling node data to JSON")
	jsonNodeData, err := json.Marshal(n)
	if err != nil {
		return fmt.Errorf("error marshaling node data: %w", err)
	}
	log.Println("Node data marshaled to JSON")

	log.Println("Creating new transaction")
	tx, err := NewTransaction("chain", n.Wallet, n.Wallet)
	if err != nil {
		return fmt.Errorf("error creating transaction: %w", err)
	}
	log.Println("New transaction created")

	p2pTx := P2PTransaction{
		Tx:     *tx,
		Target: "node",
		Action: "register",
		Data:   jsonNodeData,
	}

	log.Println("Adding transaction to P2P network")
	n.P2P.AddTransaction(p2pTx)
	log.Println("Transaction added to P2P network")

	log.Println("Broadcasting transaction to P2P network")
	err = n.P2P.Broadcast(p2pTx)
	if err != nil {
		return fmt.Errorf("error broadcasting transaction: %w", err)
	}
	log.Println("Transaction broadcast completed")

	return nil
}

func (n *Node) validateTransaction(tx P2PTransaction) error {
	isValid, err := tx.Tx.Verify([]byte(tx.Tx.From.PublicPEM()), tx.Tx.GetSignature())
	if err != nil {
		return fmt.Errorf("error validating transaction: %w", err)
	}

	if isValid {
		log.Printf("Transaction %s is valid\n", tx.ID)
		n.Blockchain.AddTransaction(&tx.Tx)
	} else {
		log.Printf("Transaction %s is invalid\n", tx.ID)
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
		log.Printf("Updated status of node %s: %s\n", status.NodeID, status.Status)
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
	log.Printf("Added new node to the network: %s\n", newNode.ID)
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
		log.Printf("Removed node from the network: %s\n", nodeID)
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
	log.Printf("Registered new node in the network: %s\n", newNode.ID)

	n.P2P.Broadcast(P2PTransaction{
		Tx:     tx.Tx,
		Target: "all",
		Action: "add",
		Data:   tx.Data,
	})

	return nil
}
