package sdk

// Node is a node in the blockchain network.
type Node struct {
	// Config is the node configuration
	Config *Config

	// Blockchain is the blockchain
	Blockchain *Blockchain

	// API is the API server
	API *API
}

// node is the node instance
var node *Node

// NewNode returns a new node instance.
func NewNode() *Node {
	node = &Node{}
	node.Config = NewConfig()
	node.Blockchain = NewBlockchain()
	node.API = NewAPI(node.Blockchain)

	return node
}

// Run runs the node.
func (n *Node) Run() {
	// Run the blockchain as a goroutine
	go n.Blockchain.Run(1)

	// Start the API server if enabled
	if EnableAPI {
		// Start the API server
		n.API.Start()
	} else {
		// This is to keep the main goroutine alive if API not enabled.
		select {}
	}
}
