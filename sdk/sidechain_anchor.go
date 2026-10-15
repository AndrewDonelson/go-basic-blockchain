// Package sdk is a software development kit for building blockchain applications.
// File sdk/sidechain_anchor.go - Committing a sidechain's tip to the main chain.
package sdk

import (
	"fmt"
	"sync"
)

// WHAT AN ANCHOR IS FOR
//
// A sidechain is written by one party -- the publisher whose game it records. On
// its own that is a database with extra steps: the publisher could reorder it,
// drop entries from it, or present two different histories to two different
// people, and nobody could tell.
//
// An anchor is a commitment to the main chain: at main-chain height H, this
// sidechain's history up to height N hashed to exactly this. Because sidechain
// blocks chain by PreviousHash, committing the tip commits the entire prefix --
// changing anything at any earlier height changes every hash after it, and the
// anchor no longer matches.
//
// That converts "trust the publisher" into "the publisher cannot rewrite what
// they have already published", which is a materially different promise and the
// one a player or an auditor actually needs.
//
// # WHAT AN ANCHOR IS NOT
//
// It is not data availability. The anchor proves a history existed; it does not
// make the publisher hand it over. A publisher who loses or withholds the blocks
// leaves everyone able to prove that something was committed and unable to say
// what. That is a real limit and the honest place to state it.
//
// Nor does it constrain the unanchored window. Between anchors the publisher can
// still reorder or drop; anchoring more often narrows that window at the cost of
// more main-chain traffic, which is the dial an operator actually has.
// SignAnchor signs an anchor with a publisher's identity key.
func SignAnchor(identity *PeerIdentity, anchor SidechainAnchor) ([]byte, error) {
	if identity == nil {
		return nil, fmt.Errorf("%w: no publisher identity", ErrAnchorUnauthorised)
	}
	if err := anchor.Validate(); err != nil {
		return nil, err
	}
	return identity.Sign(anchor.SigningBytes())
}

// anchorState is the ledger's record for one sidechain.
type anchorState struct {
	// anchors is every anchor for this sidechain, in order. The history is kept
	// rather than only the tip because an availability proof has to be checked
	// against the anchor that covers the height in question, which is usually
	// not the latest one.
	anchors []SidechainAnchor
	// mainHeights[i] is the main-chain height of the block that recorded
	// anchors[i]. It is kept beside the anchor rather than inside it because
	// it is a fact about where the commitment landed, not part of what the
	// publisher signed -- and it is what lets anyone holding an anchor go and
	// find the mined block that makes it binding.
	mainHeights []int64
	// lastMainHeight is the main-chain height that recorded the latest anchor,
	// which is what a gap is measured against.
	lastMainHeight int64
	// delinquent records that this sidechain has at some point left history
	// uncommitted for longer than MaxAnchorGap.
	delinquent bool
}

// AnchorLedger tracks what has been anchored, per sidechain.
//
// This is the main chain's view. It holds no game data -- only the commitments --
// which is what keeps the main chain's cost proportional to the number of
// publishers rather than to how much their players do.
type AnchorLedger struct {
	mu       sync.RWMutex
	registry *PublisherRegistry
	states   map[string]*anchorState
}

// NewAnchorLedger creates a ledger backed by a publisher registry.
func NewAnchorLedger(registry *PublisherRegistry) *AnchorLedger {
	if registry == nil {
		registry = NewPublisherRegistry()
	}
	return &AnchorLedger{registry: registry, states: map[string]*anchorState{}}
}

// Registry returns the ledger's publisher registry.
func (l *AnchorLedger) Registry() *PublisherRegistry { return l.registry }

// Latest returns the most recent anchor for a sidechain.
func (l *AnchorLedger) Latest(id SidechainID) (SidechainAnchor, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	state, ok := l.states[id.String()]
	if !ok || len(state.anchors) == 0 {
		return SidechainAnchor{}, false
	}
	return state.anchors[len(state.anchors)-1], true
}

