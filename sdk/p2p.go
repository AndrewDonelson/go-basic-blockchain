// Package sdk is a software development kit for building blockchain applications.
// File: sdk/p2p.go
package sdk

import (
	"fmt"
	"strings"
	"sync"
	"time"
)

// P2PTransactionState is the current state of a P2P transaction
type P2PTransactionState int

const (
	// P2PTxNone is the initial state of a P2P transaction 0) NONE - Message is not broadcasted
	P2PTxNone = iota

	// P2PTxQueued is the state of a P2P transaction 1) QUEUED - Message is queued for broadcast
	P2PTxQueued

	// P2PTxPnd13 is the state of a P2P transaction 2) PND13 - Message is broadcasted to random 1/3 of nodes and is waiting on validations
	P2PTxPnd13

	// P2PTxValid is the state of a P2P transaction 3) VALID - Message is broadcasted to random 1/3 of nodes and received all 1/3 validations
	P2PTxValid

	// P2PTxPnd23 is the state of a P2P transaction 4) PND23 - Message is broadcasted to random 2/3 of nodes and is waiting on validations
	P2PTxPnd23

	// P2PTxFinal is the state of a P2P transaction 5) FINAL - Message is broadcasted to random 2/3 of nodes and received all 2/3 validations
	P2PTxFinal

	// P2PTxPnd is the state of a P2P transaction 6) PND - Message is broadcasted to all nodes and is waiting on validations
	P2PTxPnd

	// P2PTxArchived is the state of a P2P transaction 7) ARCHIVED - Message is broadcasted to all nodes and received all validations
	P2PTxArchived
)

// String returns the string representation of the P2PTransactionState
func (p *P2PTransactionState) String() string {
	switch *p {
	case P2PTxNone:
		return "NONE"
	case P2PTxQueued:
		return "QUEUED"
	case P2PTxPnd13:
		return "PND13"
	case P2PTxValid:
		return "VALID"
	case P2PTxPnd23:
		return "PND23"
	case P2PTxFinal:
		return "FINAL"
	case P2PTxPnd:
		return "PND"
	case P2PTxArchived:
		return "ARCHIVED"
	default:
		return "UNKNOWN"
	}
}

// Value returns the int representation of the P2PTransactionState
func (p *P2PTransactionState) Value(s string) int {
	switch strings.ToUpper(s) {
	case "NONE":
		return P2PTxNone
	case "QUEUED":
		return P2PTxQueued
	case "PND13":
		return P2PTxPnd13
	case "VALID":
		return P2PTxValid
	case "PND23":
		return P2PTxPnd23
	case "FINAL":
		return P2PTxFinal
	case "PND":
		return P2PTxPnd
	case "ARCHIVED":
		return P2PTxArchived
	default:
		return -1
	}

}

// Next returns the next state of the P2PTransactionState
func (p *P2PTransactionState) Next() {
	switch *p {
	case P2PTxNone:
		*p = P2PTxQueued
	case P2PTxQueued:
		*p = P2PTxPnd13
	case P2PTxPnd13:
		*p = P2PTxValid
	case P2PTxValid:
		*p = P2PTxPnd23
	case P2PTxPnd23:
		*p = P2PTxFinal
	case P2PTxFinal:
		*p = P2PTxPnd
	case P2PTxPnd:
		*p = P2PTxArchived
	case P2PTxArchived:
		*p = P2PTxArchived
	default:
		*p = P2PTxNone
	}
}

// P2P represents the P2P network.
type P2P struct {
	nodes   []*Node
	queue   []P2PTransaction
	mutex   sync.Mutex
	running bool
	// Other fields as per your requirements
}

// P2PTransaction represents a transaction to be processed.
type P2PTransaction struct {
	Tx
	Target string              // The target node for the Action. For example, to add a new node to the network, the target would be "node" and the node info will be in the Data field.
	Action string              // Can be "validate", "status", "add", "remove", or any command known to the P2P network.
	State  P2PTransactionState // The current state of the transaction
	Data   interface{}         // Depending on the Action, the data can be different. For example, if Action is "add", then Data can be a new node to be added to the network.
}

