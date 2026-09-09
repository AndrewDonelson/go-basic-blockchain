package sdk

import (
	"context"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// Integration tests: two P2P instances talking over real TCP sockets.
//
// sync_test.go covers the download/apply logic against fakes. These cover the
// wire: the handshake, the request/response framing, and the server handlers.
// -----------------------------------------------------------------------------

// freePort reserves an ephemeral port and returns its address.
func freePort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("reserve port: %v", err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

// syncTestChain builds a standalone chain of the given height, with correct hash
// linkage so AcceptBlock will take the blocks.
func syncTestChain(t *testing.T, height int) *Blockchain {
	t.Helper()

	cfg := NewConfig()
	cfg.DataPath = t.TempDir()

	bc := &Blockchain{cfg: cfg, TXLookup: NewTXLookupManager(), Blocks: []*Block{}}

	previousHash := ""
	for i := 0; i <= height; i++ {
		b := NewBlock(nil, previousHash)
		b.Index = *big.NewInt(int64(i))
		b.Hash = b.CalculateHash()
		bc.Blocks = append(bc.Blocks, b)
		previousHash = b.Hash
	}
	if height >= 0 {
		bc.CurrentBlockIndex = height
		bc.NextBlockIndex = height + 1
	}

	// Helios verification is exercised elsewhere; these fixtures are plain blocks.
	bc.useHeliosMining = false
	return bc
}

// newTestP2P builds a P2P instance with a fresh identity.
//
// Every node needs an identity now: peers authenticate against it and the session
// key is derived from it, so a node without one cannot connect at all.
func newTestP2P(t *testing.T, address string) *P2P {
	t.Helper()

	identity, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("create identity: %v", err)
	}

	p := NewP2P()
	p.SetIdentity(identity)
	p.SetSelfInfo(identity.NodeID, address)
	return p
}

// startPeer boots a P2P server on its own port, serving the given chain.
func startPeer(t *testing.T, _ string, chain *Blockchain) (*P2P, string) {
	t.Helper()

	address := freePort(t)

	p := newTestP2P(t, address)
	p.SetChain(chain)
	p.SetAsSeedNode() // seed nodes listen

	// Register self so the address is resolvable, then listen.
	if err := p.RegisterNode(&Node{ID: p.Identity().NodeID, Config: &Config{P2PHostName: address}}); err != nil {
		t.Fatalf("register self: %v", err)
	}
	if err := p.Start(); err != nil {
		t.Fatalf("start p2p: %v", err)
	}
	t.Cleanup(func() { _ = p.Stop() })

	waitForListener(t, address)
	return p, address
}

// waitForListener blocks until the address accepts connections.
func waitForListener(t *testing.T, address string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, 200*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("listener at %s never came up", address)
}

// TestP2PChainStatusOverTheWire covers the handshake plus GET_STATUS.
func TestP2PChainStatusOverTheWire(t *testing.T) {
	server, address := startPeer(t, "server", syncTestChain(t, 4))

	client := newTestP2P(t, "127.0.0.1:0")

	status, err := client.RequestChainStatus(address)
	if err != nil {
		t.Fatalf("request status: %v", err)
	}

	if status.Height != 4 {
		t.Fatalf("expected height 4, got %d", status.Height)
	}
	if status.HeadHash == "" || status.GenesisHash == "" {
		t.Fatalf("expected head and genesis hashes, got %+v", status)
	}
	if status.GenesisHash != server.getChain().GenesisHash() {
		t.Fatal("genesis hash does not match the served chain")
	}
}

// TestP2PFetchBlocksOverTheWire covers GET_BLOCKS and its bounds.
func TestP2PFetchBlocksOverTheWire(t *testing.T) {
	_, address := startPeer(t, "server", syncTestChain(t, 9))

	client := newTestP2P(t, "127.0.0.1:0")

	t.Run("a middle range", func(t *testing.T) {
		blocks, err := client.RequestBlocks(address, 3, 4)
		if err != nil {
			t.Fatalf("request blocks: %v", err)
		}
		if len(blocks) != 4 {
			t.Fatalf("expected 4 blocks, got %d", len(blocks))
		}
		for i, b := range blocks {
			if got := int(b.Index.Int64()); got != 3+i {
				t.Fatalf("block %d has index %d", i, got)
			}
		}
	})

	t.Run("past the end returns empty", func(t *testing.T) {
		blocks, err := client.RequestBlocks(address, 500, 10)
		if err != nil {
			t.Fatalf("request blocks: %v", err)
		}
		if len(blocks) != 0 {
			t.Fatalf("expected no blocks past the end, got %d", len(blocks))
		}
	})

	t.Run("more than available", func(t *testing.T) {
		blocks, err := client.RequestBlocks(address, 0, 1000)
		if err != nil {
			t.Fatalf("request blocks: %v", err)
		}
		if len(blocks) != 10 {
			t.Fatalf("expected the whole 10-block chain, got %d", len(blocks))
		}
	})

	t.Run("a non-positive count is refused locally", func(t *testing.T) {
		if _, err := client.RequestBlocks(address, 0, 0); err == nil {
			t.Fatal("expected an error for a zero count")
		}
	})
}

// TestP2PServerCapsBlockRequests guards against a peer asking for the whole chain
// in one allocation.
func TestP2PServerCapsBlockRequests(t *testing.T) {
	_, address := startPeer(t, "server", syncTestChain(t, 3))

	// Bypass the client, which clamps before sending, to check the server's own cap.
	conn := rawHandshake(t, address, "client")
	defer conn.close()

	raw, err := conn.request(fmt.Sprintf("%s 0 %d", cmdGetBlocks, maxSyncBatchSize*100))
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	var blocks []*Block
	if err := json.Unmarshal([]byte(raw), &blocks); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(blocks) > maxSyncBatchSize {
		t.Fatalf("server served %d blocks, above the %d cap", len(blocks), maxSyncBatchSize)
	}
}

// rawHandshake opens a handshaken connection for tests that need to send
// hand-built commands.
func rawHandshake(t *testing.T, address, _ string) *peerConn {
	t.Helper()
	client := newTestP2P(t, "127.0.0.1:0")

	pc, err := client.dialPeer(address)
	if err != nil {
		t.Fatalf("dial %s: %v", address, err)
	}
	return pc
}

// TestP2PServerRejectsMalformedRequests covers the parsing paths.
func TestP2PServerRejectsMalformedRequests(t *testing.T) {
	_, address := startPeer(t, "server", syncTestChain(t, 2))

	cases := []string{
		cmdGetBlocks,                    // no arguments
		cmdGetBlocks + " notanumber 5",  // bad start
		cmdGetBlocks + " 0 notanumber",  // bad count
		cmdAnnounceBlock + " {not json", // malformed block
		cmdAnnounceTx + " {not json",    // malformed transaction
	}

	for _, request := range cases {
		t.Run(firstWord(request)+"/"+request, func(t *testing.T) {
			conn := rawHandshake(t, address, "client")
			defer conn.close()

			// The server must not panic. It either answers or closes the
			// connection; both are acceptable, a crash is not.
			_, _ = conn.request(request)

			// The server must still be serving afterwards.
			check := rawHandshake(t, address, "client2")
			defer check.close()
			if _, err := check.request(cmdGetStatus); err != nil {
				t.Fatalf("server stopped serving after %q: %v", request, err)
			}
		})
	}
}

// TestP2PRequiresHandshake guards the protocol: a client that skips HELLO must
// not be served.
func TestP2PRequiresHandshake(t *testing.T) {
	_, address := startPeer(t, "server", syncTestChain(t, 1))

	conn, err := net.DialTimeout("tcp", address, 2*time.Second)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer conn.Close()

	// Skip the handshake entirely and issue a command.
	if _, err := conn.Write([]byte(cmdGetStatus + "\n")); err != nil {
		t.Fatalf("write: %v", err)
	}

	// The server should hang up rather than answer.
	_ = conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	buf := make([]byte, 128)
	n, err := conn.Read(buf)
	if err == nil && n > 0 {
		t.Fatalf("server answered a request that skipped the handshake: %q", buf[:n])
	}
}

// TestNodeDiscoveryOverTheWire covers GET_NODES, which never worked: the client
// dialled and sent GET_NODES immediately while the server expected HELLO first.
func TestNodeDiscoveryOverTheWire(t *testing.T) {
	server, address := startPeer(t, "server", syncTestChain(t, 0))

	if err := server.RegisterNode(&Node{ID: "other", Config: &Config{P2PHostName: "10.0.0.9:8101"}}); err != nil {
		t.Fatalf("register peer: %v", err)
	}

	client := newTestP2P(t, "127.0.0.1:0")

	nodes, err := client.requestNodeList(&Node{ID: "server", Config: &Config{P2PHostName: address}})
	if err != nil {
		t.Fatalf("request node list: %v", err)
	}

	if len(nodes) < 2 {
		t.Fatalf("expected at least the server and its peer, got %d", len(nodes))
	}

	found := false
	for _, n := range nodes {
		if n.ID == "other" {
			found = true
		}
	}
	if !found {
		t.Fatal("the server's peer was not in the returned node list")
	}
}

// -----------------------------------------------------------------------------
// End-to-end: two chains converge
// -----------------------------------------------------------------------------

// TestTwoNodesConvergeOverTCP is the headline test: a node five blocks behind
// synchronises to its peer over a real socket.
func TestTwoNodesConvergeOverTCP(t *testing.T) {
	ahead := syncTestChain(t, 7)
	_, aheadAddr := startPeer(t, "ahead", ahead)

	behind := syncTestChain(t, 2)
	// The two chains must share a genesis block for sync to be meaningful, which
	// is exactly what the syncer checks. Rebuild `behind` as a prefix of `ahead`.
	behind.Blocks = append([]*Block{}, ahead.Blocks[:3]...)
	behind.CurrentBlockIndex = 2
	behind.NextBlockIndex = 3

	client := newTestP2P(t, "127.0.0.1:0")
	if err := client.RegisterNode(&Node{ID: "ahead", Config: &Config{P2PHostName: aheadAddr}}); err != nil {
		t.Fatalf("register peer: %v", err)
	}

	syncer, err := NewSyncer(SyncerOptions{Chain: behind, Transport: client, Peers: client, BatchSize: 3})
	if err != nil {
		t.Fatalf("new syncer: %v", err)
	}

	result, err := syncer.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("sync: %v (skipped: %v)", err, result.Skipped)
	}

	if behind.Height() != 7 {
		t.Fatalf("expected the trailing node to reach height 7, got %d (applied %d, skipped %v)",
			behind.Height(), result.BlocksApplied, result.Skipped)
	}
	if result.BlocksApplied != 5 {
		t.Fatalf("expected 5 blocks applied, got %d", result.BlocksApplied)
	}
	if behind.HeadHash() != ahead.HeadHash() {
		t.Fatal("the two chains did not converge on the same head")
	}
}

