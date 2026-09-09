// Package sdk is a software development kit for building blockchain applications.
// File sdk/p2p_sync.go - the TCP transport for chain synchronisation
//
// Wire protocol. Every exchange is one newline-terminated request and one
// newline-terminated JSON response, over a connection that has completed the
// HELLO/ACK/node-info/OK handshake:
//
//	GET_NODES                    -> [ {id, address}, ... ]
//	GET_STATUS                   -> {node_id, height, head_hash, genesis_hash}
//	GET_BLOCKS <start> <count>   -> [ block, ... ]
//	ANNOUNCE_BLOCK <json block>  -> {"accepted": bool, "reason": string}
//	ANNOUNCE_TX <json tx>        -> {"accepted": bool, "reason": string}
package sdk

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"strconv"
	"strings"
	"time"
)

// Sync protocol commands.
const (
	cmdGetNodes       = "GET_NODES"
	cmdGetStatus      = "GET_STATUS"
	cmdGetBlocks      = "GET_BLOCKS"
	cmdAnnounceBlock  = "ANNOUNCE_BLOCK"
	cmdAnnounceTx     = "ANNOUNCE_TX"
	p2pDialTimeout    = 10 * time.Second
	p2pRequestTimeout = 30 * time.Second
)

// announceResult is the reply to an announcement.
type announceResult struct {
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"`
}

// SetChain attaches the local chain so this node can serve sync requests and
// apply what it receives.
func (p *P2P) SetChain(chain *Blockchain) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.chain = chain
}

// getChain returns the attached chain, if any.
func (p *P2P) getChain() *Blockchain {
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	return p.chain
}

// SetSelfInfo records this node's own identity for the handshake.
//
// The client handshake used to look itself up in p.nodes via getSelfNodeID(),
// which returned "" whenever the node had not registered itself -- and the code
// then dereferenced the resulting nil map entry.
func (p *P2P) SetSelfInfo(id, address string) {
	p.mutex.Lock()
	defer p.mutex.Unlock()
	p.selfID = id
	p.selfAddress = address
}

// selfInfo returns this node's identity, falling back to the registered self node.
func (p *P2P) selfInfo() NodeInfo {
	p.mutex.RLock()
	id, address := p.selfID, p.selfAddress
	p.mutex.RUnlock()

	if id != "" {
		return NodeInfo{ID: id, Address: address}
	}

	selfID := p.getSelfNodeID()
	p.mutex.RLock()
	defer p.mutex.RUnlock()
	if node, ok := p.nodes[selfID]; ok && node != nil && node.Config != nil {
		return NodeInfo{ID: node.ID, Address: node.Config.P2PHostName}
	}
	return NodeInfo{}
}

// SyncPeers implements PeerSource: every registered peer except ourselves.
func (p *P2P) SyncPeers() []PeerRef {
	self := p.selfInfo()

	p.mutex.RLock()
	defer p.mutex.RUnlock()

	peers := make([]PeerRef, 0, len(p.nodes))
	for id, node := range p.nodes {
		if node == nil || node.Config == nil {
			continue
		}
		if id == self.ID || node.Config.P2PHostName == self.Address {
			continue
		}
		peers = append(peers, PeerRef{ID: id, Address: node.Config.P2PHostName})
	}
	return peers
}

// -----------------------------------------------------------------------------
// Client side
// -----------------------------------------------------------------------------

// peerConn is an authenticated, encrypted session with a peer.
type peerConn struct {
	conn   net.Conn
	reader *bufio.Reader
	// peerID is the authenticated node ID of the far side. It is derived from the
	// key the peer proved it holds, not from anything the peer merely asserted.
	peerID string
	// raw is the underlying socket, kept so deadlines can still be set.
	raw net.Conn
}

func (pc *peerConn) close() { _ = pc.raw.Close() }

// request sends one command and reads one response line.
func (pc *peerConn) request(command string) (string, error) {
	if err := pc.raw.SetDeadline(time.Now().Add(p2pRequestTimeout)); err != nil {
		return "", fmt.Errorf("set deadline: %w", err)
	}

	if _, err := pc.conn.Write([]byte(command + "\n")); err != nil {
		return "", fmt.Errorf("send %q: %w", command, err)
	}

	response, err := readLimitedLine(pc.reader)
	if err != nil {
		return "", fmt.Errorf("read response to %q: %w", firstWord(command), err)
	}
	return strings.TrimSpace(response), nil
}

// firstWord returns the command verb, so error messages do not echo a whole block.
func firstWord(s string) string {
	if i := strings.IndexByte(s, ' '); i >= 0 {
		return s[:i]
	}
	return s
}

