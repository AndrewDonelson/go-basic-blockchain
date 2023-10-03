// Package sdk is a software development kit for building blockchain applications.
// File  sdk/node.go - Node for all Node related Protocol based transactions
package sdk

import (
	"encoding/json"
	"fmt"

	"github.com/pborman/uuid"
)

// NodeOptions is the options for a node.
type NodeOptions struct {
	// EnvName is the environment filename name (.XXXXX)
	EnvName string

	// DataPath is the path to the node data directory
	DataPath string

	// Config is the custom node configuration
	Config *Config
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

// Node is a node in the blockchain network.
type Node struct {
	// this will ne true if the node is initialized and is validated & ready for use
	initialized bool

	// ID is the unique node ID
	ID string

	// Config is the node configuration
	Config *Config

	// Blockchain is the blockchain
	Blockchain *Blockchain

	// API is the API server
	API *API

	// P2P is the P2P network
	P2P *P2P
}

// node is the node instance
var node *Node

// NewNode returns a new node instance.
func NewNode(opts *NodeOptions) *Node {

	// Create a new default node instance
	node = &Node{}

	if opts != nil {
		node.Config = opts.Config
	} else {
		node.Config = NewConfig()
	}

	// Create a new node LocalStorage instance
	localStorage = NewLocalStorage(node.Config.DataPath)

	err := node.load()
	if err != nil {
		fmt.Printf("No existing node state found: %s\n", err)

		// Create a unique ID for new node
		node.ID = uuid.New()

		// save node state
		node.save()
	}

	node.Blockchain = NewBlockchain(node.Config)
	node.API = NewAPI(node.Blockchain)
	node.P2P = NewP2P()

	// Register the our node with the P2P network
	node.Register()

	// Show the node config
	node.Config.Show()

	node.initialized = true

	return node
}

// IsReady returns true if the node is ready for use.
func (n *Node) IsReady() bool {
	return n.initialized
}

// save saves the node state to disk.
func (n *Node) save() error {
	// Create the node persist data
	data := &NodePersistData{
		ID:     n.ID,
		Config: n.Config,
		//Blockchain: n.Blockchain.persistData(),
	}

	// Save the node state to disk
	err := localStorage.Set("state", data)
	if err != nil {
		return err
	}

	fmt.Printf("Saved node state: %s\n", n.ID)

	return nil
}

// load loads the node state from disk.
func (n *Node) load() error {
	// Load the node state from disk
	data := &NodePersistData{}
	err := localStorage.Get("state", data)
	if err != nil {
		return err
	}

	// Set the node state
	n.ID = data.ID
	n.Config = data.Config

	fmt.Printf("Loaded node state: %s\n", n.ID)
	return nil
}

// Run runs the node.
func (n *Node) Run() {

	// Start the P2P network as a goroutine
	go n.P2P.Start()

	// Run the blockchain as a goroutine
	go n.Blockchain.Run(1)

	// Start the API server if enabled
	if n.Config.EnableAPI {
		// Start the API server
		n.API.Start()
	} else {
		// This is to keep the main goroutine alive if API not enabled.
		select {}
	}
}

// ProcessP2PTransaction processes a P2PTransaction received from the P2P network.
func (n *Node) ProcessP2PTransaction(tx P2PTransaction) {
	// Process the P2P transaction based on the Action and Data
	// You can implement the logic here to handle different types of P2P transactions
	fmt.Printf("Processing P2P transaction: %s (%s)\n", tx.ID, tx.Protocol)
}

// Register registers the node with the P2P network.
// Example usage:
func (n *Node) Register() {
	// Register the node with the P2P network
	n.P2P.RegisterNode(n)

	jsonNodeData, _ := json.Marshal(n)

	// Example P2P transaction
	p2pTx := P2PTransaction{
		Tx:     Tx{},
		Target: "node",
		Action: "register",
		Data:   jsonNodeData,
	}

	// Add the P2P transaction to the P2P network
	n.P2P.AddTransaction(p2pTx)

	// Broadcast the P2P transaction to all nodes
	//n.P2P.Broadcast(p2pTx)
}
