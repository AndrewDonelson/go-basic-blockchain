package sdk

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strings"
	"sync"
	"time"
)

type NodeInfo struct {
	ID      string `json:"id"`
	Address string `json:"address"`
}

const (
	// maxP2PMessageSize caps a single peer message. Without a cap a peer could
	// stream bytes without a newline until the process ran out of memory.
	maxP2PMessageSize = 1 << 20 // 1 MiB
	// p2pReadTimeout bounds how long a peer may keep a connection idle.
	p2pReadTimeout = 60 * time.Second
	// p2pHandshakeTimeout bounds the handshake exchange.
	p2pHandshakeTimeout = 10 * time.Second
)

// readLimitedLine reads a newline-terminated message from an already size-limited
// reader, converting a truncated read into an explicit error rather than silently
// treating a partial message as complete.
func readLimitedLine(reader *bufio.Reader) (string, error) {
	message, err := reader.ReadString('\n')
	if err != nil {
		if errors.Is(err, io.EOF) && message != "" {
			return "", fmt.Errorf("peer message exceeded %d bytes or was truncated", maxP2PMessageSize)
		}
		return "", err
	}
	return message, nil
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
	switch p {
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
//
// Data is json.RawMessage rather than interface{}: after a JSON round-trip an
// interface{} is never []byte (it is nil, a string, or a map), so the
// `tx.Data.([]byte)` assertions this type used to require either panicked or
// silently failed on every message that arrived over the wire.
type P2PTransaction struct {
	Tx     `json:"tx"`
	Target string              `json:"target"`
	Action string              `json:"action"`
	State  P2PTransactionState `json:"state"`
	Data   json.RawMessage     `json:"data"`
}

// p2pTransactionWire is the on-the-wire shape of a P2PTransaction.
//
// P2PTransaction embeds Tx, which defines MarshalJSON on a pointer receiver. Go
// promotes that method, so json.Marshal(&p2pTx) used to serialise *only the base
// transaction* and silently drop Target, Action, State and Data -- every P2P
// message on the network was undispatchable. Explicit marshalling keeps the
// envelope intact.
type p2pTransactionWire struct {
	Tx     json.RawMessage     `json:"tx"`
	Target string              `json:"target"`
	Action string              `json:"action"`
	State  P2PTransactionState `json:"state"`
	Data   json.RawMessage     `json:"data"`
}

// MarshalJSON serialises the full P2P envelope.
func (p P2PTransaction) MarshalJSON() ([]byte, error) {
	base := p.Tx
	txBytes, err := json.Marshal(&base)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal embedded transaction: %w", err)
	}
	return json.Marshal(p2pTransactionWire{
		Tx:     txBytes,
		Target: p.Target,
		Action: p.Action,
		State:  p.State,
		Data:   p.Data,
	})
}

// UnmarshalJSON restores the full P2P envelope.
func (p *P2PTransaction) UnmarshalJSON(data []byte) error {
	var wire p2pTransactionWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	if len(wire.Tx) > 0 {
		if err := json.Unmarshal(wire.Tx, &p.Tx); err != nil {
			return fmt.Errorf("failed to unmarshal embedded transaction: %w", err)
		}
	}
	p.Target = wire.Target
	p.Action = wire.Action
	p.State = wire.State
	p.Data = wire.Data
	return nil
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
	LogInfof("Registered node: %s", node.ID)
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
	peers := p.peerSnapshot()
	if len(peers) == 0 {
		return errors.New("no nodes in the network to broadcast to")
	}

	for _, node := range peers {
		if err := node.ProcessP2PTransaction(msg); err != nil {
			LogInfof("Error broadcasting to node %s: %v", node.ID, err)
		}
	}

	return nil
}

// peerSnapshot returns the currently registered peers. Callers iterate the copy so
// they never hold p.mutex while calling back into node code that re-enters it.
func (p *P2P) peerSnapshot() []*Node {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	peers := make([]*Node, 0, len(p.nodes))
	for _, node := range p.nodes {
		if node != nil {
			peers = append(peers, node)
		}
	}
	return peers
}

// AddTransaction adds a new transaction to the processing queue.
func (p *P2P) AddTransaction(tx P2PTransaction) {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	p.queue = append(p.queue, tx)
	LogVerbosef("New transaction added to the queue: %s", tx.ID)
}

// HasTransaction checks if the P2P network has a specified transaction.
func (p *P2P) HasTransaction(id *PUID) bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()

	for _, node := range p.nodes {
		if node == nil || node.Blockchain == nil {
			continue
		}
		if node.Blockchain.HasTransaction(id) {
			LogInfof("Transaction found in the network: %s", id)
			return true
		}
	}

	LogInfof("Transaction not found in the network: %s", id)
	return false
}

