package sdk

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// Fakes
//
// The Syncer talks to interfaces precisely so the download/apply loop can be
// driven through failure paths that are painful to provoke over a real socket:
// a peer that lies about its height, one that serves a nil block, one that
// disconnects mid-download. The TCP path is covered separately in
// p2p_sync_test.go.
// -----------------------------------------------------------------------------

// fakeChain is an in-memory chain that accepts blocks only in strict sequence.
type fakeChain struct {
	mu          sync.Mutex
	blocks      []*Block
	genesisHash string
	rejectFrom  int // reject any block at or above this index; -1 disables
	rejectErr   error
}

func newFakeChain(height int) *fakeChain {
	fc := &fakeChain{rejectFrom: -1}
	for i := 0; i <= height; i++ {
		fc.blocks = append(fc.blocks, syncTestBlock(i))
	}
	if len(fc.blocks) > 0 {
		fc.genesisHash = fc.blocks[0].Hash
	}
	return fc
}

func (f *fakeChain) Height() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.blocks) - 1
}

func (f *fakeChain) GenesisHash() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.genesisHash
}

func (f *fakeChain) ChainStatus() ChainStatus {
	f.mu.Lock()
	defer f.mu.Unlock()

	status := ChainStatus{NodeID: "fake", Height: len(f.blocks) - 1}
	if len(f.blocks) > 0 {
		status.HeadHash = f.blocks[len(f.blocks)-1].Hash
		status.GenesisHash = f.blocks[0].Hash
	}
	return status
}

func (f *fakeChain) GetBlocksFrom(startIndex, count int) []*Block {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := []*Block{}
	for _, b := range f.blocks {
		if int(b.Index.Int64()) < startIndex {
			continue
		}
		out = append(out, b)
		if len(out) == count {
			break
		}
	}
	return out
}

func (f *fakeChain) AcceptBlock(b *Block) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if b == nil {
		return errors.New("nil block")
	}
	idx := int(b.Index.Int64())
	if f.rejectFrom >= 0 && idx >= f.rejectFrom {
		if f.rejectErr != nil {
			return f.rejectErr
		}
		return fmt.Errorf("rejecting block %d", idx)
	}
	if idx != len(f.blocks) {
		return fmt.Errorf("block %d does not extend head %d", idx, len(f.blocks)-1)
	}
	f.blocks = append(f.blocks, b)
	return nil
}

// syncTestBlock builds a deterministic block at a given index.
func syncTestBlock(index int) *Block {
	b := &Block{}
	b.Index = *big.NewInt(int64(index))
	b.Header.Version = 1
	b.Header.Timestamp = time.Unix(1700000000+int64(index), 0).UTC()
	b.Header.PreviousHash = fmt.Sprintf("hash-%d", index-1)
	b.Hash = fmt.Sprintf("hash-%d", index)
	return b
}

// fakeTransport serves a set of scripted peers.
type fakeTransport struct {
	mu sync.Mutex

	status     map[string]ChainStatus
	blocks     map[string]*fakeChain
	statusErr  map[string]error
	blocksErr  map[string]error
	blockCalls int
	// nilBlockAt injects a nil block into the response from this address.
	nilBlockAt string
	// serveEmpty makes an address advertise a height but serve no blocks.
	serveEmpty string
}

func newFakeTransport() *fakeTransport {
	return &fakeTransport{
		status:    map[string]ChainStatus{},
		blocks:    map[string]*fakeChain{},
		statusErr: map[string]error{},
		blocksErr: map[string]error{},
	}
}

func (t *fakeTransport) addPeer(address string, height int, genesis string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	chain := newFakeChain(height)
	if genesis != "" {
		chain.genesisHash = genesis
	}
	t.blocks[address] = chain
	t.status[address] = ChainStatus{
		NodeID:      address,
		Height:      height,
		HeadHash:    fmt.Sprintf("hash-%d", height),
		GenesisHash: chain.genesisHash,
	}
}

func (t *fakeTransport) RequestChainStatus(address string) (ChainStatus, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	if err := t.statusErr[address]; err != nil {
		return ChainStatus{}, err
	}
	status, ok := t.status[address]
	if !ok {
		return ChainStatus{}, fmt.Errorf("no such peer %s", address)
	}
	return status, nil
}

