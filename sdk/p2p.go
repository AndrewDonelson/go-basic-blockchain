package sdk

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
)

type NodeInfo struct {
	ID      string `json:"id"`
	Address string `json:"address"`
}

// P2PTransactionState represents the current state of a P2P transaction
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
	nodes      map[string]*Node
	queue      []P2PTransaction
	mutex      sync.RWMutex
	running    bool
	isSeedNode bool
	listener   net.Listener
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
		nodes: make(map[string]*Node),
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

	if _, exists := p.nodes[node.ID]; exists {
		return fmt.Errorf("node already registered: %s", node.ID)
	}

	p.nodes[node.ID] = node
	log.Printf("Registered node: %s\n", node.ID)
	return nil
}

// IsRegistered returns true if the given node is registered with the P2P network.
func (p *P2P) IsRegistered(nodeID string) bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	_, exists := p.nodes[nodeID]
	return exists
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
			log.Printf("Error broadcasting to node %s: %v\n", node.ID, err)
		}
	}

	return nil
}

// AddTransaction adds a new transaction to the processing queue.
func (p *P2P) AddTransaction(tx P2PTransaction) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.queue = append(p.queue, tx)
	log.Printf("New transaction added to the queue: %s\n", tx.ID)
}

// HasTransaction checks if the P2P network has a specified transaction.
func (p *P2P) HasTransaction(id *PUID) bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	for _, node := range p.nodes {
		if node.Blockchain.HasTransaction(id) {
			log.Printf("Transaction found in the network: %s\n", id)
			return true
		}
	}

	log.Printf("Transaction not found in the network: %s\n", id)
	return false
}

// ProcessQueue processes the pending transactions in the queue.
func (p *P2P) ProcessQueue() {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	for _, tx := range p.queue {
		log.Printf("Processing transaction: %s (%s)\n", tx.ID, tx.Action)

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
			log.Printf("Unknown transaction action: %s\n", tx.Action)
		}
	}

	// Clear the queue
	p.queue = []P2PTransaction{}
}

// Broadcast broadcasts a P2PTransaction to nodes in the network.
func (p *P2P) Broadcast(tx P2PTransaction) error {
	log.Println("Starting P2P broadcast")
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	if len(p.nodes) == 0 {
		log.Println("No nodes to broadcast to")
		return nil
	}

	for _, node := range p.nodes {
		log.Printf("Broadcasting to node: %s", node.ID)
		err := node.ProcessP2PTransaction(tx)
		if err != nil {
			log.Printf("Error broadcasting to node %s: %v", node.ID, err)
			return fmt.Errorf("error broadcasting to node %s: %w", node.ID, err)
		}
	}
	log.Printf("Broadcasted transaction: %s", tx.ID)
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

	log.Printf("P2P network starting on %s", p2pHostname)
	p.running = true

	go p.runProcessQueue()

	if p.isSeedNode {
		go p.listenForConnections()
	} else {
		go p.runNodeDiscovery()
	}

	return nil
}

func (p *P2P) Stop() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if !p.running {
		return errors.New("P2P network is not running")
	}

	p.running = false
	if p.listener != nil {
		p.listener.Close()
	}

	log.Println("P2P network stopped")
	return nil
}

func (p *P2P) runProcessQueue() {
	for p.IsRunning() {
		p.ProcessQueue()
		time.Sleep(500 * time.Millisecond)
	}
}

func (p *P2P) runNodeDiscovery() {
	s := spinner.New(spinner.CharSets[9], 100*time.Millisecond)
	log.Println("Starting node discovery...")
	s.Start()
	defer s.Stop()
	for p.IsRunning() {
		p.discoverNodes()
		time.Sleep(30 * time.Second)
	}
}

func (p *P2P) listenForConnections() {
	var err error
	p.listener, err = net.Listen("tcp", p2pHostname)
	if err != nil {
		log.Printf("Error starting P2P listener: %v", err)
		return
	}
	defer p.listener.Close()

	log.Printf("P2P seed node listening on %s", p2pHostname)

	for p.IsRunning() {
		conn, err := p.listener.Accept()
		if err != nil {
			log.Printf("Error accepting connection: %v", err)
			continue
		}
		go p.handleConnection(conn)
	}
}

func (p *P2P) handleConnection(conn net.Conn) {
	defer conn.Close()
	log.Printf("New connection from %s", conn.RemoteAddr())

	// Perform handshake
	err := p.performHandshake(conn)
	if err != nil {
		log.Printf("Handshake failed: %v", err)
		return
	}

	// Read and process messages
	reader := bufio.NewReader(conn)
	for {
		message, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				log.Printf("Error reading message: %v", err)
			}
			break
		}

		// Process the message
		err = p.processMessage(strings.TrimSpace(message), conn)
		if err != nil {
			log.Printf("Error processing message: %v", err)
			break
		}
	}
}