// dialPeer opens a connection, authenticates mutually, and returns an encrypted
// session.
//
// Every client path goes through this. Previously requestNodeList dialled and
// sent GET_NODES immediately while the server's handleConnection expected HELLO
// first, so the handshake failed and the connection was dropped -- peer discovery
// could never have worked. And the handshake it did perform proved nothing: a
// node simply asserted an ID and the peer believed it.
func (p *P2P) dialPeer(address string) (*peerConn, error) {
	if address == "" {
		return nil, errors.New("peer address is empty")
	}

	conn, err := net.DialTimeout("tcp", address, p2pDialTimeout)
	if err != nil {
		return nil, fmt.Errorf("dial %s: %w", address, err)
	}

	secure, peer, err := p.clientHandshakeAuthenticated(conn)
	if err != nil {
		_ = conn.Close()
		return nil, fmt.Errorf("handshake with %s: %w", address, err)
	}

	return &peerConn{
		conn:   secure,
		raw:    conn,
		reader: bufio.NewReader(io.LimitReader(secure, maxP2PMessageSize)),
		peerID: peer.NodeID,
	}, nil
}

// RequestChainStatus asks a peer to describe its chain. Implements PeerTransport.
func (p *P2P) RequestChainStatus(address string) (ChainStatus, error) {
	pc, err := p.dialPeer(address)
	if err != nil {
		return ChainStatus{}, err
	}
	defer pc.close()

	raw, err := pc.request(cmdGetStatus)
	if err != nil {
		return ChainStatus{}, err
	}

	var status ChainStatus
	if err := json.Unmarshal([]byte(raw), &status); err != nil {
		return ChainStatus{}, fmt.Errorf("decode chain status from %s: %w", address, err)
	}
	return status, nil
}

// RequestBlocks downloads up to count blocks starting at startIndex.
// Implements PeerTransport.
func (p *P2P) RequestBlocks(address string, startIndex, count int) ([]*Block, error) {
	if count <= 0 {
		return nil, fmt.Errorf("block count must be positive, got %d", count)
	}
	if count > maxSyncBatchSize {
		count = maxSyncBatchSize
	}

	pc, err := p.dialPeer(address)
	if err != nil {
		return nil, err
	}
	defer pc.close()

	raw, err := pc.request(fmt.Sprintf("%s %d %d", cmdGetBlocks, startIndex, count))
	if err != nil {
		return nil, err
	}

	var blocks []*Block
	if err := json.Unmarshal([]byte(raw), &blocks); err != nil {
		return nil, fmt.Errorf("decode blocks from %s: %w", address, err)
	}
	return blocks, nil
}

// AnnounceBlock pushes a block to every known peer, best-effort.
func (p *P2P) AnnounceBlock(block *Block) {
	if block == nil {
		return
	}

	encoded, err := json.Marshal(block)
	if err != nil {
		LogVerbosef("Failed to encode block %s for announcement: %v", block.Index.String(), err)
		return
	}

	p.announce(cmdAnnounceBlock, encoded, fmt.Sprintf("block %s", block.Index.String()))
}

// AnnounceTransaction pushes a transaction to every known peer, best-effort.
func (p *P2P) AnnounceTransaction(tx Transaction) {
	if tx == nil {
		return
	}

	encoded, err := json.Marshal(tx)
	if err != nil {
		LogVerbosef("Failed to encode transaction %s for announcement: %v", tx.GetID(), err)
		return
	}

	p.announce(cmdAnnounceTx, encoded, fmt.Sprintf("transaction %s", tx.GetID()))
}

// announce sends one command to every peer concurrently.
//
// Delivery failures are logged, never fatal: one unreachable peer must not stop
// delivery to the rest, and must not block block production.
func (p *P2P) announce(command string, payload []byte, description string) {
	peers := p.SyncPeers()
	if len(peers) == 0 {
		return
	}

	for _, peer := range peers {
		go func(peer PeerRef) {
			pc, err := p.dialPeer(peer.Address)
			if err != nil {
				LogVerbosef("Could not announce %s to %s: %v", description, peer.ID, err)
				return
			}
			defer pc.close()

			if _, err := pc.request(command + " " + string(payload)); err != nil {
				LogVerbosef("Announcing %s to %s failed: %v", description, peer.ID, err)
			}
		}(peer)
	}
}

// -----------------------------------------------------------------------------
// Server side
// -----------------------------------------------------------------------------

// handleSyncCommand serves the sync protocol. It returns handled=false when the
// message is not a sync command, so the caller can fall through to the legacy
// P2PTransaction path.
func (p *P2P) handleSyncCommand(message string, conn net.Conn) (handled bool, err error) {
	verb := firstWord(message)
	argument := strings.TrimSpace(strings.TrimPrefix(message, verb))

	switch verb {
	case cmdGetStatus:
		return true, p.serveChainStatus(conn)
	case cmdGetBlocks:
		return true, p.serveBlocks(conn, argument)
	case cmdAnnounceBlock:
		return true, p.serveAnnouncedBlock(conn, argument)
	case cmdAnnounceTx:
		return true, p.serveAnnouncedTransaction(conn, argument)
	default:
		return false, nil
	}
}