// TestSyncRefusesAPeerOnADifferentNetworkOverTCP is the genesis check end to end.
func TestSyncRefusesAPeerOnADifferentNetworkOverTCP(t *testing.T) {
	stranger := syncTestChain(t, 20)
	// Give it a genuinely different genesis block.
	stranger.Blocks[0].Header.PreviousHash = "a-different-network"
	stranger.Blocks[0].Hash = stranger.Blocks[0].CalculateHash()
	_, strangerAddr := startPeer(t, "stranger", stranger)

	local := syncTestChain(t, 1)

	client := newTestP2P(t, "127.0.0.1:0")
	if err := client.RegisterNode(&Node{ID: "stranger", Config: &Config{P2PHostName: strangerAddr}}); err != nil {
		t.Fatalf("register peer: %v", err)
	}

	syncer, err := NewSyncer(SyncerOptions{Chain: local, Transport: client, Peers: client})
	if err != nil {
		t.Fatalf("new syncer: %v", err)
	}

	result, err := syncer.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("sync should not error on a forked peer: %v", err)
	}
	if local.Height() != 1 {
		t.Fatalf("blocks were pulled from a different network; height is now %d", local.Height())
	}
	if len(result.Skipped) == 0 {
		t.Fatal("expected the stranger to be recorded as skipped")
	}
}

