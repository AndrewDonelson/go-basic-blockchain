# Section 7 Quiz Answers

## 📋 Answer Key

Here are the correct answers and explanations for the Section 7 quiz.

---

## **Multiple Choice Questions**

### **Question 1: P2P Network Purpose**
**Answer: B) To enable decentralized node communication**

**Explanation**: P2P networking in blockchain enables decentralized communication between nodes without requiring a central authority, which is essential for maintaining the distributed nature of blockchain systems.

### **Question 2: Node Discovery**
**Answer: B) Bootstrap nodes and peer exchange**

**Explanation**: Node discovery in P2P networks typically uses bootstrap nodes (initial known nodes) and peer exchange protocols where nodes share information about other nodes they know.

### **Question 3: Network Synchronization**
**Answer: A) All nodes have the same blockchain state**

**Explanation**: Network synchronization ensures that all nodes in the blockchain network maintain the same state, preventing forks and ensuring consensus across the network.

### **Question 4: Fault Tolerance**
**Answer: B) Other nodes continue operating**

**Explanation**: In a properly designed P2P network, when one node fails, other nodes continue operating normally, demonstrating the network's fault tolerance and resilience.

### **Question 5: Message Routing**
**Answer: B) From node to node directly**

**Explanation**: Messages in P2P networks propagate directly from node to node without going through a central server, maintaining the decentralized nature of the network.

### **Question 6: Peer Management**
**Answer: B) Connection monitoring and health checks**

**Explanation**: Effective peer management requires monitoring connections and performing health checks to ensure network reliability and identify failing nodes.

### **Question 7: Network Topology**
**Answer: B) Mesh topology**

**Explanation**: Blockchain P2P networks typically use mesh topology where nodes connect to multiple other nodes, providing redundancy and fault tolerance.

### **Question 8: Consensus Coordination**
**Answer: B) By broadcasting consensus messages**

**Explanation**: P2P networks coordinate consensus by broadcasting consensus-related messages to all participating nodes, ensuring all nodes can participate in the consensus process.

---

## **True/False Questions**

### **Question 9**
**Answer: True**

**Explanation**: P2P networks are inherently decentralized by design, with no central authority controlling the network.

### **Question 10**
**Answer: False**

**Explanation**: Node discovery can work without a central server using bootstrap nodes and peer exchange protocols.

### **Question 11**
**Answer: True**

**Explanation**: Network synchronization helps prevent forks by ensuring all nodes maintain the same blockchain state.

### **Question 12**
**Answer: False**

**Explanation**: Fault tolerance is crucial in P2P networks to ensure the network continues operating even when individual nodes fail.

### **Question 13**
**Answer: True**

**Explanation**: Message routing can be optimized for efficiency using various algorithms and techniques to reduce latency and improve performance.

### **Question 14**
**Answer: True**

**Explanation**: Peer management includes health monitoring to track the status and performance of connected peers.

---

## **Practical Questions**

### **Question 15: Basic P2P Node**