// AnchorCovering returns the anchor that commits a given sidechain height.
func (l *AnchorLedger) AnchorCovering(id SidechainID, height uint64) (SidechainAnchor, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	state, ok := l.states[id.String()]
	if !ok {
		return SidechainAnchor{}, false
	}

	// Anchors are contiguous and ordered, so a binary search is exact.
	low, high := 0, len(state.anchors)-1
	for low <= high {
		mid := (low + high) / 2
		switch {
		case height < state.anchors[mid].FromHeight:
			high = mid - 1
		case height > state.anchors[mid].ToHeight:
			low = mid + 1
		default:
			return state.anchors[mid], true
		}
	}
	return SidechainAnchor{}, false
}

// AnchorRecord is one anchor as the main chain recorded it.
type AnchorRecord struct {
	Anchor SidechainAnchor
	// MainHeight is the index of the main-chain block that carries the anchor.
	MainHeight int64
}

// Anchors returns every anchor recorded for a sidechain, oldest first.
//
// The slice is a copy; the ledger keeps changing underneath as blocks are
// applied and rolled back, and a caller iterating its own copy cannot observe
// that half-way.
func (l *AnchorLedger) Anchors(id SidechainID) []AnchorRecord {
	l.mu.RLock()
	defer l.mu.RUnlock()

	state, ok := l.states[id.String()]
	if !ok {
		return nil
	}

	out := make([]AnchorRecord, 0, len(state.anchors))
	for i, anchor := range state.anchors {
		record := AnchorRecord{Anchor: anchor}
		record.Anchor.TipHash = append([]byte{}, anchor.TipHash...)
		record.Anchor.HeaderRoot = append([]byte{}, anchor.HeaderRoot...)
		if i < len(state.mainHeights) {
			record.MainHeight = state.mainHeights[i]
		}
		out = append(out, record)
	}
	return out
}

// IsDelinquent reports whether a sidechain has ever exceeded MaxAnchorGap.
func (l *AnchorLedger) IsDelinquent(id SidechainID) bool {
	l.mu.RLock()
	defer l.mu.RUnlock()

	state, ok := l.states[id.String()]
	return ok && state.delinquent
}

// Verify checks an anchor against the ledger without recording it.
//
// Three questions, in order of cost: is it well formed, does it follow what this
// sidechain has already committed, and is it signed by the publisher who owns it.
func (l *AnchorLedger) Verify(anchor SidechainAnchor, signature []byte) error {
	if err := anchor.Validate(); err != nil {
		return err
	}

	id := anchor.SidechainID()

	l.mu.RLock()
	state, hasState := l.states[id.String()]
	var previous SidechainAnchor
	hasPrevious := hasState && len(state.anchors) > 0
	if hasPrevious {
		previous = state.anchors[len(state.anchors)-1]
	}
	l.mu.RUnlock()

	// Contiguity is what makes an anchor a commitment rather than a snapshot. A
	// gap would leave a stretch of history uncommitted; an overlap would let a
	// publisher re-anchor a range with different contents, which is the rewrite
	// this is here to prevent.
	if hasPrevious {
		if anchor.FromHeight != previous.ToHeight+1 {
			return fmt.Errorf("%w: %s expects to start at %d, this anchor starts at %d",
				ErrAnchorNotContiguous, id, previous.ToHeight+1, anchor.FromHeight)
		}
	} else if anchor.FromHeight != 0 {
		return fmt.Errorf("%w: the first anchor for %s must start at height 0, not %d",
			ErrAnchorNotContiguous, id, anchor.FromHeight)
	}

	publicKey, err := l.registry.AuthoriseSidechain(id)
	if err != nil {
		return fmt.Errorf("%w: %w", ErrAnchorUnauthorised, err)
	}
	if len(signature) == 0 {
		return fmt.Errorf("%w: anchor is unsigned", ErrAnchorUnauthorised)
	}

	if err := VerifyPeerSignature(publicKey, anchor.SigningBytes(), signature); err != nil {
		return fmt.Errorf("%w: %w", ErrAnchorUnauthorised, err)
	}

	return nil
}

