// Package sdk is a software development kit for building blockchain applications.
// File sdk/sync.go - chain synchronisation
//
// Before this existed, nodes exchanged peer lists and nothing else: blocks and
// transactions were never propagated, so every node mined its own independent
// chain and "the network" was a set of strangers who knew each other's addresses.
//
// The logic here is deliberately free of I/O. Syncer talks to interfaces, so the
// download/apply loop can be tested exhaustively against fakes -- including the
// failure paths that are painful to provoke over a real socket -- while the TCP
// implementation lives in p2p_sync.go.
package sdk

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"
)

const (
	// defaultSyncBatchSize bounds how many blocks are requested at once.
	defaultSyncBatchSize = 64

	// defaultSyncInterval is how often a node polls its peers for a longer chain.
	defaultSyncInterval = 10 * time.Second

	// maxSyncBatchSize caps what a peer may ask us for in a single request, so a
	// remote caller cannot request the entire chain in one allocation.
	maxSyncBatchSize = 512

	// syncRewindStep is how far back to restart a download when a peer's blocks
	// turn out to belong to a branch we do not have. Fork choice needs the
	// branch's earlier blocks before it can weigh it against our chain.
	syncRewindStep = 16

	// maxSyncRewinds bounds how far back one pass will walk looking for a fork
	// point, so a peer on a wildly different history cannot make us download
	// indefinitely.
	maxSyncRewinds = 8
)

// ErrGenesisMismatch is returned when a peer is on a different network.
var ErrGenesisMismatch = errors.New("peer genesis hash does not match ours")

// ChainStatus summarises a node's chain. It is the first thing peers exchange.
type ChainStatus struct {
	NodeID      string `json:"node_id"`
	Height      int    `json:"height"` // index of the head block; -1 when empty
	HeadHash    string `json:"head_hash"`
	GenesisHash string `json:"genesis_hash"`
}

// PeerRef identifies a peer well enough to talk to it.
type PeerRef struct {
	ID      string
	Address string
}

// ChainReader is the read side of a chain the syncer serves to peers.
type ChainReader interface {
	ChainStatus() ChainStatus
	GetBlocksFrom(startIndex, count int) []*Block
}

// ChainWriter is the write side the syncer applies downloaded blocks through.
type ChainWriter interface {
	AcceptBlock(block *Block) error
	Height() int
	GenesisHash() string
}

// PeerTransport performs sync requests against a remote peer.
type PeerTransport interface {
	RequestChainStatus(address string) (ChainStatus, error)
	RequestBlocks(address string, startIndex, count int) ([]*Block, error)
}

// PeerSource enumerates the peers currently known to this node.
type PeerSource interface {
	SyncPeers() []PeerRef
}

// SyncResult describes the outcome of a single synchronisation pass.
type SyncResult struct {
	// Peer is the peer we synced from, empty when there was nothing to do.
	Peer PeerRef
	// BlocksApplied is how many blocks were added to the local chain.
	BlocksApplied int
	// StartHeight and EndHeight bracket the local chain across the pass.
	StartHeight int
	EndHeight   int
	// Skipped records peers that could not be used, and why. Sync is best-effort:
	// one unreachable or forked peer must not prevent syncing from another.
	Skipped map[string]string
}

// SyncStats is a cumulative counter set, safe to read while a sync is running.
type SyncStats struct {
	Passes         int64
	BlocksApplied  int64
	PeersContacted int64
	Failures       int64
	LastSyncedAt   time.Time
	LastError      string
}

// Syncer pulls blocks from peers with longer chains.
//
// It does not implement fork choice: blocks are applied through ChainWriter,
// which only accepts blocks that extend the current head. A peer on a competing
// fork of equal or greater length is reported as skipped rather than reorganised
// to. That is a deliberate, documented limitation -- see the package notes and
// `_design/`.
type Syncer struct {
	chain     ChainWriter
	transport PeerTransport
	peers     PeerSource

	batchSize int

	mu    sync.RWMutex
	stats SyncStats
}

// SyncerOptions configures a Syncer.
type SyncerOptions struct {
	Chain     ChainWriter
	Transport PeerTransport
	Peers     PeerSource
	BatchSize int
}

