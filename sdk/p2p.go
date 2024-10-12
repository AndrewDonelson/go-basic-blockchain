// Package sdk is a software development kit for building blockchain applications.
// File: sdk/p2p.go
// This file handles syncing transactions across the network and broadcasting messages to all nodes in the network.
package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"strings"
	"sync"
	"time"
)

// P2PTransactionState is the current state of a P2P transaction
type P2PTransactionState int

const (
	P2PTxNone P2PTransactionState = iota
	P2PTxQueued
	P2PTxPnd13
	P2PTxValid
	P2PTxPnd23
	P2PTxFinal
	P2PTxPnd
	P2PTxArchived
)

func (p P2PTransactionState) String() string {
	return [...]string{"NONE", "QUEUED", "PND13", "VALID", "PND23", "FINAL", "PND", "ARCHIVED"}[p]
}

func P2PTransactionStateFromString(s string) (P2PTransactionState, error) {
	switch strings.ToUpper(s) {
	case "NONE":
		return P2PTxNone, nil
	case "QUEUED":
		return P2PTxQueued, nil
	case "PND13":
		return P2PTxPnd13, nil
	case "VALID":
		return P2PTxValid, nil
	case "PND23":
		return P2PTxPnd23, nil
	case "FINAL":
		return P2PTxFinal, nil
	case "PND":
		return P2PTxPnd, nil
	case "ARCHIVED":
		return P2PTxArchived, nil
	default:
		return P2PTxNone, fmt.Errorf("invalid P2PTransactionState: %s", s)
	}
}

func (p *P2PTransactionState) Next() {
	if *p < P2PTxArchived {
		*p++
	}
}

// P2P represents the P2P network.
type P2P struct {
	nodes   []*Node
	queue   []P2PTransaction
	mutex   sync.RWMutex
	running bool
}

// P2PTransaction represents a transaction to be processed.
type P2PTransaction struct {
	Tx
	Target string
	Action string
	State  P2PTransactionState
	Data   interface{}
}

// NewP2P creates a new P2P network.
func NewP2P() *P2P {
	return &P2P{
		nodes: []*Node{},
		queue: []P2PTransaction{},
	}
}

// RegisterNode registers a new node with the P2P network.
func (p *P2P) RegisterNode(node *Node) error {
	if node == nil {
		return errors.New("cannot register empty or invalid node")
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.IsRegistered(node.ID) {
		return fmt.Errorf("node already registered: %s", node.ID)
	}

	p.nodes = append(p.nodes, node)
	fmt.Printf("Registered node: %s\n", node.ID)
	return nil
}

// IsRegistered returns true if the given node is registered with the P2P network.
func (p *P2P) IsRegistered(nodeID string) bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	for _, node := range p.nodes {
		if node.ID == nodeID {
			return true
		}
	}
	return false
}

// BroadcastMessage broadcasts a p2p message to all nodes in the network
func (p *P2P) BroadcastMessage(msg P2PTransaction) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if len(p.nodes) == 0 {
		return errors.New("no nodes in the network to broadcast to")
	}

	for _, node := range p.nodes {
		err := node.ProcessP2PTransaction(msg)
		if err != nil {
			fmt.Printf("Error broadcasting to node %s: %v\n", node.ID, err)
		}
	}

	return nil
}

// OneThird returns a value of one third of the total number of nodes in the P2P network.
func (p *P2P) OneThird() int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return max(1, len(p.nodes)/3)
}

// GetRandomOneThird returns a random selection of one third of the total number of nodes in the P2P network.
func (p *P2P) GetRandomOneThird() []*Node {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	num := p.OneThird()
	return p.getRandomNodes(num)
}

// TwoThirds returns a value of two thirds of the total number of nodes in the P2P network.
func (p *P2P) TwoThirds() int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	return max(2, (len(p.nodes)*2)/3)
}

// GetRandomTwoThirds returns a random selection of two thirds of the total number of nodes in the P2P network.
func (p *P2P) GetRandomTwoThirds() []*Node {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	num := p.TwoThirds()
	return p.getRandomNodes(num)
}

// getRandomNodes returns a random selection of n nodes from the P2P network.
func (p *P2P) getRandomNodes(n int) []*Node {
	if n > len(p.nodes) {
		n = len(p.nodes)
	}

	shuffled := make([]*Node, len(p.nodes))
	copy(shuffled, p.nodes)
	rand.Shuffle(len(shuffled), func(i, j int) {
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	})

	return shuffled[:n]
}