func (t *fakeTransport) RequestBlocks(address string, startIndex, count int) ([]*Block, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.blockCalls++
	if err := t.blocksErr[address]; err != nil {
		return nil, err
	}
	if address == t.serveEmpty {
		return []*Block{}, nil
	}

	chain, ok := t.blocks[address]
	if !ok {
		return nil, fmt.Errorf("no such peer %s", address)
	}
	out := chain.GetBlocksFrom(startIndex, count)

	if address == t.nilBlockAt && len(out) > 0 {
		out[0] = nil
	}
	return out, nil
}

// fakePeers is a static peer list.
type fakePeers []PeerRef

func (f fakePeers) SyncPeers() []PeerRef { return f }

func newTestSyncer(t *testing.T, chain ChainWriter, transport PeerTransport, peers PeerSource, batch int) *Syncer {
	t.Helper()
	s, err := NewSyncer(SyncerOptions{Chain: chain, Transport: transport, Peers: peers, BatchSize: batch})
	if err != nil {
		t.Fatalf("new syncer: %v", err)
	}
	return s
}

// -----------------------------------------------------------------------------
// Construction
// -----------------------------------------------------------------------------

func TestNewSyncerRequiresDependencies(t *testing.T) {
	chain := newFakeChain(0)
	transport := newFakeTransport()
	peers := fakePeers{}

	cases := []struct {
		name string
		opts SyncerOptions
	}{
		{"no chain", SyncerOptions{Transport: transport, Peers: peers}},
		{"no transport", SyncerOptions{Chain: chain, Peers: peers}},
		{"no peers", SyncerOptions{Chain: chain, Transport: transport}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewSyncer(tc.opts); err == nil {
				t.Fatalf("expected %s to be rejected", tc.name)
			}
		})
	}

	s, err := NewSyncer(SyncerOptions{Chain: chain, Transport: transport, Peers: peers})
	if err != nil {
		t.Fatalf("valid options rejected: %v", err)
	}
	if s.batchSize != defaultSyncBatchSize {
		t.Fatalf("expected the default batch size, got %d", s.batchSize)
	}
}

func TestNewSyncerClampsBatchSize(t *testing.T) {
	chain := newFakeChain(0)
	transport := newFakeTransport()

	for _, in := range []int{-10, 0} {
		s := newTestSyncer(t, chain, transport, fakePeers{}, in)
		if s.batchSize != defaultSyncBatchSize {
			t.Fatalf("batch %d should default, got %d", in, s.batchSize)
		}
	}

	s := newTestSyncer(t, chain, transport, fakePeers{}, maxSyncBatchSize*10)
	if s.batchSize != maxSyncBatchSize {
		t.Fatalf("batch size should be capped at %d, got %d", maxSyncBatchSize, s.batchSize)
	}
}

// -----------------------------------------------------------------------------
// The happy path
// -----------------------------------------------------------------------------

// TestSyncCatchesUpToLongerPeer is the core case: a node three blocks behind
// pulls the missing blocks and reaches the peer's height.
func TestSyncCatchesUpToLongerPeer(t *testing.T) {
	local := newFakeChain(2) // heights 0..2
	transport := newFakeTransport()
	transport.addPeer("peer-a:1", 5, local.GenesisHash())

	s := newTestSyncer(t, local, transport, fakePeers{{ID: "a", Address: "peer-a:1"}}, 10)

	result, err := s.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	if result.BlocksApplied != 3 {
		t.Fatalf("expected 3 blocks applied, got %d", result.BlocksApplied)
	}
	if result.StartHeight != 2 || result.EndHeight != 5 {
		t.Fatalf("expected height 2 -> 5, got %d -> %d", result.StartHeight, result.EndHeight)
	}
	if local.Height() != 5 {
		t.Fatalf("local chain should be at height 5, got %d", local.Height())
	}
	if result.Peer.ID != "a" {
		t.Fatalf("expected to sync from peer a, got %q", result.Peer.ID)
	}
}