func (p *P2P) performHandshake(conn net.Conn) error {
	// Set a timeout for the handshake
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetDeadline(time.Time{}) // Reset the deadline

	// 1. Receive "HELLO" message
	reader := bufio.NewReader(conn)
	message, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to receive HELLO: %w", err)
	}
	if strings.TrimSpace(message) != "HELLO" {
		return fmt.Errorf("unexpected message: %s", message)
	}

	// 2. Send "ACK" message
	_, err = conn.Write([]byte("ACK\n"))
	if err != nil {
		return fmt.Errorf("failed to send ACK: %w", err)
	}

	// 3. Receive node information
	nodeInfoJSON, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to receive node info: %w", err)
	}

	var nodeInfo NodeInfo
	err = json.Unmarshal([]byte(nodeInfoJSON), &nodeInfo)
	if err != nil {
		return fmt.Errorf("failed to unmarshal node info: %w", err)
	}

	// 4. Send confirmation
	_, err = conn.Write([]byte("OK\n"))
	if err != nil {
		return fmt.Errorf("failed to send confirmation: %w", err)
	}

	// Register the new node
	newNode := &Node{
		ID:     nodeInfo.ID,
		Config: &Config{P2PHostName: nodeInfo.Address},
	}
	err = p.RegisterNode(newNode)
	if err != nil {
		return fmt.Errorf("failed to register node: %w", err)
	}

	return nil
}

func (p *P2P) processMessage(message string, conn net.Conn) error {
	switch message {
	case "GET_NODES":
		return p.sendNodeList(conn)
	default:
		return p.processP2PTransaction(message)
	}
}

func (p *P2P) sendNodeList(conn net.Conn) error {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	var nodeList []NodeInfo
	for _, node := range p.nodes {
		nodeList = append(nodeList, NodeInfo{
			ID:      node.ID,
			Address: node.Config.P2PHostName,
		})
	}

	nodeListJSON, err := json.Marshal(nodeList)
	if err != nil {
		return fmt.Errorf("failed to marshal node list: %w", err)
	}

	_, err = conn.Write(append(nodeListJSON, '\n'))
	if err != nil {
		return fmt.Errorf("failed to send node list: %w", err)
	}

	return nil
}

func (p *P2P) processP2PTransaction(message string) error {
	var tx P2PTransaction
	err := json.Unmarshal([]byte(message), &tx)
	if err != nil {
		return fmt.Errorf("failed to unmarshal P2P transaction: %w", err)
	}

	p.AddTransaction(tx)
	return nil
}

func (p *P2P) discoverNodes() {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	for _, node := range p.nodes {
		if node.ID != p.nodes[p.getSelfNodeID()].ID {
			newNodes, err := p.requestNodeList(node)
			if err != nil {
				log.Printf("Error requesting node list from %s: %v\n", node.ID, err)
				continue
			}

			for _, newNode := range newNodes {
				if !p.IsRegistered(newNode.ID) {
					err := p.RegisterNode(newNode)
					if err != nil {
						log.Printf("Error registering new node: %v\n", err)
					} else {
						log.Printf("Discovered new node: %s\n", newNode.ID)
					}
				}
			}
		}
	}
}

func (p *P2P) requestNodeList(node *Node) ([]*Node, error) {
	conn, err := net.Dial("tcp", node.Config.P2PHostName)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to node: %w", err)
	}
	defer conn.Close()

	_, err = conn.Write([]byte("GET_NODES\n"))
	if err != nil {
		return nil, fmt.Errorf("failed to send GET_NODES request: %w", err)
	}

	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to receive node list: %w", err)
	}

	var nodeInfoList []NodeInfo
	err = json.Unmarshal([]byte(response), &nodeInfoList)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal node list: %w", err)
	}

	var nodes []*Node
	for _, nodeInfo := range nodeInfoList {
		nodes = append(nodes, &Node{
			ID:     nodeInfo.ID,
			Config: &Config{P2PHostName: nodeInfo.Address},
		})
	}

	return nodes, nil
}

func (p *P2P) validateTransaction(tx P2PTransaction) {
	// Implement transaction validation logic
	log.Printf("Validating transaction: %s\n", tx.ID)
	// TODO: Implement actual validation logic
}

