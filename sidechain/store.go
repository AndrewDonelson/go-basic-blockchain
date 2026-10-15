// File sidechain/store.go - A sidechain's block space.
package sidechain

import (
	"fmt"
	"sync"
)

// Chain is one game's block space.
//
// SINGLE WRITER BY DESIGN. The publisher's own node produces these blocks; there
// is no consensus here and none is wanted. A publisher does not need to be
// protected from itself, and requiring agreement among strangers to record that
// one of their players unlocked an achievement would buy nothing and cost
// everything.
//
// What the publisher cannot do is rewrite history that has been anchored. The
// main chain is where that guarantee comes from -- see sdk/sidechain_anchor.go.
// Until a range is anchored the publisher can reorder or drop within it, and that
// is the honest description of the trust model rather than a gap in it.
type Chain struct {
	mu sync.RWMutex

	id     ID
	blocks []*Block

	// anchoredHeight is the highest height committed to the main chain, and
	// anchored reports whether any range has been. Height 0 is a real height, so
	// a sentinel value cannot stand in for "nothing anchored yet".
	anchoredHeight uint64
	anchored       bool
}

// NewChain creates an empty block space for an identity.
func NewChain(id ID) (*Chain, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return &Chain{id: id}, nil
}

// ID returns the sidechain's identity.
func (s *Chain) ID() ID { return s.id }

// Height returns the tip height, and whether the chain has any blocks.
func (s *Chain) Height() (uint64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.blocks) == 0 {
		return 0, false
	}
	return s.blocks[len(s.blocks)-1].Header.Height, true
}

// Tip returns the most recent block, or nil.
func (s *Chain) Tip() *Block {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tipLocked()
}

func (s *Chain) tipLocked() *Block {
	if len(s.blocks) == 0 {
		return nil
	}
	return s.blocks[len(s.blocks)-1]
}

// Append adds a block to the chain.
//
// The block must be internally valid, must belong to this sidechain, and must
// follow the current tip. The identity check is the isolation boundary: it is
// what stops one publisher's block being appended to another's space, and it is
// cheap enough that there is no reason to trust the caller instead.
func (s *Chain) Append(block *Block) error {
	if block == nil {
		return fmt.Errorf("%w: block is nil", ErrInvalidSidechainBlock)
	}
	if err := block.Validate(); err != nil {
		return err
	}
	if !block.SidechainID().Equal(s.id) {
		return fmt.Errorf("%w: block belongs to %s, this chain is %s",
			ErrSidechainMismatch, block.SidechainID(), s.id)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := block.Follows(s.tipLocked()); err != nil {
		return err
	}

	s.blocks = append(s.blocks, block)
	return nil
}

// BlockAt returns the block at a height.
func (s *Chain) BlockAt(height uint64) (*Block, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Heights are dense from zero, so the index is the height.
	if height >= uint64(len(s.blocks)) {
		return nil, false
	}
	return s.blocks[height], true
}

// AnchoredHeight returns the highest anchored height, and whether any exists.
func (s *Chain) AnchoredHeight() (uint64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.anchoredHeight, s.anchored
}

// PendingAnchor describes the range not yet committed to the main chain.
//
// It reports whether there is anything to anchor, so a caller does not have to
// reason about the height-zero-is-a-real-height case itself.
func (s *Chain) PendingAnchor() (from, to uint64, ok bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	tip := s.tipLocked()
	if tip == nil {
		return 0, 0, false
	}

	if !s.anchored {
		return 0, tip.Header.Height, true
	}
	if tip.Header.Height <= s.anchoredHeight {
		return 0, 0, false
	}
	return s.anchoredHeight + 1, tip.Header.Height, true
}

// MarkAnchored records that the chain is committed to the main chain up to and
// including height.
//
// AnchorSidechain does this itself when it records against a local ledger. An
// application that anchors through the network instead -- building the anchor,
// submitting an ANCHOR transaction, and waiting for it to be mined -- calls this
// once the anchor has confirmed, so PendingAnchor starts the next range in the
// right place.
//
// The watermark only moves forward, and only onto a block the chain holds.
// Moving it backwards would make the next anchor overlap one already on chain,
// and consensus refuses overlaps; moving it past the tip would claim history is
// committed that does not exist yet. Repeating the current height is a no-op,
// so a confirmation handled twice is harmless.
func (s *Chain) MarkAnchored(height uint64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tip := s.tipLocked()
	if tip == nil || height > tip.Header.Height {
		return fmt.Errorf("%w: cannot mark height %d anchored; the chain does not reach it",
			ErrAnchorNotContiguous, height)
	}
	if s.anchored && height < s.anchoredHeight {
		return fmt.Errorf("%w: height %d is below the anchored height %d",
			ErrAnchorNotContiguous, height, s.anchoredHeight)
	}

	s.anchoredHeight = height
	s.anchored = true
	return nil
}

// markAnchored records that a range has been committed to the main chain.
func (s *Chain) AdvanceAnchored(height uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.anchored && height <= s.anchoredHeight {
		return
	}
	s.anchoredHeight = height
	s.anchored = true
}

// Len returns the number of blocks held.
func (s *Chain) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.blocks)
}