// writeJSONLine writes one JSON response terminated by a newline.
func writeJSONLine(conn net.Conn, v interface{}) error {
	payload, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal response: %w", err)
	}
	if _, err := conn.Write(append(payload, '\n')); err != nil {
		return fmt.Errorf("write response: %w", err)
	}
	return nil
}

// serveChainStatus answers GET_STATUS.
func (p *P2P) serveChainStatus(conn net.Conn) error {
	chain := p.getChain()
	if chain == nil {
		// Answer with an empty chain rather than hanging up: a peer with no chain
		// attached is a valid state, and the requester should learn that cheaply.
		return writeJSONLine(conn, ChainStatus{Height: -1, NodeID: p.selfInfo().ID})
	}

	status := chain.ChainStatus()
	if status.NodeID == "" {
		status.NodeID = p.selfInfo().ID
	}
	return writeJSONLine(conn, status)
}

// serveBlocks answers GET_BLOCKS <start> <count>.
func (p *P2P) serveBlocks(conn net.Conn, argument string) error {
	fields := strings.Fields(argument)
	if len(fields) != 2 {
		return fmt.Errorf("malformed %s request: %q", cmdGetBlocks, argument)
	}

	start, err := strconv.Atoi(fields[0])
	if err != nil {
		return fmt.Errorf("malformed start index %q: %w", fields[0], err)
	}
	count, err := strconv.Atoi(fields[1])
	if err != nil {
		return fmt.Errorf("malformed count %q: %w", fields[1], err)
	}

	if start < 0 {
		start = 0
	}
	if count <= 0 {
		return writeJSONLine(conn, []*Block{})
	}
	// Cap what a remote caller can request in one go.
	if count > maxSyncBatchSize {
		count = maxSyncBatchSize
	}

	chain := p.getChain()
	if chain == nil {
		return writeJSONLine(conn, []*Block{})
	}

	return writeJSONLine(conn, chain.GetBlocksFrom(start, count))
}

// serveAnnouncedBlock applies a block pushed by a peer.
func (p *P2P) serveAnnouncedBlock(conn net.Conn, payload string) error {
	chain := p.getChain()
	if chain == nil {
		return writeJSONLine(conn, announceResult{Accepted: false, Reason: "no chain attached"})
	}

	block := &Block{}
	if err := json.Unmarshal([]byte(payload), block); err != nil {
		return writeJSONLine(conn, announceResult{Accepted: false, Reason: "malformed block: " + err.Error()})
	}

	if err := chain.AcceptBlock(block); err != nil {
		// A block that does not extend our head is not necessarily invalid -- we
		// may simply be behind. Reply honestly and let the periodic sync catch up
		// via GET_STATUS/GET_BLOCKS, which fetches the intervening blocks too.
		return writeJSONLine(conn, announceResult{Accepted: false, Reason: err.Error()})
	}

	LogVerbosef("Accepted announced block %s", block.Index.String())
	return writeJSONLine(conn, announceResult{Accepted: true})
}

// serveAnnouncedTransaction applies a transaction pushed by a peer.
func (p *P2P) serveAnnouncedTransaction(conn net.Conn, payload string) error {
	chain := p.getChain()
	if chain == nil {
		return writeJSONLine(conn, announceResult{Accepted: false, Reason: "no chain attached"})
	}

	tx, err := DecodeTransaction([]byte(payload))
	if err != nil {
		return writeJSONLine(conn, announceResult{Accepted: false, Reason: "malformed transaction: " + err.Error()})
	}

	if err := tx.Validate(); err != nil {
		return writeJSONLine(conn, announceResult{Accepted: false, Reason: err.Error()})
	}

	sender := tx.GetSenderWallet()
	if sender == nil {
		return writeJSONLine(conn, announceResult{Accepted: false, Reason: "transaction has no sender wallet"})
	}

	// Verify before accepting. A relayed transaction is attacker-controlled input.
	valid, err := tx.Verify([]byte(sender.PublicPEM()), tx.GetSignature())
	if err != nil || !valid {
		reason := "signature verification failed"
		if err != nil {
			reason = err.Error()
		}
		return writeJSONLine(conn, announceResult{Accepted: false, Reason: reason})
	}

	if chain.HasTransactionID(tx.GetID()) {
		return writeJSONLine(conn, announceResult{Accepted: false, Reason: "already known"})
	}

	// AddTransactionLocal, not AddTransaction: relaying a transaction we were just
	// given would bounce it back and forth between peers forever.
	chain.AddTransactionLocal(tx)

	LogVerbosef("Accepted announced transaction %s", tx.GetID())
	return writeJSONLine(conn, announceResult{Accepted: true})
}