// TestSyncIsIdempotentWhenLevel guards against pointless work and, more
// importantly, against an infinite request loop when there is nothing to fetch.
func TestSyncIsIdempotentWhenLevel(t *testing.T) {
	local := newFakeChain(5)
	transport := newFakeTransport()
	transport.addPeer("peer-a:1", 5, local.GenesisHash())

	s := newTestSyncer(t, local, transport, fakePeers{{ID: "a", Address: "peer-a:1"}}, 10)

	for i := 0; i < 3; i++ {
		result, err := s.SyncOnce(context.Background())
		if err != nil {
			t.Fatalf("pass %d: %v", i, err)
		}
		if result.BlocksApplied != 0 {
			t.Fatalf("pass %d applied %d blocks; nothing was missing", i, result.BlocksApplied)
		}
	}

	if transport.blockCalls != 0 {
		t.Fatalf("no block requests should have been made, got %d", transport.blockCalls)
	}
}

// TestSyncBatchesLargeGaps verifies the download is chunked rather than
// requesting the whole chain in one allocation.
func TestSyncBatchesLargeGaps(t *testing.T) {
	local := newFakeChain(0)
	transport := newFakeTransport()
	transport.addPeer("peer-a:1", 25, local.GenesisHash())

	s := newTestSyncer(t, local, transport, fakePeers{{ID: "a", Address: "peer-a:1"}}, 10)

	result, err := s.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("sync: %v", err)
	}

	if result.BlocksApplied != 25 || local.Height() != 25 {
		t.Fatalf("expected to reach height 25, got %d (applied %d)", local.Height(), result.BlocksApplied)
	}
	// 25 blocks at a batch size of 10 is three requests.
	if transport.blockCalls != 3 {
		t.Fatalf("expected 3 batched requests, got %d", transport.blockCalls)
	}
}

// TestSyncPicksTheLongestChain checks peer selection across several candidates.
func TestSyncPicksTheLongestChain(t *testing.T) {
	local := newFakeChain(1)
	genesis := local.GenesisHash()

	transport := newFakeTransport()
	transport.addPeer("short:1", 2, genesis)
	transport.addPeer("longest:1", 9, genesis)
	transport.addPeer("medium:1", 4, genesis)

	peers := fakePeers{
		{ID: "short", Address: "short:1"},
		{ID: "longest", Address: "longest:1"},
		{ID: "medium", Address: "medium:1"},
	}

	s := newTestSyncer(t, local, transport, peers, 100)

	result, err := s.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if result.Peer.ID != "longest" {
		t.Fatalf("expected to sync from the longest chain, got %q", result.Peer.ID)
	}
	if local.Height() != 9 {
		t.Fatalf("expected height 9, got %d", local.Height())
	}
}

// -----------------------------------------------------------------------------
// Failure paths
// -----------------------------------------------------------------------------

// TestSyncSkipsPeersOnADifferentNetwork guards the genesis check. Pulling blocks
// across a genesis boundary would append blocks that can never validate.
func TestSyncSkipsPeersOnADifferentNetwork(t *testing.T) {
	local := newFakeChain(1)

	transport := newFakeTransport()
	transport.addPeer("stranger:1", 50, "a-completely-different-genesis")

	s := newTestSyncer(t, local, transport, fakePeers{{ID: "stranger", Address: "stranger:1"}}, 10)

	result, err := s.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("sync should not fail because of a forked peer: %v", err)
	}
	if result.BlocksApplied != 0 || local.Height() != 1 {
		t.Fatal("blocks were pulled from a peer on a different network")
	}
	if reason := result.Skipped["stranger"]; !strings.Contains(reason, "genesis") {
		t.Fatalf("expected a genesis mismatch reason, got %q", reason)
	}
}

// TestSyncSurvivesAnUnreachablePeer: one dead peer must not prevent syncing from
// a healthy one.
func TestSyncSurvivesAnUnreachablePeer(t *testing.T) {
	local := newFakeChain(0)
	genesis := local.GenesisHash()

	transport := newFakeTransport()
	transport.addPeer("good:1", 4, genesis)
	transport.statusErr["dead:1"] = errors.New("connection refused")

	peers := fakePeers{
		{ID: "dead", Address: "dead:1"},
		{ID: "good", Address: "good:1"},
	}

	s := newTestSyncer(t, local, transport, peers, 10)

	result, err := s.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if local.Height() != 4 {
		t.Fatalf("expected to sync from the healthy peer, height %d", local.Height())
	}
	if reason := result.Skipped["dead"]; !strings.Contains(reason, "connection refused") {
		t.Fatalf("expected the dead peer to be recorded, got %q", reason)
	}
}