// NewP2P creates a new P2P network.
func NewP2P() *P2P {
	return &P2P{
		nodes: []*Node{},
		queue: []P2PTransaction{},
	}
}

// RegisterNode registers a new node with the P2P network.
func (p *P2P) RegisterNode(node *Node) {
	if node == nil {
		fmt.Printf("Cannot register empty or invalid node\n")
		return
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.nodes = append(p.nodes, node)
	fmt.Printf("Registered node: %s\n", node.ID)
}

// IsRegistered returns true if the given node is registered with the P2P network.
func (p *P2P) IsRegistered(nodeID string) bool {
	// Lock the P2P network
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Check if the node is registered
	for _, node := range p.nodes {
		if node.ID == nodeID {
			return true
		}
	}

	return false
}

// OneThird returns a value of one third of the total number of nodes in the P2P network.
// If there are less than 3 nodes in the network, then the value is 1.
// This is used for Fast Consensus and can possibly be reversed if any node is malicious.
// For example, if there are 9 nodes in the network, then one third is 3.
func (p *P2P) OneThird() float64 {
	return float64(len(p.nodes)) / 3.0
}

// GetRandomOneThird returns a random selection of one third of the total number of nodes in the P2P network.
func (p *P2P) GetRandomOneThird() []*Node {
	num := int(p.OneThird())
	selectedNodes := make([]*Node, num)
	for i := 0; i < num; i++ {
		selectedNodes[i] = p.nodes[i]
	}
	return selectedNodes
}

// TwoThirds returns a value of two thirds of the total number of nodes in the P2P network.
// If there are less than 3 nodes in the network, then the value is 2.
// This is used for the slower final permanent consensus.
// For example, if there are 9 nodes in the network, then two thirds is 6.
func (p *P2P) TwoThirds() float64 {
	return float64(len(p.nodes)) * 2 / 3.0
}

// GetRandomTwoThirds returns a random selection of two thirds of the total number of nodes in the P2P network.
func (p *P2P) GetRandomTwoThirds() []*Node {
	num := int(p.TwoThirds())
	selectedNodes := make([]*Node, num)
	for i := 0; i < num; i++ {
		selectedNodes[i] = p.nodes[i]
	}
	return selectedNodes
}

// AddTransaction adds a new transaction to the processing queue.
func (p *P2P) AddTransaction(tx P2PTransaction) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.queue = append(p.queue, tx)
	fmt.Printf("New transaction added to the queue: %s\n", tx.ID)
}

// HasTransaction checks if the P2P network has a specified transaction. it will request the same transaction from other nodes in the network.
func (p *P2P) HasTransaction(id string) bool {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	// Stack to store all blocks
	//stack := []*Block{}

	// Search the blockchain for the transaction
	// block := node.Blockchain.latestBlock
	// for block != nil {
	// 	stack = append(stack, block)
	// 	block = block.prevBlock
	// }

	// length := len(stack)
	// for i := length - 1; i >= 0; i-- {
	// 	block := stack[i]

	// 	for _, tx := range block.GetTransactions() {
	// 		if tx.ID == id {
	// 			fmt.Printf("Transaction found in the network: %s\n", tx.ID)
	// 			return true
	// 		}
	// 	}
	// }

	fmt.Printf("Transaction not found in the network: %s\n", id)
	return false
}