// ProcessQueue processes the pending transactions in the queue.
//
// The queue is drained under the lock and then processed *without* it. Previously
// this held p.mutex for the whole loop while calling handlers that each took
// p.mutex again -- sync.Mutex is not reentrant, so the first "add"/"remove"/
// "status"/"register" message deadlocked the P2P subsystem for the process
// lifetime. (sdk/p2p_test.go skipped TestProcessQueue because of exactly this.)
func (p *P2P) ProcessQueue() {
	p.mutex.Lock()
	pending := p.queue
	p.queue = []P2PTransaction{}
	p.mutex.Unlock()

	for _, tx := range pending {
		LogVerbosef("Processing transaction: %s (%s)", tx.GetID(), tx.Action)

		var err error
		switch tx.Action {
		case "validate":
			p.validateTransaction(tx)
		case "status":
			err = p.updateNodeStatus(tx)
		case "add":
			err = p.addNode(tx)
		case "remove":
			err = p.removeNode(tx)
		case "register":
			err = p.registerNode(tx)
		default:
			err = fmt.Errorf("unknown transaction action: %s", tx.Action)
		}
		if err != nil {
			LogInfof("Error processing P2P transaction %s (%s): %v", tx.GetID(), tx.Action, err)
		}
	}
}

// Broadcast broadcasts a P2PTransaction to nodes in the network.
func (p *P2P) Broadcast(tx P2PTransaction) error {
	LogInfof("Starting P2P broadcast")
	if node := GetNode(); node != nil && node.ProgressIndicator != nil {
		node.ProgressIndicator.UpdateAction("Broadcasting")
	}
	// Snapshot under the read lock, then deliver without it: ProcessP2PTransaction
	// reaches back into the P2P layer, which would re-enter this mutex.
	peers := p.peerSnapshot()
	if len(peers) == 0 {
		LogVerbosef("No nodes to broadcast to")
		return nil
	}

	// Deliver to every peer. Returning on the first failure meant one unreachable
	// peer silently prevented delivery to all the peers after it.
	var failures []string
	for _, node := range peers {
		LogVerbosef("Broadcasting to node: %s", node.ID)
		if err := node.ProcessP2PTransaction(tx); err != nil {
			LogInfof("Error broadcasting to node %s: %v", node.ID, err)
			failures = append(failures, node.ID)
		}
	}

	if len(failures) > 0 {
		return fmt.Errorf("broadcast failed for %d of %d nodes: %s",
			len(failures), len(peers), strings.Join(failures, ", "))
	}

	LogVerbosef("Broadcasted transaction: %s", tx.GetID())
	return nil
}

// IsRunning returns true if the P2P network is running
func (p *P2P) IsRunning() bool {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.running
}

// PeerCount returns the number of registered peers.
func (p *P2P) PeerCount() int {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return len(p.nodes)
}

func (p *P2P) getBindAddress() string {
	if n := GetNode(); n != nil && n.Config != nil && n.Config.P2PHostName != "" {
		return n.Config.P2PHostName
	}
	return p2pHostname
}

// Start starts the P2P network
func (p *P2P) Start() error {
	p.mutex.Lock()
	defer p.mutex.Unlock()

	if p.running {
		return errors.New("P2P network is already running")
	}

	LogInfof("P2P network starting on %s", p.getBindAddress())
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

	LogInfof("P2P network stopped")
	return nil
}

func (p *P2P) runProcessQueue() {
	for p.IsRunning() {
		p.ProcessQueue()
		time.Sleep(500 * time.Millisecond)
	}
}

func (p *P2P) runNodeDiscovery() {
	LogInfof("Starting node discovery...")
	if node := GetNode(); node != nil && node.ProgressIndicator != nil {
		node.ProgressIndicator.UpdateAction("Syncing")
	}
	for p.IsRunning() {
		p.discoverNodes()
		time.Sleep(30 * time.Second)
	}
}

func (p *P2P) listenForConnections() {
	var err error
	bindAddr := p.getBindAddress()
	p.listener, err = net.Listen("tcp", bindAddr)
	if err != nil {
		LogInfof("Error starting P2P listener: %v", err)
		return
	}
	defer p.listener.Close()

	LogInfof("P2P seed node listening on %s", bindAddr)

	for p.IsRunning() {
		conn, err := p.listener.Accept()
		if err != nil {
			// A closed listener returns an error on every call, so `continue`
			// here span the CPU at 100% after Stop().
			if errors.Is(err, net.ErrClosed) || !p.IsRunning() {
				return
			}
			LogInfof("Error accepting connection: %v", err)
			// Back off so a persistent accept error cannot become a hot loop.
			time.Sleep(100 * time.Millisecond)
			continue
		}
		go p.handleConnection(conn)
	}
}