// NewSyncer creates a Syncer. It returns an error rather than panicking on a
// missing dependency, because a half-configured syncer would fail silently.
func NewSyncer(opts SyncerOptions) (*Syncer, error) {
	if opts.Chain == nil {
		return nil, errors.New("syncer requires a chain")
	}
	if opts.Transport == nil {
		return nil, errors.New("syncer requires a transport")
	}
	if opts.Peers == nil {
		return nil, errors.New("syncer requires a peer source")
	}

	batch := opts.BatchSize
	if batch <= 0 {
		batch = defaultSyncBatchSize
	}
	if batch > maxSyncBatchSize {
		batch = maxSyncBatchSize
	}

	return &Syncer{
		chain:     opts.Chain,
		transport: opts.Transport,
		peers:     opts.Peers,
		batchSize: batch,
	}, nil
}

// Stats returns a snapshot of the cumulative counters.
func (s *Syncer) Stats() SyncStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stats
}

// Run synchronises on a ticker until ctx is cancelled.
func (s *Syncer) Run(ctx context.Context, interval time.Duration) {
	if ctx == nil {
		ctx = context.Background()
	}
	if interval <= 0 {
		interval = defaultSyncInterval
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			LogVerbosef("Chain sync stopped")
			return
		case <-ticker.C:
			if _, err := s.SyncOnce(ctx); err != nil {
				LogVerbosef("Chain sync pass failed: %v", err)
			}
		}
	}
}

// SyncOnce performs a single synchronisation pass.
//
// It surveys every peer, picks the one with the longest chain that shares our
// genesis block, and downloads from it in batches until we are level or a block
// is rejected. An unreachable or forked peer is recorded in Skipped and does not
// abort the pass.
func (s *Syncer) SyncOnce(ctx context.Context) (SyncResult, error) {
	// A nil context is a caller mistake, but panicking on it inside a library is
	// worse than treating it as an un-cancellable background context.
	if ctx == nil {
		ctx = context.Background()
	}

	result := SyncResult{
		StartHeight: s.chain.Height(),
		Skipped:     map[string]string{},
	}
	result.EndHeight = result.StartHeight

	s.mu.Lock()
	s.stats.Passes++
	s.mu.Unlock()

	peers := s.peers.SyncPeers()
	if len(peers) == 0 {
		return result, nil
	}

	// Deterministic order, so behaviour does not depend on map iteration.
	sort.Slice(peers, func(i, j int) bool { return peers[i].ID < peers[j].ID })

	best, bestStatus, found := s.selectBestPeer(ctx, peers, result.Skipped)
	if !found {
		return result, nil
	}

	result.Peer = best
	applied, err := s.downloadFrom(ctx, best, bestStatus, result.Skipped)
	result.BlocksApplied = applied
	result.EndHeight = s.chain.Height()

	s.mu.Lock()
	s.stats.BlocksApplied += int64(applied)
	s.stats.LastSyncedAt = time.Now()
	if err != nil {
		s.stats.Failures++
		s.stats.LastError = err.Error()
	}
	s.mu.Unlock()

	return result, err
}

// selectBestPeer surveys peers and returns the one with the longest compatible
// chain that is strictly ahead of us.
func (s *Syncer) selectBestPeer(ctx context.Context, peers []PeerRef, skipped map[string]string) (PeerRef, ChainStatus, bool) {
	localHeight := s.chain.Height()
	localGenesis := s.chain.GenesisHash()

	var (
		best       PeerRef
		bestStatus ChainStatus
		found      bool
	)

	for _, peer := range peers {
		if ctx.Err() != nil {
			return PeerRef{}, ChainStatus{}, false
		}
		if peer.Address == "" {
			skipped[peer.ID] = "peer has no address"
			continue
		}

		status, err := s.transport.RequestChainStatus(peer.Address)

		s.mu.Lock()
		s.stats.PeersContacted++
		s.mu.Unlock()

		if err != nil {
			skipped[peer.ID] = fmt.Sprintf("status request failed: %v", err)
			continue
		}

		// Different genesis means a different network. Pulling blocks across that
		// boundary would append blocks that can never validate.
		if localGenesis != "" && status.GenesisHash != "" && status.GenesisHash != localGenesis {
			skipped[peer.ID] = ErrGenesisMismatch.Error()
			continue
		}

		if status.Height <= localHeight {
			continue
		}

		if !found || status.Height > bestStatus.Height {
			best, bestStatus, found = peer, status, true
		}
	}

	return best, bestStatus, found
}