// AddTransaction adds a new transaction to the processing queue.
func (p *P2P) AddTransaction(tx P2PTransaction) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.queue = append(p.queue, tx)
	fmt.Printf("New transaction added to the queue: %s\n", tx.ID)
}

// HasTransaction checks if the P2P network has a specified transaction.
func (p *P2P) HasTransaction(id *PUID) bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	for _, node := range p.nodes {
		// Assuming each node has a method to check for a transaction
		if node.Blockchain.HasTransaction(id) {
			fmt.Printf("Transaction found in the network: %s\n", id)
			return true
		}
	}

	fmt.Printf("Transaction not found in the network: %s\n", id)
	return false
}

// ProcessQueue processes the pending transactions in the queue.
func (p *P2P) ProcessQueue() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, tx := range p.queue {
		fmt.Printf("Processing transaction: %s (%s)\n", tx.ID, tx.Action)

		switch tx.Action {
		case "validate":
			p.validateTransaction(tx)
		case "status":
			p.updateNodeStatus(tx)
		case "add":
			p.addNode(tx)
		case "remove":
			p.removeNode(tx)
		case "register":
			p.registerNode(tx)
		default:
			fmt.Printf("Unknown transaction action: %s\n", tx.Action)
		}
	}

	// Clear the queue
	p.queue = []P2PTransaction{}
}

// Broadcast broadcasts a P2PTransaction to nodes in the network.
func (p *P2P) Broadcast(tx P2PTransaction) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if len(p.nodes) == 0 {
		return errors.New("cannot broadcast to empty or invalid nodes")
	}

	for _, node := range p.nodes {
		err := node.ProcessP2PTransaction(tx)
		if err != nil {
			fmt.Printf("Error broadcasting to node %s: %v\n", node.ID, err)
		} else {
			fmt.Printf("Broadcasted transaction: %s to node: %s\n", tx.ID, node.ID)
		}
	}

	return nil
}

// IsRunning returns true if the P2P network is running
func (p *P2P) IsRunning() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.running
}

// Start starts the P2P network
func (p *P2P) Start() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.running {
		return errors.New("P2P network is already running")
	}

	fmt.Printf("P2P network starting on %s\n", p2pHostname)
	p.running = true

	go p.runProcessQueue()
	go p.runNodeDiscovery()

	return nil
}

func (p *P2P) runProcessQueue() {
	for p.IsRunning() {
		p.ProcessQueue()
		time.Sleep(500 * time.Millisecond)
	}
}

func (p *P2P) runNodeDiscovery() {
	for p.IsRunning() {
		p.discoverNodes()
		time.Sleep(30 * time.Second)
	}
}

func (p *P2P) discoverNodes() {
	// Implement node discovery logic here
	// This could involve reaching out to known seed nodes or using a DHT
	fmt.Println("Running node discovery...")
}

func (p *P2P) validateTransaction(tx P2PTransaction) {
	// Implement transaction validation logic
	fmt.Printf("Validating transaction: %s\n", tx.ID)
}

func (p *P2P) updateNodeStatus(tx P2PTransaction) {
	// Implement node status update logic
	fmt.Printf("Updating node status: %s\n", tx.ID)
}

func (p *P2P) addNode(tx P2PTransaction) {
	// Implement logic to add a new node to the network
	fmt.Printf("Adding new node: %s\n", tx.ID)
}

func (p *P2P) removeNode(tx P2PTransaction) {
	// Implement logic to remove a node from the network
	fmt.Printf("Removing node: %s\n", tx.ID)
}

func (p *P2P) registerNode(tx P2PTransaction) {
	// Implement logic to register a new node in the network
	fmt.Printf("Registering new node: %s\n", tx.ID)
}

func (p *P2P) BroadcastStatus(node *Node, status string) error {
	nodeStatus := NodeStatus{
		NodeID: node.ID,
		Status: status,
	}
	statusData, err := json.Marshal(nodeStatus)
	if err != nil {
		return fmt.Errorf("error marshaling node status: %w", err)
	}

	tx, err := NewTransaction("p2p", node.Wallet, nil)
	if err != nil {
		return fmt.Errorf("error creating transaction: %w", err)
	}

	p2pTx := P2PTransaction{
		Tx:     *tx,
		Target: "all",
		Action: "status",
		Data:   statusData,
	}

	return p.Broadcast(p2pTx)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