// Restore rebuilds a sidechain from blocks an application persisted
// itself.
//
// Nothing read back from storage is trusted. Every block goes through Append, so
// each one is re-validated -- its identity, its payload root against its own
// payloads, its hash against its own header -- and each must follow the one
// before it by height and hash. The first block must be height 0: a chain
// restored from the middle would have no way to show its first block follows
// anything, and an anchor built from it would fail contiguity on chain.
//
// anchoredThrough is the highest height the application knows to be anchored,
// or nil when nothing has been. It must name a height the restored chain holds.
//
// A chain that does not check out is refused whole. Handing back the valid
// prefix would quietly lose the rest, and the application -- which is the only
// party holding this data -- is the one that needs to know.
func Restore(id ID, blocks []*Block, anchoredThrough *uint64) (*Chain, error) {
	chain, err := NewChain(id)
	if err != nil {
		return nil, err
	}

	for i, block := range blocks {
		if err := chain.Append(block); err != nil {
			return nil, fmt.Errorf("restore %s: block %d: %w", id, i, err)
		}
	}

	if anchoredThrough != nil {
		if err := chain.MarkAnchored(*anchoredThrough); err != nil {
			return nil, fmt.Errorf("restore %s: %w", id, err)
		}
	}
	return chain, nil
}

// Set holds the block spaces a node is serving.
//
// A publisher's node carries its own games' chains; it has no reason to hold
// anyone else's, which is the point of the design -- the main chain sees anchors,
// not game traffic, so it scales with the number of publishers rather than with
// the number of players.
type Set struct {
	mu     sync.RWMutex
	chains map[string]*Chain
}

// NewSet creates an empty set.
func NewSet() *Set {
	return &Set{chains: map[string]*Chain{}}
}

// GetOrCreate returns the block space for an identity, creating it if needed.
func (set *Set) GetOrCreate(id ID) (*Chain, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}

	key := id.String()

	set.mu.Lock()
	defer set.mu.Unlock()

	if chain, ok := set.chains[key]; ok {
		return chain, nil
	}

	chain, err := NewChain(id)
	if err != nil {
		return nil, err
	}
	set.chains[key] = chain
	return chain, nil
}

// Get returns an existing block space.
func (set *Set) Get(id ID) (*Chain, bool) {
	set.mu.RLock()
	defer set.mu.RUnlock()
	chain, ok := set.chains[id.String()]
	return chain, ok
}

// IDs returns the identities held.
func (set *Set) IDs() []ID {
	set.mu.RLock()
	defer set.mu.RUnlock()

	out := make([]ID, 0, len(set.chains))
	for _, chain := range set.chains {
		out = append(out, chain.ID())
	}
	return out
}