// downloadFrom pulls blocks from one peer until we reach its height.
//
// When a block turns out to be an orphan -- its parent is on a branch we do not
// have -- the download rewinds and requests earlier blocks, so fork choice gets
// the whole branch and can weigh it. Without the rewind a node could never follow
// a reorganisation: it would keep asking from its own height and keep receiving
// blocks whose parents it was missing.
func (s *Syncer) downloadFrom(ctx context.Context, peer PeerRef, status ChainStatus, skipped map[string]string) (int, error) {
	applied := 0
	rewinds := 0
	// nextOverride forces the next request to start at a specific index.
	nextOverride := -1

	for {
		if err := ctx.Err(); err != nil {
			return applied, err
		}

		next := s.chain.Height() + 1
		if nextOverride >= 0 {
			next = nextOverride
			nextOverride = -1
		}
		if next < 0 {
			next = 0
		}
		if next > status.Height {
			return applied, nil // caught up
		}

		count := status.Height - next + 1
		if count > s.batchSize {
			count = s.batchSize
		}

		blocks, err := s.transport.RequestBlocks(peer.Address, next, count)
		if err != nil {
			skipped[peer.ID] = fmt.Sprintf("block request failed: %v", err)
			return applied, fmt.Errorf("requesting blocks %d..%d from %s: %w", next, next+count-1, peer.ID, err)
		}

		if len(blocks) == 0 {
			// The peer advertised a height it will not serve. Stop rather than
			// spinning: without this the loop would request the same empty range
			// forever.
			skipped[peer.ID] = fmt.Sprintf("peer advertised height %d but served no block at %d", status.Height, next)
			return applied, nil
		}

		progressed := false
		for _, block := range blocks {
			if block == nil {
				skipped[peer.ID] = "peer sent a nil block"
				return applied, fmt.Errorf("peer %s sent a nil block", peer.ID)
			}

			err := s.chain.AcceptBlock(block)
			switch {
			case err == nil:
				applied++
				progressed = true

			case errors.Is(err, ErrKnownBlock), errors.Is(err, ErrWeakerBranch):
				// Already have it, or it sits on a branch that does not beat ours.
				// Both are normal while walking back to a fork point.
				progressed = true

			case errors.Is(err, ErrOrphanBlock):
				// The parent is on a branch we do not have. Rewind and fetch the
				// branch's earlier blocks so fork choice can see the whole thing.
				if rewinds >= maxSyncRewinds {
					skipped[peer.ID] = fmt.Sprintf("gave up looking for a fork point after %d rewinds", rewinds)
					return applied, fmt.Errorf("no common ancestor with %s within %d blocks",
						peer.ID, maxSyncRewinds*syncRewindStep)
				}
				rewinds++

				rewindTo := int(block.Index.Int64()) - syncRewindStep
				if rewindTo < 0 {
					rewindTo = 0
				}
				nextOverride = rewindTo
				LogVerbosef("Orphan block %s from %s; rewinding to %d",
					block.Index.String(), peer.ID, rewindTo)
				progressed = true

			default:
				skipped[peer.ID] = fmt.Sprintf("block %s rejected: %v", block.Index.String(), err)
				return applied, fmt.Errorf("applying block %s from %s: %w", block.Index.String(), peer.ID, err)
			}

			if nextOverride >= 0 {
				break // restart the download from the rewind point
			}
		}

		if !progressed && nextOverride < 0 {
			// Nothing was applied and nothing was rewound: without this the loop
			// would request the same range forever.
			skipped[peer.ID] = "made no progress against this peer"
			return applied, nil
		}
	}
}