// TestBlockAnnouncementOverTCP covers push propagation: a mined block reaches a
// peer immediately, without waiting for the next sync tick.
func TestBlockAnnouncementOverTCP(t *testing.T) {
	receiver := syncTestChain(t, 3)
	_, receiverAddr := startPeer(t, "receiver", receiver)

	sender := newTestP2P(t, "127.0.0.1:0")
	if err := sender.RegisterNode(&Node{ID: "receiver", Config: &Config{P2PHostName: receiverAddr}}); err != nil {
		t.Fatalf("register peer: %v", err)
	}

	// Build the block that extends the receiver's head.
	next := NewBlock(nil, receiver.HeadHash())
	next.Index = *big.NewInt(4)
	next.Hash = next.CalculateHash()

	sender.AnnounceBlock(next)

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if receiver.Height() == 4 {
			return
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatalf("announced block was not applied; receiver is still at height %d", receiver.Height())
}

// TestAnnouncedBlockThatDoesNotExtendIsRefused: the receiver must not accept a
// block from the future, and must say so rather than silently ignoring it.
func TestAnnouncedBlockThatDoesNotExtendIsRefused(t *testing.T) {
	receiver := syncTestChain(t, 2)
	_, address := startPeer(t, "receiver", receiver)

	orphan := NewBlock(nil, "unrelated-parent")
	orphan.Index = *big.NewInt(99)
	orphan.Hash = orphan.CalculateHash()

	encoded, err := json.Marshal(orphan)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	conn := rawHandshake(t, address, "sender")
	defer conn.close()

	raw, err := conn.request(cmdAnnounceBlock + " " + string(encoded))
	if err != nil {
		t.Fatalf("announce: %v", err)
	}

	var result announceResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	if result.Accepted {
		t.Fatal("a block that does not extend the head was accepted")
	}
	if result.Reason == "" {
		t.Fatal("expected a reason for the rejection")
	}
	if receiver.Height() != 2 {
		t.Fatalf("receiver height changed to %d", receiver.Height())
	}
}

// TestAnnouncedTransactionIsVerifiedBeforeAcceptance: a relayed transaction is
// attacker-controlled input, so its signature must be checked.
func TestAnnouncedTransactionIsVerifiedBeforeAcceptance(t *testing.T) {
	chain := syncTestChain(t, 0)
	_, address := startPeer(t, "receiver", chain)

	from := newTestWallet(t, "announce-from", 1000)
	to := newTestWallet(t, "announce-to", 0)
	fundWalletForTest(t, chain, from, 1000)

	t.Run("a signed transaction is accepted", func(t *testing.T) {
		tx, err := NewBankTransaction(from, to, 5)
		if err != nil {
			t.Fatalf("create tx: %v", err)
		}
		tx.Signature, err = tx.Sign([]byte(from.PrivatePEM()))
		if err != nil {
			t.Fatalf("sign: %v", err)
		}

		result := announceTx(t, address, tx)
		if !result.Accepted {
			t.Fatalf("a correctly signed transaction was refused: %s", result.Reason)
		}
		if !chain.HasTransactionID(tx.GetID()) {
			t.Fatal("accepted transaction did not reach the mempool")
		}
	})

	t.Run("an unsigned transaction is refused", func(t *testing.T) {
		tx, err := NewBankTransaction(from, to, 6)
		if err != nil {
			t.Fatalf("create tx: %v", err)
		}
		// No signature at all.
		result := announceTx(t, address, tx)
		if result.Accepted {
			t.Fatal("an unsigned transaction was accepted")
		}
	})

	t.Run("a tampered transaction is refused", func(t *testing.T) {
		tx, err := NewBankTransaction(from, to, 7)
		if err != nil {
			t.Fatalf("create tx: %v", err)
		}
		tx.Signature, err = tx.Sign([]byte(from.PrivatePEM()))
		if err != nil {
			t.Fatalf("sign: %v", err)
		}
		tx.Amount = 1000000 // tamper after signing

		result := announceTx(t, address, tx)
		if result.Accepted {
			t.Fatal("a transaction whose amount was altered after signing was accepted")
		}
	})
}

// announceTx pushes one transaction and returns the peer's verdict.
func announceTx(t *testing.T, address string, tx Transaction) announceResult {
	t.Helper()

	encoded, err := json.Marshal(tx)
	if err != nil {
		t.Fatalf("marshal transaction: %v", err)
	}

	conn := rawHandshake(t, address, "sender")
	defer conn.close()

	raw, err := conn.request(cmdAnnounceTx + " " + string(encoded))
	if err != nil {
		t.Fatalf("announce: %v", err)
	}

	var result announceResult
	if err := json.Unmarshal([]byte(raw), &result); err != nil {
		t.Fatalf("decode result: %v", err)
	}
	return result
}

// TestRelayedTransactionIsNotReAnnounced guards the relay loop: a transaction we
// were handed must not be echoed back to the peer that sent it.
func TestRelayedTransactionIsNotReAnnounced(t *testing.T) {
	chain := syncTestChain(t, 0)

	announced := 0
	chain.SetTransactionAnnouncer(func(Transaction) { announced++ })

	from := newTestWallet(t, "relay-from", 100)
	to := newTestWallet(t, "relay-to", 0)
	fundWalletForTest(t, chain, from, 100)
	tx, err := NewBankTransaction(from, to, 1)
	if err != nil {
		t.Fatalf("create tx: %v", err)
	}

	// AddTransactionLocal is the relay path: queue it, do not announce.
	if !chain.AddTransactionLocal(tx) {
		t.Fatal("expected the transaction to be queued")
	}
	if announced != 0 {
		t.Fatalf("a relayed transaction was re-announced %d times", announced)
	}

	// AddTransaction is the local-origin path: queue it and announce.
	tx2, err := NewBankTransaction(from, to, 2)
	if err != nil {
		t.Fatalf("create tx: %v", err)
	}
	chain.AddTransaction(tx2)
	if announced != 1 {
		t.Fatalf("expected a locally originated transaction to be announced once, got %d", announced)
	}
}

// TestAddTransactionLocalDeduplicates guards double-queueing under gossip.
func TestAddTransactionLocalDeduplicates(t *testing.T) {
	chain := syncTestChain(t, 0)

	from := newTestWallet(t, "dedup-from", 100)
	to := newTestWallet(t, "dedup-to", 0)
	fundWalletForTest(t, chain, from, 100)
	tx, err := NewBankTransaction(from, to, 1)
	if err != nil {
		t.Fatalf("create tx: %v", err)
	}

	if !chain.AddTransactionLocal(tx) {
		t.Fatal("first add should succeed")
	}
	if chain.AddTransactionLocal(tx) {
		t.Fatal("the same transaction was queued twice")
	}
	if got := len(chain.GetPendingTransactions()); got != 1 {
		t.Fatalf("expected 1 pending transaction, got %d", got)
	}
}

// TestSyncPeersExcludesSelf guards a node syncing from itself.
func TestSyncPeersExcludesSelf(t *testing.T) {
	p := NewP2P()
	p.SetSelfInfo("me", "127.0.0.1:9999")

	if err := p.RegisterNode(&Node{ID: "me", Config: &Config{P2PHostName: "127.0.0.1:9999"}}); err != nil {
		t.Fatalf("register self: %v", err)
	}
	if err := p.RegisterNode(&Node{ID: "other", Config: &Config{P2PHostName: "127.0.0.1:8888"}}); err != nil {
		t.Fatalf("register peer: %v", err)
	}
	if err := p.RegisterNode(&Node{ID: "broken", Config: nil}); err != nil {
		t.Fatalf("register peer: %v", err)
	}

	peers := p.SyncPeers()
	if len(peers) != 1 || peers[0].ID != "other" {
		t.Fatalf("expected only the other peer, got %+v", peers)
	}
}

// TestChainStatusOnAnEmptyChain covers the boundary.
func TestChainStatusOnAnEmptyChain(t *testing.T) {
	bc := &Blockchain{cfg: NewConfig(), TXLookup: NewTXLookupManager(), Blocks: []*Block{}}

	if got := bc.Height(); got != -1 {
		t.Fatalf("an empty chain should have height -1, got %d", got)
	}
	if bc.HeadHash() != "" || bc.GenesisHash() != "" {
		t.Fatal("an empty chain should have no head or genesis hash")
	}

	status := bc.ChainStatus()
	if status.Height != -1 {
		t.Fatalf("expected height -1, got %d", status.Height)
	}

	if blocks := bc.GetBlocksFrom(0, 10); len(blocks) != 0 {
		t.Fatalf("expected no blocks, got %d", len(blocks))
	}
	if blocks := bc.GetBlocksFrom(0, 0); len(blocks) != 0 {
		t.Fatal("a zero count must return nothing")
	}
}

// TestGetBlocksFromSelectsByIndex guards selection by block index rather than
// slice position.
func TestGetBlocksFromSelectsByIndex(t *testing.T) {
	bc := syncTestChain(t, 9)

	blocks := bc.GetBlocksFrom(7, 5)
	if len(blocks) != 3 {
		t.Fatalf("expected the last 3 blocks, got %d", len(blocks))
	}
	for i, b := range blocks {
		if got := int(b.Index.Int64()); got != 7+i {
			t.Fatalf("expected index %d, got %d", 7+i, got)
		}
	}

	if got := bc.GetBlocksFrom(100, 5); len(got) != 0 {
		t.Fatalf("expected nothing past the end, got %d", len(got))
	}
}
