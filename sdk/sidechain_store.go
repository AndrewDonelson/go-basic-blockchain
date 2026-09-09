// Package sdk is a software development kit for building blockchain applications.
// File sdk/sidechain_store.go - A sidechain's block space.
package sdk

import (
	"fmt"
	"sync"
)

// Sidechain is one game's block space.
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
type Sidechain struct {
	mu sync.RWMutex

	id     SidechainID
	blocks []*SidechainBlock

	// anchoredHeight is the highest height committed to the main chain, and
	// anchored reports whether any range has been. Height 0 is a real height, so
	// a sentinel value cannot stand in for "nothing anchored yet".
	anchoredHeight uint64
	anchored       bool
}

// NewSidechain creates an empty block space for an identity.
func NewSidechain(id SidechainID) (*Sidechain, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	return &Sidechain{id: id}, nil
}

// ID returns the sidechain's identity.
func (s *Sidechain) ID() SidechainID { return s.id }

// Height returns the tip height, and whether the chain has any blocks.
func (s *Sidechain) Height() (uint64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.blocks) == 0 {
		return 0, false
	}
	return s.blocks[len(s.blocks)-1].Header.Height, true
}

// Tip returns the most recent block, or nil.
func (s *Sidechain) Tip() *SidechainBlock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.tipLocked()
}

func (s *Sidechain) tipLocked() *SidechainBlock {
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
func (s *Sidechain) Append(block *SidechainBlock) error {
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
func (s *Sidechain) BlockAt(height uint64) (*SidechainBlock, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Heights are dense from zero, so the index is the height.
	if height >= uint64(len(s.blocks)) {
		return nil, false
	}
	return s.blocks[height], true
}

// AnchoredHeight returns the highest anchored height, and whether any exists.
func (s *Sidechain) AnchoredHeight() (uint64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.anchoredHeight, s.anchored
}

// PendingAnchor describes the range not yet committed to the main chain.
//
// It reports whether there is anything to anchor, so a caller does not have to
// reason about the height-zero-is-a-real-height case itself.
func (s *Sidechain) PendingAnchor() (from, to uint64, ok bool) {
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

// markAnchored records that a range has been committed to the main chain.
func (s *Sidechain) markAnchored(height uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.anchored && height <= s.anchoredHeight {
		return
	}
	s.anchoredHeight = height
	s.anchored = true
}

// Len returns the number of blocks held.
func (s *Sidechain) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.blocks)
}

// SidechainSet holds the block spaces a node is serving.
//
// A publisher's node carries its own games' chains; it has no reason to hold
// anyone else's, which is the point of the design -- the main chain sees anchors,
// not game traffic, so it scales with the number of publishers rather than with
// the number of players.
type SidechainSet struct {
	mu     sync.RWMutex
	chains map[string]*Sidechain
}

// NewSidechainSet creates an empty set.
func NewSidechainSet() *SidechainSet {
	return &SidechainSet{chains: map[string]*Sidechain{}}
}

// GetOrCreate returns the block space for an identity, creating it if needed.
func (set *SidechainSet) GetOrCreate(id SidechainID) (*Sidechain, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}

	key := id.String()

	set.mu.Lock()
	defer set.mu.Unlock()

	if chain, ok := set.chains[key]; ok {
		return chain, nil
	}

	chain, err := NewSidechain(id)
	if err != nil {
		return nil, err
	}
	set.chains[key] = chain
	return chain, nil
}

// Get returns an existing block space.
func (set *SidechainSet) Get(id SidechainID) (*Sidechain, bool) {
	set.mu.RLock()
	defer set.mu.RUnlock()
	chain, ok := set.chains[id.String()]
	return chain, ok
}

// IDs returns the identities held.
func (set *SidechainSet) IDs() []SidechainID {
	set.mu.RLock()
	defer set.mu.RUnlock()

	out := make([]SidechainID, 0, len(set.chains))
	for _, chain := range set.chains {
		out = append(out, chain.ID())
	}
	return out
}