```go
package p2p

import (
    "encoding/json"
    "fmt"
    "net"
    "sync"
    "time"
)

// P2PNode represents a basic P2P node
type P2PNode struct {
    ID          string
    Address     string
    Port        int
    Peers       map[string]*Peer
    ListenAddr  string
    mu          sync.RWMutex
    stopChan    chan bool
}

// Peer represents a connected peer
type Peer struct {
    ID       string
    Address  string
    Port     int
    Conn     net.Conn
    Status   string
    LastSeen time.Time
}

// NewP2PNode creates a new P2P node
func NewP2PNode(id, address string, port int) *P2PNode {
    return &P2PNode{
        ID:         id,
        Address:    address,
        Port:       port,
        Peers:      make(map[string]*Peer),
        ListenAddr: fmt.Sprintf("%s:%d", address, port),
        stopChan:   make(chan bool),
    }
}

// Start starts the P2P node
func (n *P2PNode) Start() error {
    // Start listening for incoming connections
    listener, err := net.Listen("tcp", n.ListenAddr)
    if err != nil {
        return fmt.Errorf("failed to start listener: %w", err)
    }
    defer listener.Close()
    
    fmt.Printf("🚀 P2P Node %s started on %s\n", n.ID, n.ListenAddr)
    
    // Accept incoming connections
    go n.acceptConnections(listener)
    
    // Start peer health monitoring
    go n.monitorPeers()
    
    return nil
}

// acceptConnections accepts incoming peer connections
func (n *P2PNode) acceptConnections(listener net.Listener) {
    for {
        conn, err := listener.Accept()
        if err != nil {
            fmt.Printf("❌ Failed to accept connection: %v\n", err)
            continue
        }
        
        go n.handleConnection(conn)
    }
}

// handleConnection handles a new peer connection
func (n *P2PNode) handleConnection(conn net.Conn) {
    defer conn.Close()
    
    // Perform handshake
    peer, err := n.performHandshake(conn)
    if err != nil {
        fmt.Printf("❌ Handshake failed: %v\n", err)
        return
    }
    
    // Add peer to network
    n.addPeer(peer)
    
    // Handle peer messages
    n.handlePeerMessages(peer)
}

// performHandshake performs initial handshake with peer
func (n *P2PNode) performHandshake(conn net.Conn) (*Peer, error) {
    // Send handshake message
    handshake := HandshakeMessage{
        NodeID:  n.ID,
        Address: n.Address,
        Port:    n.Port,
        Version: "1.0",
    }
    
    if err := json.NewEncoder(conn).Encode(handshake); err != nil {
        return nil, fmt.Errorf("failed to send handshake: %w", err)
    }
    
    // Receive handshake response
    var response HandshakeMessage
    if err := json.NewDecoder(conn).Decode(&response); err != nil {
        return nil, fmt.Errorf("failed to receive handshake: %w", err)
    }
    
    return &Peer{
        ID:       response.NodeID,
        Address:  response.Address,
        Port:     response.Port,
        Conn:     conn,
        Status:   "connected",
        LastSeen: time.Now(),
    }, nil
}

// addPeer adds a peer to the network
func (n *P2PNode) addPeer(peer *Peer) {
    n.mu.Lock()
    defer n.mu.Unlock()
    
    n.Peers[peer.ID] = peer
    fmt.Printf("✅ Peer %s connected from %s:%d\n", peer.ID, peer.Address, peer.Port)
}

// removePeer removes a peer from the network
func (n *P2PNode) removePeer(peerID string) {
    n.mu.Lock()
    defer n.mu.Unlock()
    
    if peer, exists := n.Peers[peerID]; exists {
        peer.Conn.Close()
        delete(n.Peers, peerID)
        fmt.Printf("❌ Peer %s disconnected\n", peerID)
    }
}

// handlePeerMessages handles messages from a peer
func (n *P2PNode) handlePeerMessages(peer *Peer) {
    decoder := json.NewDecoder(peer.Conn)
    
    for {
        var message Message
        if err := decoder.Decode(&message); err != nil {
            fmt.Printf("❌ Failed to decode message from %s: %v\n", peer.ID, err)
            n.removePeer(peer.ID)
            return
        }
        
        // Update last seen time
        peer.LastSeen = time.Now()
        
        // Process message
        n.processMessage(peer, message)
    }
}

// processMessage processes a message from a peer
func (n *P2PNode) processMessage(peer *Peer, message Message) {
    switch message.Type {
    case "ping":
        n.sendPong(peer)
    case "pong":
        // Handle pong response
    case "block":
        n.handleBlockMessage(peer, message)
    case "transaction":
        n.handleTransactionMessage(peer, message)
    default:
        fmt.Printf("⚠️  Unknown message type: %s\n", message.Type)
    }
}

// sendPong sends a pong response to a peer
func (n *P2PNode) sendPong(peer *Peer) {
    pong := Message{
        Type: "pong",
        Data: map[string]interface{}{
            "timestamp": time.Now().Unix(),
        },
    }
    
    if err := json.NewEncoder(peer.Conn).Encode(pong); err != nil {
        fmt.Printf("❌ Failed to send pong to %s: %v\n", peer.ID, err)
    }
}

// handleBlockMessage handles block messages
func (n *P2PNode) handleBlockMessage(peer *Peer, message Message) {
    fmt.Printf("📦 Received block from %s\n", peer.ID)
    // Process block data
}

// handleTransactionMessage handles transaction messages
func (n *P2PNode) handleTransactionMessage(peer *Peer, message Message) {
    fmt.Printf("💸 Received transaction from %s\n", peer.ID)
    // Process transaction data
}

// monitorPeers monitors peer health
func (n *P2PNode) monitorPeers() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            n.checkPeerHealth()
        case <-n.stopChan:
            return
        }
    }
}

// checkPeerHealth checks the health of all peers
func (n *P2PNode) checkPeerHealth() {
    n.mu.RLock()
    peers := make([]*Peer, 0, len(n.Peers))
    for _, peer := range n.Peers {
        peers = append(peers, peer)
    }
    n.mu.RUnlock()
    
    for _, peer := range peers {
        if time.Since(peer.LastSeen) > 2*time.Minute {
            fmt.Printf("⚠️  Peer %s seems inactive, removing\n", peer.ID)
            n.removePeer(peer.ID)
        }
    }
}

// ConnectToPeer connects to a remote peer
func (n *P2PNode) ConnectToPeer(address string, port int) error {
    conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", address, port))
    if err != nil {
        return fmt.Errorf("failed to connect to peer: %w", err)
    }
    
    go n.handleConnection(conn)
    return nil
}

// BroadcastMessage broadcasts a message to all peers
func (n *P2PNode) BroadcastMessage(messageType string, data map[string]interface{}) {
    message := Message{
        Type: messageType,
        Data: data,
    }
    
    n.mu.RLock()
    peers := make([]*Peer, 0, len(n.Peers))
    for _, peer := range n.Peers {
        peers = append(peers, peer)
    }
    n.mu.RUnlock()
    
    for _, peer := range peers {
        if err := json.NewEncoder(peer.Conn).Encode(message); err != nil {
            fmt.Printf("❌ Failed to broadcast to %s: %v\n", peer.ID, err)
            n.removePeer(peer.ID)
        }
    }
}

// GetPeerCount returns the number of connected peers
func (n *P2PNode) GetPeerCount() int {
    n.mu.RLock()
    defer n.mu.RUnlock()
    return len(n.Peers)
}

// Stop stops the P2P node
func (n *P2PNode) Stop() {
    close(n.stopChan)
    
    n.mu.Lock()
    for _, peer := range n.Peers {
        peer.Conn.Close()
    }
    n.mu.Unlock()
    
    fmt.Printf("🛑 P2P Node %s stopped\n", n.ID)
}

// Message represents a P2P message
type Message struct {
    Type string                 `json:"type"`
    Data map[string]interface{} `json:"data"`
}

// HandshakeMessage represents a handshake message
type HandshakeMessage struct {
    NodeID  string `json:"node_id"`
    Address string `json:"address"`
    Port    int    `json:"port"`
    Version string `json:"version"`
}
```