// TestSyncStopsWhenAPeerLiesAboutItsHeight guards an infinite request loop: a
// peer advertising height 10 but serving nothing at index 1 would otherwise be
// asked for the same empty range forever.
func TestSyncStopsWhenAPeerLiesAboutItsHeight(t *testing.T) {
	local := newFakeChain(0)
	transport := newFakeTransport()
	transport.addPeer("liar:1", 10, local.GenesisHash())
	transport.serveEmpty = "liar:1"

	s := newTestSyncer(t, local, transport, fakePeers{{ID: "liar", Address: "liar:1"}}, 10)

	done := make(chan SyncResult, 1)
	go func() {
		r, _ := s.SyncOnce(context.Background())
		done <- r
	}()

	select {
	case result := <-done:
		if result.BlocksApplied != 0 {
			t.Fatalf("no blocks should have been applied, got %d", result.BlocksApplied)
		}
		if reason := result.Skipped["liar"]; !strings.Contains(reason, "served no block") {
			t.Fatalf("expected the lie to be recorded, got %q", reason)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("sync looped forever against a peer that advertised blocks it would not serve")
	}
}

// TestSyncStopsOnARejectedBlock: a peer sending a block we cannot apply ends the
// download rather than hammering on.
func TestSyncStopsOnARejectedBlock(t *testing.T) {
	local := newFakeChain(0)
	local.rejectFrom = 3 // accept 1 and 2, reject 3 onward

	transport := newFakeTransport()
	transport.addPeer("peer:1", 6, local.GenesisHash())

	s := newTestSyncer(t, local, transport, fakePeers{{ID: "peer", Address: "peer:1"}}, 10)

	result, err := s.SyncOnce(context.Background())
	if err == nil {
		t.Fatal("expected an error when a block is rejected")
	}
	if result.BlocksApplied != 2 {
		t.Fatalf("expected the 2 valid blocks to be kept, got %d", result.BlocksApplied)
	}
	if local.Height() != 2 {
		t.Fatalf("expected height 2, got %d", local.Height())
	}
	if reason := result.Skipped["peer"]; !strings.Contains(reason, "rejected") {
		t.Fatalf("expected the rejection to be recorded, got %q", reason)
	}
}

// TestSyncRejectsANilBlock guards against a nil dereference from a hostile peer.
func TestSyncRejectsANilBlock(t *testing.T) {
	local := newFakeChain(0)
	transport := newFakeTransport()
	transport.addPeer("hostile:1", 3, local.GenesisHash())
	transport.nilBlockAt = "hostile:1"

	s := newTestSyncer(t, local, transport, fakePeers{{ID: "hostile", Address: "hostile:1"}}, 10)

	// Must return an error, not panic.
	if _, err := s.SyncOnce(context.Background()); err == nil {
		t.Fatal("expected an error for a nil block")
	}
}

// TestSyncHandlesABlockRequestFailureMidDownload covers a peer that disconnects
// after the status exchange.
func TestSyncHandlesABlockRequestFailureMidDownload(t *testing.T) {
	local := newFakeChain(0)
	transport := newFakeTransport()
	transport.addPeer("flaky:1", 5, local.GenesisHash())
	transport.blocksErr["flaky:1"] = errors.New("connection reset")

	s := newTestSyncer(t, local, transport, fakePeers{{ID: "flaky", Address: "flaky:1"}}, 10)

	result, err := s.SyncOnce(context.Background())
	if err == nil {
		t.Fatal("expected an error when the block request fails")
	}
	if result.BlocksApplied != 0 {
		t.Fatalf("no blocks should have been applied, got %d", result.BlocksApplied)
	}
	if reason := result.Skipped["flaky"]; !strings.Contains(reason, "connection reset") {
		t.Fatalf("expected the failure to be recorded, got %q", reason)
	}
}

// TestSyncWithNoPeersIsANoOp covers the empty network.
func TestSyncWithNoPeersIsANoOp(t *testing.T) {
	local := newFakeChain(3)
	s := newTestSyncer(t, local, newFakeTransport(), fakePeers{}, 10)

	result, err := s.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("sync with no peers should succeed: %v", err)
	}
	if result.BlocksApplied != 0 || result.Peer.ID != "" {
		t.Fatal("expected a no-op with no peers")
	}
}