// Record verifies an anchor and commits it to the ledger.
func (l *AnchorLedger) Record(anchor SidechainAnchor, signature []byte, mainHeight int64) error {
	_, err := l.recordForUndo(anchor, signature, mainHeight)
	return err
}

// AnchorSidechain builds, signs, records, and marks an anchor in one step.
//
// This is the operation a publisher's node actually performs on a timer or every
// N blocks. Doing it as one call keeps the ledger and the chain's watermark from
// drifting apart: recording an anchor the chain does not know about would make
// the next anchor start too low and fail contiguity, and marking one the ledger
// rejected would silently skip a range.
//
// It returns nil when the chain has nothing new to commit.
func AnchorSidechain(chain *Sidechain, identity *PeerIdentity, ledger *AnchorLedger, mainHeight int64) (*SidechainAnchor, error) {
	if ledger == nil {
		return nil, fmt.Errorf("%w: ledger is nil", ErrInvalidAnchor)
	}

	anchor, err := BuildAnchor(chain)
	if err != nil || anchor == nil {
		return nil, err
	}

	signature, err := SignAnchor(identity, *anchor)
	if err != nil {
		return nil, err
	}

	if err := ledger.Record(*anchor, signature, mainHeight); err != nil {
		return nil, err
	}

	chain.AdvanceAnchored(anchor.ToHeight)
	return anchor, nil
}

// anchorRestore remembers a sidechain's anchor state as it was before a block
// was applied.
type anchorRestore struct {
	Key            string
	Existed        bool
	AnchorCount    int
	LastMainHeight int64
	Delinquent     bool
}

// recordForUndo verifies and records an anchor, returning what to restore if the
// block carrying it is later rolled back.
//
// A reorg that dropped an anchor without restoring the prior watermark would
// leave the ledger claiming a range was committed when the block committing it
// is no longer on the chain -- and the next honest anchor would then fail
// contiguity forever.
func (l *AnchorLedger) recordForUndo(anchor SidechainAnchor, signature []byte, mainHeight int64) (anchorRestore, error) {
	if err := l.Verify(anchor, signature); err != nil {
		return anchorRestore{}, err
	}

	key := anchor.SidechainID().String()

	l.mu.Lock()
	defer l.mu.Unlock()

	state, existed := l.states[key]
	if !existed {
		state = &anchorState{}
		l.states[key] = state
	}

	restore := anchorRestore{
		Key:            key,
		Existed:        existed,
		AnchorCount:    len(state.anchors),
		LastMainHeight: state.lastMainHeight,
		Delinquent:     state.delinquent,
	}

	// A gap longer than MaxAnchorGap means history sat uncommitted for longer
	// than the rules allow. The anchor still lands -- see MaxAnchorGap for why
	// refusing it would be worse -- but the lapse is recorded permanently.
	if len(state.anchors) > 0 && mainHeight-state.lastMainHeight > MaxAnchorGap {
		state.delinquent = true
	}

	state.anchors = append(state.anchors, anchor)
	state.mainHeights = append(state.mainHeights, mainHeight)
	state.lastMainHeight = mainHeight

	return restore, nil
}

// restore puts a sidechain's anchor state back to a previous point.
func (l *AnchorLedger) restore(r anchorRestore) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !r.Existed {
		delete(l.states, r.Key)
		return
	}

	state, ok := l.states[r.Key]
	if !ok {
		return
	}
	if r.AnchorCount <= len(state.anchors) {
		state.anchors = state.anchors[:r.AnchorCount]
	}
	if r.AnchorCount <= len(state.mainHeights) {
		state.mainHeights = state.mainHeights[:r.AnchorCount]
	}
	state.lastMainHeight = r.LastMainHeight
	state.delinquent = r.Delinquent
}