func (p *P2P) handleConnection(conn net.Conn) {
	defer conn.Close()
	LogInfof("New connection from %s", conn.RemoteAddr())

	// Perform handshake
	err := p.performHandshake(conn)
	if err != nil {
		LogInfof("Handshake failed: %v", err)
		return
	}

	// Read and process messages.
	//
	// Every read is bounded in both size and time. bufio.Reader.ReadString has no
	// size limit, so a peer that never sent a newline could exhaust memory, and
	// with no deadline a peer that simply stalled held a goroutine forever.
	reader := bufio.NewReader(io.LimitReader(conn, maxP2PMessageSize))
	for {
		if err := conn.SetReadDeadline(time.Now().Add(p2pReadTimeout)); err != nil {
			LogInfof("Error setting read deadline: %v", err)
			return
		}

		message, err := readLimitedLine(reader)
		if err != nil {
			if !errors.Is(err, io.EOF) {
				LogInfof("Error reading message: %v", err)
			}
			return
		}

		if err := p.processMessage(strings.TrimSpace(message), conn); err != nil {
			LogInfof("Error processing message: %v", err)
			return
		}
	}
}

func (p *P2P) performHandshake(conn net.Conn) error {
	// Set a timeout for the handshake
	if err := conn.SetDeadline(time.Now().Add(p2pHandshakeTimeout)); err != nil {
		// Log error but continue
		_ = err // Suppress unused variable warning
	}
	defer func() {
		if err := conn.SetDeadline(time.Time{}); err != nil {
			// Log error but continue
			_ = err // Suppress unused variable warning
		}
	}() // Reset the deadline

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

	nodeList := make([]NodeInfo, 0, len(p.nodes))
	for _, node := range p.nodes {
		if node == nil || node.Config == nil {
			continue
		}
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

// discoverNodes asks known peers for their peer lists and registers anything new.
//
// It snapshots the peer set under a read lock and then performs network I/O and
// registration *without* holding it. The previous version held RLock across the
// whole loop while calling getSelfNodeID (RLock), IsRegistered (RLock) and
// RegisterNode (Lock): recursive RLock on sync.RWMutex is documented as illegal
// and deadlocks when a writer is queued, and the nested Lock deadlocked outright.
// It also indexed p.nodes[p.getSelfNodeID()] which returns nil when the self node
// is not registered, then dereferenced it.
func (p *P2P) discoverNodes() {
	selfID := p.getSelfNodeID()

	p.mutex.RLock()
	peers := make([]*Node, 0, len(p.nodes))
	for id, node := range p.nodes {
		if id == selfID || node == nil || node.Config == nil {
			continue
		}
		peers = append(peers, node)
	}
	p.mutex.RUnlock()

	for _, node := range peers {
		newNodes, err := p.requestNodeList(node)
		if err != nil {
			LogInfof("Error requesting node list from %s: %v", node.ID, err)
			continue
		}

		for _, newNode := range newNodes {
			if p.IsRegistered(newNode.ID) {
				continue
			}
			if err := p.RegisterNode(newNode); err != nil {
				LogInfof("Error registering new node: %v", err)
			} else {
				LogInfof("Discovered new node: %s", newNode.ID)
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
	LogVerbosef("Validating transaction: %s", tx.ID)
	// TODO: Implement actual validation logic
}

func (p *P2P) updateNodeStatus(tx P2PTransaction) error {
	LogVerbosef("Updating node status: %s", tx.GetID())
	var status NodeStatus
	if err := json.Unmarshal(tx.Data, &status); err != nil {
		return fmt.Errorf("error unmarshaling node status: %w", err)
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	node, exists := p.nodes[status.NodeID]
	if !exists {
		return fmt.Errorf("node %s not found in the network", status.NodeID)
	}

	node.Status = status.Status
	node.LastSeen = time.Now()
	LogVerbosef("Updated status of node %s: %s", status.NodeID, status.Status)
	return nil
}

func (p *P2P) addNode(tx P2PTransaction) error {
	LogVerbosef("Adding new node: %s", tx.GetID())
	newNode, err := decodePeerNode(tx.Data)
	if err != nil {
		return fmt.Errorf("error unmarshaling new node data: %w", err)
	}

	return p.RegisterNode(newNode)
}

// decodePeerNode builds a remote peer from an announcement payload.
//
// Only the fields a peer is allowed to assert about itself are taken. Decoding
// straight into a Node would let a remote host populate Blockchain, Wallet and
// API pointers, and left Config nil -- which every consumer of node.Config
// (sendNodeList, discoverNodes, requestNodeList) then dereferenced.
func decodePeerNode(data []byte) (*Node, error) {
	var info NodeInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, err
	}
	if info.ID == "" {
		return nil, errors.New("peer announcement has no node ID")
	}
	return &Node{
		ID:       info.ID,
		Config:   &Config{P2PHostName: info.Address},
		Status:   "known",
		LastSeen: time.Now(),
	}, nil
}

func (p *P2P) removeNode(tx P2PTransaction) error {
	LogVerbosef("Removing node: %s", tx.GetID())
	var nodeID string
	if err := json.Unmarshal(tx.Data, &nodeID); err != nil {
		return fmt.Errorf("error unmarshaling node ID: %w", err)
	}

	p.mutex.Lock()
	defer p.mutex.Unlock()

	if _, exists := p.nodes[nodeID]; !exists {
		return fmt.Errorf("node %s not found in the network", nodeID)
	}

	delete(p.nodes, nodeID)
	LogVerbosef("Removed node from the network: %s", nodeID)
	return nil
}

func (p *P2P) registerNode(tx P2PTransaction) error {
	LogVerbosef("Registering new node: %s", tx.GetID())
	newNode, err := decodePeerNode(tx.Data)
	if err != nil {
		return fmt.Errorf("error unmarshaling new node data: %w", err)
	}

	if err := p.RegisterNode(newNode); err != nil {
		return err
	}

	// Gossip the new node onward. A broadcast failure is not a registration
	// failure, so it is reported but does not undo the local registration.
	if err := p.BroadcastMessage(P2PTransaction{
		Tx:     tx.Tx,
		Target: "all",
		Action: "add",
		Data:   tx.Data,
	}); err != nil {
		LogVerbosef("Could not gossip new node %s: %v", newNode.ID, err)
	}
	return nil
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

	// This used to be NewTransaction("p2p", node.Wallet, nil), which failed twice
	// over: "P2P" was not in AvailableProtocols, and a nil recipient is rejected.
	// The function therefore returned an error 100% of the time.
	if node.Wallet == nil {
		return errors.New("cannot broadcast status: node has no wallet")
	}

	tx, err := NewTransaction(P2PProtocolID, node.Wallet, node.Wallet)
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
	LogInfof("This node is set as a seed node")
}

func (p *P2P) ConnectToSeedNode(address string) error {
	LogInfof("Connecting to seed node at %s", address)

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
			LogInfof("Error registering node from seed: %v", err)
		} else {
			LogInfof("Added node from seed: %s (%s)", newNode.ID, newNode.Config.P2PHostName)
		}
	}

	return nil
}

func (p *P2P) performClientHandshake(conn net.Conn) error {
	// Set a timeout for the handshake
	if err := conn.SetDeadline(time.Now().Add(p2pHandshakeTimeout)); err != nil {
		// Log error but continue
		_ = err // Suppress unused variable warning
	}
	defer func() {
		if err := conn.SetDeadline(time.Time{}); err != nil {
			// Log error but continue
			_ = err // Suppress unused variable warning
		}
	}() // Reset the deadline

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

	// 3. Send node information.
	// Indexing p.nodes with an unknown ID returns nil, and the old code
	// dereferenced that immediately.
	selfID := p.getSelfNodeID()
	p.mutex.RLock()
	selfNode := p.nodes[selfID]
	p.mutex.RUnlock()
	if selfNode == nil || selfNode.Config == nil {
		return errors.New("cannot handshake: this node is not registered with the P2P network")
	}
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
	if err := conn.SetDeadline(time.Now().Add(p2pHandshakeTimeout)); err != nil {
		// Log error but continue
		_ = err // Suppress unused variable warning
	}
	defer func() {
		if err := conn.SetDeadline(time.Time{}); err != nil {
			// Log error but continue
			_ = err // Suppress unused variable warning
		}
	}() // Reset the deadline

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
	selfAddr := p.getBindAddress()
	for id, node := range p.nodes {
		if node == nil || node.Config == nil {
			continue
		}
		if node.Config.P2PHostName == selfAddr {
			return id
		}
	}
	return ""
}

// max returns the maximum of two integers
// This function is currently unused but kept for potential future use
//
//nolint:unused
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
