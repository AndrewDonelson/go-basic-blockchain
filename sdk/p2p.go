package sdk

import (
	"fmt"
	"sync"
	"time"
)

// P2P represents the P2P network.
type P2P struct {
	nodes []*Node
	queue []P2PTransaction
	mutex sync.Mutex
	// Other fields as per your requirements
}

// P2PTransaction represents a transaction to be processed.
type P2PTransaction struct {
	Tx
	Target string      // The target node for the Action. For example, to add a new node to the network, the target would be "node" and the node info will be in the Data field.
	Action string      // Can be "validate", "status", "add", "remove", or any command known to the P2P network.
	Data   interface{} // Depending on the Action, the data can be different. For example, if Action is "add", then Data can be a new node to be added to the network.
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

// AddTransaction adds a new transaction to the processing queue.
func (p *P2P) AddTransaction(tx P2PTransaction) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.queue = append(p.queue, tx)
	fmt.Printf("New transaction added to the queue: %s\n", tx.ID)
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

// Broadcast broadcasts a P2PTransaction to all nodes in the network.
func (p *P2P) Broadcast(tx P2PTransaction) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, node := range p.nodes {
		// Send the transaction to each node
		node.ProcessP2PTransaction(tx)
		fmt.Printf("Broadcasted transaction: %s\n", tx.ID)
	}
}

// StartP2P starts the P2P network.
func (p *P2P) Start() {
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