### **Question 16: Node Discovery**

```go
// NodeDiscovery represents node discovery functionality
type NodeDiscovery struct {
    BootstrapNodes []string
    KnownNodes     map[string]*NodeInfo
    mu             sync.RWMutex
}

// NodeInfo represents information about a known node
type NodeInfo struct {
    ID       string
    Address  string
    Port     int
    LastSeen time.Time
    Status   string
}

// NewNodeDiscovery creates a new node discovery system
func NewNodeDiscovery(bootstrapNodes []string) *NodeDiscovery {
    return &NodeDiscovery{
        BootstrapNodes: bootstrapNodes,
        KnownNodes:     make(map[string]*NodeInfo),
    }
}

// DiscoverNodes discovers new nodes in the network
func (nd *NodeDiscovery) DiscoverNodes() error {
    // Try to connect to bootstrap nodes
    for _, bootstrapAddr := range nd.BootstrapNodes {
        if err := nd.connectToBootstrap(bootstrapAddr); err != nil {
            fmt.Printf("⚠️  Failed to connect to bootstrap node %s: %v\n", bootstrapAddr, err)
        }
    }
    
    // Exchange peer lists with known nodes
    nd.exchangePeerLists()
    
    return nil
}

// connectToBootstrap connects to a bootstrap node
func (nd *NodeDiscovery) connectToBootstrap(address string) error {
    conn, err := net.Dial("tcp", address)
    if err != nil {
        return err
    }
    defer conn.Close()
    
    // Perform handshake
    handshake := HandshakeMessage{
        NodeID:  "discovery",
        Address: "localhost",
        Port:    0,
        Version: "1.0",
    }
    
    if err := json.NewEncoder(conn).Encode(handshake); err != nil {
        return err
    }
    
    // Request peer list
    request := Message{
        Type: "get_peers",
        Data: map[string]interface{}{},
    }
    
    if err := json.NewEncoder(conn).Encode(request); err != nil {
        return err
    }
    
    // Receive peer list
    var response Message
    if err := json.NewDecoder(conn).Decode(&response); err != nil {
        return err
    }
    
    if response.Type == "peer_list" {
        nd.processPeerList(response.Data)
    }
    
    return nil
}

// processPeerList processes a received peer list
func (nd *NodeDiscovery) processPeerList(data map[string]interface{}) {
    if peersData, ok := data["peers"].([]interface{}); ok {
        for _, peerData := range peersData {
            if peerMap, ok := peerData.(map[string]interface{}); ok {
                nodeInfo := &NodeInfo{
                    ID:       peerMap["id"].(string),
                    Address:  peerMap["address"].(string),
                    Port:     int(peerMap["port"].(float64)),
                    LastSeen: time.Now(),
                    Status:   "discovered",
                }
                
                nd.addKnownNode(nodeInfo)
            }
        }
    }
}

// addKnownNode adds a node to the known nodes list
func (nd *NodeDiscovery) addKnownNode(nodeInfo *NodeInfo) {
    nd.mu.Lock()
    defer nd.mu.Unlock()
    
    nd.KnownNodes[nodeInfo.ID] = nodeInfo
    fmt.Printf("🔍 Discovered node: %s (%s:%d)\n", nodeInfo.ID, nodeInfo.Address, nodeInfo.Port)
}

// exchangePeerLists exchanges peer lists with known nodes
func (nd *NodeDiscovery) exchangePeerLists() {
    nd.mu.RLock()
    nodes := make([]*NodeInfo, 0, len(nd.KnownNodes))
    for _, node := range nd.KnownNodes {
        nodes = append(nodes, node)
    }
    nd.mu.RUnlock()
    
    for _, node := range nodes {
        go nd.exchangeWithNode(node)
    }
}

// exchangeWithNode exchanges peer list with a specific node
func (nd *NodeDiscovery) exchangeWithNode(node *NodeInfo) {
    conn, err := net.Dial("tcp", fmt.Sprintf("%s:%d", node.Address, node.Port))
    if err != nil {
        return
    }
    defer conn.Close()
    
    // Send our peer list
    nd.sendPeerList(conn)
    
    // Request their peer list
    request := Message{
        Type: "get_peers",
        Data: map[string]interface{}{},
    }
    
    if err := json.NewEncoder(conn).Encode(request); err != nil {
        return
    }
    
    // Receive their peer list
    var response Message
    if err := json.NewDecoder(conn).Decode(&response); err != nil {
        return
    }
    
    if response.Type == "peer_list" {
        nd.processPeerList(response.Data)
    }
}

// sendPeerList sends our peer list to a node
func (nd *NodeDiscovery) sendPeerList(conn net.Conn) {
    nd.mu.RLock()
    peers := make([]map[string]interface{}, 0, len(nd.KnownNodes))
    for _, node := range nd.KnownNodes {
        peers = append(peers, map[string]interface{}{
            "id":      node.ID,
            "address": node.Address,
            "port":    node.Port,
        })
    }
    nd.mu.RUnlock()
    
    response := Message{
        Type: "peer_list",
        Data: map[string]interface{}{
            "peers": peers,
        },
    }
    
    json.NewEncoder(conn).Encode(response)
}

// GetKnownNodes returns all known nodes
func (nd *NodeDiscovery) GetKnownNodes() []*NodeInfo {
    nd.mu.RLock()
    defer nd.mu.RUnlock()
    
    nodes := make([]*NodeInfo, 0, len(nd.KnownNodes))
    for _, node := range nd.KnownNodes {
        nodes = append(nodes, node)
    }
    
    return nodes
}
```