// TestSyncSkipsPeersWithNoAddress covers a peer record with nothing to dial.
func TestSyncSkipsPeersWithNoAddress(t *testing.T) {
	local := newFakeChain(0)
	s := newTestSyncer(t, local, newFakeTransport(), fakePeers{{ID: "ghost", Address: ""}}, 10)

	result, err := s.SyncOnce(context.Background())
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if reason := result.Skipped["ghost"]; !strings.Contains(reason, "no address") {
		t.Fatalf("expected the peer to be skipped, got %q", reason)
	}
}

// TestSyncHonoursContextCancellation guards the shutdown path.
func TestSyncHonoursContextCancellation(t *testing.T) {
	local := newFakeChain(0)
	transport := newFakeTransport()
	transport.addPeer("peer:1", 1000, local.GenesisHash())

	s := newTestSyncer(t, local, transport, fakePeers{{ID: "peer", Address: "peer:1"}}, 1)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancelled before we start

	if _, err := s.SyncOnce(ctx); err == nil && local.Height() > 0 {
		t.Fatal("a cancelled context should stop the download")
	}
}

// TestSyncerRunStopsOnContextCancel guards the ticker loop.
func TestSyncerRunStopsOnContextCancel(t *testing.T) {
	local := newFakeChain(0)
	transport := newFakeTransport()
	transport.addPeer("peer:1", 2, local.GenesisHash())

	s := newTestSyncer(t, local, transport, fakePeers{{ID: "peer", Address: "peer:1"}}, 10)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		s.Run(ctx, 10*time.Millisecond)
	}()

	// Let a few passes happen, then stop.
	time.Sleep(120 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Syncer.Run did not stop on context cancellation")
	}

	if local.Height() != 2 {
		t.Fatalf("expected the run loop to have synced to height 2, got %d", local.Height())
	}
}

// TestSyncStatsAccumulate covers the counters.
func TestSyncStatsAccumulate(t *testing.T) {
	local := newFakeChain(0)
	transport := newFakeTransport()
	transport.addPeer("peer:1", 3, local.GenesisHash())

	s := newTestSyncer(t, local, transport, fakePeers{{ID: "peer", Address: "peer:1"}}, 10)

	if _, err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("sync: %v", err)
	}
	if _, err := s.SyncOnce(context.Background()); err != nil {
		t.Fatalf("sync: %v", err)
	}

	stats := s.Stats()
	if stats.Passes != 2 {
		t.Fatalf("expected 2 passes, got %d", stats.Passes)
	}
	if stats.BlocksApplied != 3 {
		t.Fatalf("expected 3 blocks applied, got %d", stats.BlocksApplied)
	}
	if stats.PeersContacted != 2 {
		t.Fatalf("expected 2 peer contacts, got %d", stats.PeersContacted)
	}
	if stats.LastSyncedAt.IsZero() {
		t.Fatal("expected LastSyncedAt to be set")
	}
}

// TestSyncIsSafeUnderConcurrency runs passes from several goroutines at once.
func TestSyncIsSafeUnderConcurrency(t *testing.T) {
	local := newFakeChain(0)
	transport := newFakeTransport()
	transport.addPeer("peer:1", 20, local.GenesisHash())

	s := newTestSyncer(t, local, transport, fakePeers{{ID: "peer", Address: "peer:1"}}, 5)

	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 5; j++ {
				_, _ = s.SyncOnce(context.Background())
				_ = s.Stats()
			}
		}()
	}
	wg.Wait()

	// Concurrent passes race each other to apply the same blocks; the chain must
	// still end up correct, and never above the peer's height.
	if h := local.Height(); h != 20 {
		t.Fatalf("expected height 20 after concurrent syncing, got %d", h)
	}
}