// ProcessQueue processes the pending transactions in the queue.
func (p *P2P) ProcessQueue() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, tx := range p.queue {
		fmt.Printf("Processing transaction: %s (%s)\n", tx.ID, tx.Action)
		// Process the transaction
		switch tx.Action {
		case "validate":
			// Validate the transaction
			// For example, you can validate the transaction signature
			// If the transaction is invalid, you can remove it from the queue
			// and return
			// If the transaction is valid, you can continue processing it
			// by calling node.ProcessP2PTransaction(tx)
			// For example:
			// if !tx.Validate() {
			// 	fmt.Printf("Invalid transaction: %s\n", tx.ID)
			// 	continue
			// }
			// node.ProcessP2PTransaction(tx)
		case "status":
			// Get the status of the node
			// For example, you can get the node's status by calling node.Status()
			// and then broadcast the status to all nodes in the network
			// For example:
			// status := node.Status()
			// p.Broadcast(P2PTransaction{
			// 	Tx:     Tx{ID: NewUUID()},
			// 	Target: "node",
			// 	Action: "status",
			// 	Data:   status,
			// })
		case "add":
			// Add a new node to the network
			// For example, you can add a new node by calling p.RegisterNode(node)
			// and then broadcast the new node to all nodes in the network
			// For example:
			// p.RegisterNode(node)
			// p.Broadcast(P2PTransaction{
			// 	Tx:     Tx{ID: NewUUID()},
			// 	Target: "node",
			// 	Action: "add",
			// 	Data:   node,
			// })
		case "remove":
			// Remove a node from the network
			// For example, you can remove a node by calling p.RemoveNode(node)
			// and then broadcast the removed node to all nodes in the network
			// For example:
			// p.RemoveNode(node)
			// p.Broadcast(P2PTransaction{
			// 	Tx:     Tx{ID: NewUUID()},
			// 	Target: "node",
			// 	Action: "remove",
			// 	Data:   node,
			// })
		case "register":
			// Register a new node to the network
			// For example, you can register a new node by calling p.RegisterNode(node)
			// and then broadcast the new node to all nodes in the network
			// For example:
			// p.RegisterNode(node)
			// p.Broadcast(P2PTransaction{
			// 	Tx:     Tx{ID: NewUUID()},
			// 	Target: "node",
			// 	Action: "register",
			// 	Data:   node,
			// })
		default:
			fmt.Printf("Unknown transaction: %s\n", tx.ID)
		}

	}

	// Clear the queue
	p.queue = []P2PTransaction{}
}

// Broadcast broadcasts a P2PTransaction to nodes in the network.
// 1) Broadcast a Message to random 1/3 of nodes. Upon validation it is then (VALID)
// 2) Broadcast a Message to random 2/3 of nodes. Finally upon validation it is (FINAL)
// 3) Broadcast to all nodes (ARCHIVED)
//
// Broadcast message States:
// 1) QUEUED - Message is queued for broadcast
// 2) PND13 - Message is broadcasted to random 1/3 of nodes and is waiting on validations
// 3) VALID - Message is broadcasted to random 1/3 of nodes and received all 1/3 validations
// 4) PND23 - Message is broadcasted to random 2/3 of nodes and is waiting on validations
// 5) FINAL - Message is broadcasted to random 2/3 of nodes and received all 2/3 validations
// 6) PND - Message is broadcasted to all nodes and is waiting on validations
// 7) ARCHIVED - Message is broadcasted to all nodes and received all validations
func (p *P2P) Broadcast(tx P2PTransaction) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, node := range p.nodes {
		// Send the transaction to each node
		node.ProcessP2PTransaction(tx)
		fmt.Printf("Broadcasted transaction: %s\n", tx.ID)
	}
}

// IsRunning returns true if the API is running
func (p *P2P) IsRunning() bool {
	return p.running
}

// Start starts the API and listens for incoming requests
func (p *P2P) Start() {

	if p.IsRunning() {
		return
	}

	// Create a logging middleware
	//api.router.Use(loggingMiddleware)

	// Start the P2P server
	fmt.Printf("P2P listening on %s\n", p2pHostname)
	p.running = true
	//log.Fatal(http.ListenAndServe(apiHostname, api.router))
}

// StartOld starts the P2P network.
func (p *P2P) StartOld() {
	// Start processing the transaction queue
	go func() {
		for {
			p.ProcessQueue()
			// Sleep for a certain duration before processing the next batch
			// You can adjust the duration based on your requirements
			time.Sleep(500 * time.Millisecond)
		}
	}()

	// Other initialization and connection logic
	// Add node discovery, message broadcasting, health monitoring, etc.
}