### **Question 17: Message Routing**

```go
// MessageRouter represents message routing functionality
type MessageRouter struct {
    Routes     map[string]MessageHandler
    Middleware []Middleware
    mu         sync.RWMutex
}

// MessageHandler represents a message handler function
type MessageHandler func(*Peer, Message) error

// Middleware represents middleware function
type Middleware func(MessageHandler) MessageHandler

// NewMessageRouter creates a new message router
func NewMessageRouter() *MessageRouter {
    return &MessageRouter{
        Routes:     make(map[string]MessageHandler),
        Middleware: make([]Middleware, 0),
    }
}

// RegisterRoute registers a message route
func (mr *MessageRouter) RegisterRoute(messageType string, handler MessageHandler) {
    mr.mu.Lock()
    defer mr.mu.Unlock()
    
    // Apply middleware
    for _, middleware := range mr.Middleware {
        handler = middleware(handler)
    }
    
    mr.Routes[messageType] = handler
}

// AddMiddleware adds middleware to the router
func (mr *MessageRouter) AddMiddleware(middleware Middleware) {
    mr.mu.Lock()
    defer mr.mu.Unlock()
    
    mr.Middleware = append(mr.Middleware, middleware)
}

// RouteMessage routes a message to the appropriate handler
func (mr *MessageRouter) RouteMessage(peer *Peer, message Message) error {
    mr.mu.RLock()
    handler, exists := mr.Routes[message.Type]
    mr.mu.RUnlock()
    
    if !exists {
        return fmt.Errorf("no handler for message type: %s", message.Type)
    }
    
    return handler(peer, message)
}

// BroadcastRouter represents broadcast routing
type BroadcastRouter struct {
    Node       *P2PNode
    TTL        int
    SeenMsgs   map[string]bool
    mu         sync.RWMutex
}

// NewBroadcastRouter creates a new broadcast router
func NewBroadcastRouter(node *P2PNode) *BroadcastRouter {
    return &BroadcastRouter{
        Node:     node,
        TTL:      10,
        SeenMsgs: make(map[string]bool),
    }
}

// BroadcastMessage broadcasts a message to the network
func (br *BroadcastRouter) BroadcastMessage(messageType string, data map[string]interface{}) {
    message := Message{
        Type: messageType,
        Data: data,
    }
    
    // Add message ID for deduplication
    messageID := br.generateMessageID(message)
    message.Data["message_id"] = messageID
    message.Data["ttl"] = br.TTL
    message.Data["origin"] = br.Node.ID
    
    // Mark as seen
    br.markMessageSeen(messageID)
    
    // Broadcast to all peers
    br.Node.BroadcastMessage(messageType, message.Data)
}

// generateMessageID generates a unique message ID
func (br *BroadcastRouter) generateMessageID(message Message) string {
    data := fmt.Sprintf("%s%v", message.Type, message.Data)
    hash := sha256.Sum256([]byte(data))
    return hex.EncodeToString(hash[:])
}

// markMessageSeen marks a message as seen
func (br *BroadcastRouter) markMessageSeen(messageID string) {
    br.mu.Lock()
    defer br.mu.Unlock()
    
    br.SeenMsgs[messageID] = true
}

// isMessageSeen checks if a message has been seen
func (br *BroadcastRouter) isMessageSeen(messageID string) bool {
    br.mu.RLock()
    defer br.mu.RUnlock()
    
    return br.SeenMsgs[messageID]
}

// HandleBroadcastMessage handles a broadcast message
func (br *BroadcastRouter) HandleBroadcastMessage(peer *Peer, message Message) error {
    messageID := message.Data["message_id"].(string)
    ttl := int(message.Data["ttl"].(float64))
    origin := message.Data["origin"].(string)
    
    // Check if we've seen this message
    if br.isMessageSeen(messageID) {
        return nil // Already processed
    }
    
    // Mark as seen
    br.markMessageSeen(messageID)
    
    // Process the message
    fmt.Printf("📡 Received broadcast from %s: %s\n", origin, message.Type)
    
    // Forward if TTL > 0 and not from us
    if ttl > 0 && origin != br.Node.ID {
        message.Data["ttl"] = ttl - 1
        br.Node.BroadcastMessage(message.Type, message.Data)
    }
    
    return nil
}
```

