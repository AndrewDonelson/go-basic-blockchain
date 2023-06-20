// Package sdk is a software development kit for building blockchain applications.
// File  sdk/node.go - Node for all Node related Protocol based transactions
package sdk

import (
	"encoding/json"
	"fmt"
)

// Node is a node in the blockchain network.
type Node struct {
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
func NewNode() *Node {
	node = &Node{}
	node.Config = NewConfig()
	localStorage = NewLocalStorage(node.Config.DataPath)
	node.Blockchain = NewBlockchain(node.Config)
	node.API = NewAPI(node.Blockchain)
	node.P2P = NewP2P(node)

	return node
}

// Run runs the node.
func (n *Node) Run() {
	// Run the blockchain as a goroutine
	go n.Blockchain.Run(1)

	// Register the our node with the P2P network
	n.Register()

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
	fmt.Printf("Processing P2P transaction: %+v\n", tx)
}

// Example usage:
func (n *Node) Register() {
	// Register the node with the P2P network
	n.P2P.RegisterNode(node)

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
	n.P2P.Broadcast(p2pTx)
}