func (p *P2P) updateNodeStatus(tx P2PTransaction) {
	log.Printf("Updating node status: %s\n", tx.ID)
	var status NodeStatus
	err := json.Unmarshal(tx.Data.([]byte), &status)
	if err != nil {
		log.Printf("Error unmarshaling node status: %v\n", err)
		return
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if node, exists := p.nodes[status.NodeID]; exists {
		node.Status = status.Status
		node.LastSeen = time.Now()
		log.Printf("Updated status of node %s: %s\n", status.NodeID, status.Status)
	} else {
		log.Printf("Node %s not found in the network\n", status.NodeID)
	}
}

func (p *P2P) addNode(tx P2PTransaction) {
	log.Printf("Adding new node: %s\n", tx.ID)
	var newNode Node
	err := json.Unmarshal(tx.Data.([]byte), &newNode)
	if err != nil {
		log.Printf("Error unmarshaling new node data: %v\n", err)
		return
	}

	err = p.RegisterNode(&newNode)
	if err != nil {
		log.Printf("Error registering new node: %v\n", err)
	}
}

func (p *P2P) removeNode(tx P2PTransaction) {
	log.Printf("Removing node: %s\n", tx.ID)
	var nodeID string
	err := json.Unmarshal(tx.Data.([]byte), &nodeID)
	if err != nil {
		log.Printf("Error unmarshaling node ID: %v\n", err)
		return
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if _, exists := p.nodes[nodeID]; exists {
		delete(p.nodes, nodeID)
		log.Printf("Removed node from the network: %s\n", nodeID)
	} else {
		log.Printf("Node %s not found in the network\n", nodeID)
	}
}

func (p *P2P) registerNode(tx P2PTransaction) {
	log.Printf("Registering new node: %s\n", tx.ID)
	var newNode Node
	err := json.Unmarshal(tx.Data.([]byte), &newNode)
	if err != nil {
		log.Printf("Error unmarshaling new node data: %v\n", err)
		return
	}

	err = p.RegisterNode(&newNode)
	if err != nil {
		log.Printf("Error registering new node: %v\n", err)
		return
	}

	// Broadcast the new node to all other nodes
	p.BroadcastMessage(P2PTransaction{
		Tx:     tx.Tx,
		Target: "all",
		Action: "add",
		Data:   tx.Data,
	})
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

func (p *P2P) SetAsSeedNode() {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.isSeedNode = true
	log.Println("This node is set as a seed node")
}

func (p *P2P) ConnectToSeedNode(address string) error {
	log.Printf("Connecting to seed node at %s\n", address)

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to connect to seed node: %w", err)
	}
	defer conn.Close()

	// Perform handshake
	err = p.performClientHandshake(conn)
	if err != nil {
		return fmt.Errorf("handshake failed: %w", err)
	}

	// Request node list
	nodeList, err := p.requestNodeListFromSeed(conn)
	if err != nil {
		return fmt.Errorf("failed to get node list from seed: %w", err)
	}

	// Add nodes from the received list
	for _, nodeInfo := range nodeList {
		newNode := &Node{
			ID:     nodeInfo.ID,
			Config: &Config{P2PHostName: nodeInfo.Address},
		}
		err := p.RegisterNode(newNode)
		if err != nil {
			log.Printf("Error registering node from seed: %v\n", err)
		} else {
			log.Printf("Added node from seed: %s (%s)\n", newNode.ID, newNode.Config.P2PHostName)
		}
	}

	return nil
}

func (p *P2P) performClientHandshake(conn net.Conn) error {
	// Set a timeout for the handshake
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetDeadline(time.Time{}) // Reset the deadline

	// 1. Send a "HELLO" message
	_, err := conn.Write([]byte("HELLO\n"))
	if err != nil {
		return fmt.Errorf("failed to send HELLO: %w", err)
	}

	// 2. Receive an "ACK" message
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to receive ACK: %w", err)
	}
	response = strings.TrimSpace(response)
	if response != "ACK" {
		return fmt.Errorf("unexpected response: %s", response)
	}

	// 3. Send node information
	selfNode := p.nodes[p.getSelfNodeID()]
	nodeInfo := NodeInfo{
		ID:      selfNode.ID,
		Address: selfNode.Config.P2PHostName,
	}
	nodeInfoJSON, err := json.Marshal(nodeInfo)
	if err != nil {
		return fmt.Errorf("failed to marshal node info: %w", err)
	}
	_, err = conn.Write(append(nodeInfoJSON, '\n'))
	if err != nil {
		return fmt.Errorf("failed to send node info: %w", err)
	}

	// 4. Receive confirmation
	response, err = reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("failed to receive confirmation: %w", err)
	}
	response = strings.TrimSpace(response)
	if response != "OK" {
		return fmt.Errorf("unexpected confirmation: %s", response)
	}

	return nil
}

func (p *P2P) requestNodeListFromSeed(conn net.Conn) ([]NodeInfo, error) {
	// Set a timeout for the request
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	defer conn.SetDeadline(time.Time{}) // Reset the deadline

	// 1. Send a "GET_NODES" message
	_, err := conn.Write([]byte("GET_NODES\n"))
	if err != nil {
		return nil, fmt.Errorf("failed to send GET_NODES: %w", err)
	}

	// 2. Receive a list of node information
	reader := bufio.NewReader(conn)
	response, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to receive node list: %w", err)
	}

	// 3. Parse and return the node list
	var nodeList []NodeInfo
	err = json.Unmarshal([]byte(response), &nodeList)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal node list: %w", err)
	}

	return nodeList, nil
}

func (p *P2P) getSelfNodeID() string {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	for id, node := range p.nodes {
		if node.Config.P2PHostName == p2pHostname {
			return id
		}
	}
	return ""
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