### **Question 18: Network Sync**

```go
// NetworkSync represents blockchain network synchronization
type NetworkSync struct {
    Node       *P2PNode
    Blockchain *Blockchain
    SyncStatus string
    mu         sync.RWMutex
}

// NewNetworkSync creates a new network sync
func NewNetworkSync(node *P2PNode, blockchain *Blockchain) *NetworkSync {
    return &NetworkSync{
        Node:       node,
        Blockchain: blockchain,
        SyncStatus: "syncing",
    }
}

// StartSync starts blockchain synchronization
func (ns *NetworkSync) StartSync() {
    go ns.syncLoop()
}

// syncLoop runs the main sync loop
func (ns *NetworkSync) syncLoop() {
    ticker := time.NewTicker(10 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            ns.performSync()
        }
    }
}

// performSync performs blockchain synchronization
func (ns *NetworkSync) performSync() {
    // Get current blockchain height
    currentHeight := ns.Blockchain.GetCurrentHeight()
    
    // Request blockchain info from peers
    ns.requestBlockchainInfo(currentHeight)
    
    // Check for new blocks
    ns.checkForNewBlocks()
    
    // Update sync status
    ns.updateSyncStatus()
}

// requestBlockchainInfo requests blockchain information from peers
func (ns *NetworkSync) requestBlockchainInfo(currentHeight int) {
    request := Message{
        Type: "get_blockchain_info",
        Data: map[string]interface{}{
            "current_height": currentHeight,
        },
    }
    
    ns.Node.BroadcastMessage(request.Type, request.Data)
}

// checkForNewBlocks checks for new blocks from peers
func (ns *NetworkSync) checkForNewBlocks() {
    request := Message{
        Type: "get_latest_blocks",
        Data: map[string]interface{}{
            "count": 10,
        },
    }
    
    ns.Node.BroadcastMessage(request.Type, request.Data)
}

// updateSyncStatus updates the synchronization status
func (ns *NetworkSync) updateSyncStatus() {
    ns.mu.Lock()
    defer ns.mu.Unlock()
    
    peerCount := ns.Node.GetPeerCount()
    if peerCount > 0 {
        ns.SyncStatus = "synced"
    } else {
        ns.SyncStatus = "syncing"
    }
}

// HandleBlockchainInfo handles blockchain info responses
func (ns *NetworkSync) HandleBlockchainInfo(peer *Peer, message Message) error {
    peerHeight := int(message.Data["height"].(float64))
    currentHeight := ns.Blockchain.GetCurrentHeight()
    
    if peerHeight > currentHeight {
        // Request missing blocks
        ns.requestMissingBlocks(currentHeight+1, peerHeight)
    }
    
    return nil
}

// requestMissingBlocks requests missing blocks from a peer
func (ns *NetworkSync) requestMissingBlocks(fromHeight, toHeight int) {
    request := Message{
        Type: "get_blocks",
        Data: map[string]interface{}{
            "from_height": fromHeight,
            "to_height":   toHeight,
        },
    }
    
    ns.Node.BroadcastMessage(request.Type, request.Data)
}

// HandleBlocks handles received blocks
func (ns *NetworkSync) HandleBlocks(peer *Peer, message Message) error {
    blocksData := message.Data["blocks"].([]interface{})
    
    for _, blockData := range blocksData {
        blockMap := blockData.(map[string]interface{})
        
        // Convert to block structure
        block := ns.convertToBlock(blockMap)
        
        // Add block to blockchain
        if err := ns.Blockchain.AddBlock(block); err != nil {
            fmt.Printf("❌ Failed to add block: %v\n", err)
        } else {
            fmt.Printf("✅ Added block %d from %s\n", block.Index, peer.ID)
        }
    }
    
    return nil
}

// convertToBlock converts map data to block structure
func (ns *NetworkSync) convertToBlock(blockMap map[string]interface{}) *Block {
    // Implementation depends on your Block structure
    // This is a simplified example
    return &Block{
        Index: int(blockMap["index"].(float64)),
        Hash:  blockMap["hash"].(string),
        // Add other fields as needed
    }
}

// GetSyncStatus returns the current sync status
func (ns *NetworkSync) GetSyncStatus() string {
    ns.mu.RLock()
    defer ns.mu.RUnlock()
    return ns.SyncStatus
}
```

