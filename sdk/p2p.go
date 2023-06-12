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
func NewP2P(node *Node) *P2P {
	return &P2P{
		nodes: []*Node{},
		queue: []P2PTransaction{},
	}
}

// RegisterNode registers a new node with the P2P network.
func (p *P2P) RegisterNode(node *Node) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.nodes = append(p.nodes, node)
	fmt.Printf("New node registered: %s\n", node.ID)
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
		fmt.Printf("Processing transaction: %s\n", tx.ID)
		// Process the transaction
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
	}

	fmt.Printf("Broadcasted transaction: %s\n", tx.ID)
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