---

## **Bonus Challenge**

### **Question 19: Complete P2P Network**

```go
// CompleteP2PNetwork represents a complete P2P network system
type CompleteP2PNetwork struct {
    Node        *P2PNode
    Discovery   *NodeDiscovery
    Router      *MessageRouter
    Broadcast   *BroadcastRouter
    Sync        *NetworkSync
    Monitor     *NetworkMonitor
    Security    *NetworkSecurity
    mu          sync.RWMutex
}

// NetworkMonitor monitors network performance
type NetworkMonitor struct {
    Metrics map[string]float64
    Alerts  []Alert
}

// NetworkSecurity handles network security
type NetworkSecurity struct {
    Blacklist map[string]bool
    Whitelist map[string]bool
}

// NewCompleteP2PNetwork creates a complete P2P network
func NewCompleteP2PNetwork(nodeID, address string, port int, bootstrapNodes []string, blockchain *Blockchain) *CompleteP2PNetwork {
    node := NewP2PNode(nodeID, address, port)
    discovery := NewNodeDiscovery(bootstrapNodes)
    router := NewMessageRouter()
    broadcast := NewBroadcastRouter(node)
    sync := NewNetworkSync(node, blockchain)
    monitor := &NetworkMonitor{
        Metrics: make(map[string]float64),
        Alerts:  make([]Alert, 0),
    }
    security := &NetworkSecurity{
        Blacklist: make(map[string]bool),
        Whitelist: make(map[string]bool),
    }
    
    network := &CompleteP2PNetwork{
        Node:      node,
        Discovery: discovery,
        Router:    router,
        Broadcast: broadcast,
        Sync:      sync,
        Monitor:   monitor,
        Security:  security,
    }
    
    // Setup message routes
    network.setupRoutes()
    
    return network
}

// setupRoutes sets up message routing
func (cpn *CompleteP2PNetwork) setupRoutes() {
    cpn.Router.RegisterRoute("ping", cpn.handlePing)
    cpn.Router.RegisterRoute("pong", cpn.handlePong)
    cpn.Router.RegisterRoute("block", cpn.handleBlock)
    cpn.Router.RegisterRoute("transaction", cpn.handleTransaction)
    cpn.Router.RegisterRoute("get_peers", cpn.handleGetPeers)
    cpn.Router.RegisterRoute("peer_list", cpn.handlePeerList)
    cpn.Router.RegisterRoute("get_blockchain_info", cpn.handleGetBlockchainInfo)
    cpn.Router.RegisterRoute("get_blocks", cpn.handleGetBlocks)
    cpn.Router.RegisterRoute("get_latest_blocks", cpn.handleGetLatestBlocks)
}

// Start starts the complete P2P network
func (cpn *CompleteP2PNetwork) Start() error {
    // Start the P2P node
    if err := cpn.Node.Start(); err != nil {
        return err
    }
    
    // Start node discovery
    go cpn.startDiscovery()
    
    // Start network sync
    cpn.Sync.StartSync()
    
    // Start monitoring
    go cpn.startMonitoring()
    
    fmt.Println("🚀 Complete P2P Network started successfully!")
    return nil
}

// startDiscovery starts node discovery
func (cpn *CompleteP2PNetwork) startDiscovery() {
    ticker := time.NewTicker(60 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            cpn.Discovery.DiscoverNodes()
        }
    }
}

// startMonitoring starts network monitoring
func (cpn *CompleteP2PNetwork) startMonitoring() {
    ticker := time.NewTicker(30 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            cpn.updateMetrics()
            cpn.checkAlerts()
        }
    }
}

// updateMetrics updates network metrics
func (cpn *CompleteP2PNetwork) updateMetrics() {
    cpn.mu.Lock()
    defer cpn.mu.Unlock()
    
    cpn.Monitor.Metrics["peer_count"] = float64(cpn.Node.GetPeerCount())
    cpn.Monitor.Metrics["sync_status"] = 1.0 // 1 if synced, 0 if syncing
    cpn.Monitor.Metrics["uptime"] = time.Since(time.Now()).Seconds()
}

// checkAlerts checks for network alerts
func (cpn *CompleteP2PNetwork) checkAlerts() {
    peerCount := cpn.Node.GetPeerCount()
    if peerCount == 0 {
        alert := Alert{
            Type:    "warning",
            Message: "No peers connected",
            Time:    time.Now(),
        }
        cpn.Monitor.Alerts = append(cpn.Monitor.Alerts, alert)
    }
}

// Message handlers
func (cpn *CompleteP2PNetwork) handlePing(peer *Peer, message Message) error {
    return cpn.Node.sendPong(peer)
}

func (cpn *CompleteP2PNetwork) handlePong(peer *Peer, message Message) error {
    // Handle pong response
    return nil
}

func (cpn *CompleteP2PNetwork) handleBlock(peer *Peer, message Message) error {
    return cpn.Sync.HandleBlocks(peer, message)
}

func (cpn *CompleteP2PNetwork) handleTransaction(peer *Peer, message Message) error {
    // Handle transaction
    return nil
}

func (cpn *CompleteP2PNetwork) handleGetPeers(peer *Peer, message Message) error {
    cpn.Discovery.sendPeerList(peer.Conn)
    return nil
}

func (cpn *CompleteP2PNetwork) handlePeerList(peer *Peer, message Message) error {
    cpn.Discovery.processPeerList(message.Data)
    return nil
}

func (cpn *CompleteP2PNetwork) handleGetBlockchainInfo(peer *Peer, message Message) error {
    return cpn.Sync.HandleBlockchainInfo(peer, message)
}

func (cpn *CompleteP2PNetwork) handleGetBlocks(peer *Peer, message Message) error {
    // Handle get blocks request
    return nil
}

func (cpn *CompleteP2PNetwork) handleGetLatestBlocks(peer *Peer, message Message) error {
    // Handle get latest blocks request
    return nil
}

// GetNetworkStatus returns network status
func (cpn *CompleteP2PNetwork) GetNetworkStatus() map[string]interface{} {
    return map[string]interface{}{
        "peer_count":   cpn.Node.GetPeerCount(),
        "sync_status":  cpn.Sync.GetSyncStatus(),
        "known_nodes":  len(cpn.Discovery.GetKnownNodes()),
        "metrics":      cpn.Monitor.Metrics,
        "alerts":       cpn.Monitor.Alerts,
    }
}

// Alert represents a network alert
type Alert struct {
    Type    string    `json:"type"`
    Message string    `json:"message"`
    Time    time.Time `json:"time"`
}
```

---

## **Scoring Your Quiz**

### **How to Calculate Your Score:**

1. **Multiple Choice**: Count correct answers × 2 points each
2. **True/False**: Count correct answers × 1 point each  
3. **Practical Questions**: Rate each answer 0-5 points based on completeness and accuracy
4. **Bonus Challenge**: Rate 0-10 points based on code completeness and functionality

### **Grade Interpretation:**

- **Excellent (90%+)**: 47+ points - You have mastered P2P networking
- **Good (80-89%)**: 42-46 points - You understand the concepts well with minor gaps
- **Satisfactory (70-79%)**: 36-41 points - You have a basic understanding but need more practice
- **Needs Improvement (<70%)**: <36 points - Review the section material and retake the quiz

### **Next Steps Based on Your Score:**

- **90%+**: Excellent! You're ready for Section 8
- **80-89%**: Good work! Review any missed concepts before moving on
- **70-79%**: Spend more time on the hands-on exercises before proceeding
- **<70%**: Review the section material thoroughly and retake the quiz

---

**Great job completing Section 7! 🎉**

Ready for the next challenge? Move on to [Section 8: RESTful API Development](../section8/README.md)!
