// Package sdk is a software development kit for building blockchain applications.
// File sdk/sidechain_anchor.go - Committing a sidechain's tip to the main chain.
package sdk

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
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
// WHAT AN ANCHOR IS NOT
//
// It is not data availability. The anchor proves a history existed; it does not
// make the publisher hand it over. A publisher who loses or withholds the blocks
// leaves everyone able to prove that something was committed and unable to say
// what. That is a real limit and the honest place to state it.
//
// Nor does it constrain the unanchored window. Between anchors the publisher can
// still reorder or drop; anchoring more often narrows that window at the cost of
// more main-chain traffic, which is the dial an operator actually has.

var (
	// ErrInvalidAnchor is returned for a structurally invalid anchor.
	ErrInvalidAnchor = errors.New("invalid sidechain anchor")

	// ErrAnchorNotContiguous is returned when an anchor leaves a gap or
	// re-anchors history already committed.
	ErrAnchorNotContiguous = errors.New("sidechain anchor is not contiguous")

	// ErrAnchorUnauthorised is returned when an anchor is not signed by the
	// publisher that owns the sidechain.
	ErrAnchorUnauthorised = errors.New("sidechain anchor is not authorised")
)

// SidechainAnchor commits a range of a sidechain's history to the main chain.
//
// Protobuf-shaped: fixed-width scalars and fixed-length hashes only.
type SidechainAnchor struct {
	PublisherID uint64
	GameID      uint64

	// FromHeight and ToHeight are inclusive, and must be contiguous with the
	// previous anchor for this sidechain.
	FromHeight uint64
	ToHeight   uint64

	// TipHash is the hash of the block at ToHeight. It is the commitment: the
	// sidechain's chaining makes it cover every block below.
	TipHash []byte

	// PayloadCount is what the publisher claims the range contains. It is
	// advisory -- nothing on the main chain can check it without the data -- and
	// is carried so an auditor holding the blocks can compare.
	PayloadCount uint64
}

// SidechainID returns the identity this anchor commits.
func (a SidechainAnchor) SidechainID() SidechainID {
	return SidechainID{PublisherID: a.PublisherID, GameID: a.GameID}
}

// Validate checks an anchor's internal consistency.
func (a SidechainAnchor) Validate() error {
	if err := a.SidechainID().Validate(); err != nil {
		return err
	}
	if a.ToHeight < a.FromHeight {
		return fmt.Errorf("%w: range %d..%d runs backwards",
			ErrInvalidAnchor, a.FromHeight, a.ToHeight)
	}
	if len(a.TipHash) != sha256.Size {
		return fmt.Errorf("%w: tip hash is %d bytes, expected %d",
			ErrInvalidAnchor, len(a.TipHash), sha256.Size)
	}
	return nil
}

// SigningBytes returns the canonical payload an anchor's signature covers.
//
// Every field the anchor asserts is included. Leaving the range out would let one
// signature be reused to commit a different span of history, which is precisely
// the rewrite the anchor exists to prevent.
func (a SidechainAnchor) SigningBytes() []byte {
	var buf bytes.Buffer
	buf.WriteString("gbb/sidechain/anchor/v1")

	var tmp [8]byte
	for _, v := range []uint64{a.PublisherID, a.GameID, a.FromHeight, a.ToHeight, a.PayloadCount} {
		binary.BigEndian.PutUint64(tmp[:], v)
		buf.Write(tmp[:])
	}

	binary.BigEndian.PutUint64(tmp[:], uint64(len(a.TipHash)))
	buf.Write(tmp[:])
	buf.Write(a.TipHash)

	return buf.Bytes()
}

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

// BuildAnchor produces the anchor committing a sidechain's pending range.
//
// It returns nil when there is nothing new to commit, which is the normal state
// between blocks rather than an error.
func BuildAnchor(chain *Sidechain) (*SidechainAnchor, error) {
	if chain == nil {
		return nil, fmt.Errorf("%w: chain is nil", ErrInvalidAnchor)
	}

	from, to, ok := chain.PendingAnchor()
	if !ok {
		return nil, nil
	}

	tip, found := chain.BlockAt(to)
	if !found {
		return nil, fmt.Errorf("%w: no block at height %d", ErrInvalidAnchor, to)
	}

	var payloads uint64
	for height := from; height <= to; height++ {
		block, found := chain.BlockAt(height)
		if !found {
			return nil, fmt.Errorf("%w: the range has a gap at height %d",
				ErrInvalidAnchor, height)
		}
		payloads += uint64(block.Header.PayloadCount)
	}

	id := chain.ID()
	return &SidechainAnchor{
		PublisherID:  id.PublisherID,
		GameID:       id.GameID,
		FromHeight:   from,
		ToHeight:     to,
		TipHash:      append([]byte{}, tip.Hash...),
		PayloadCount: payloads,
	}, nil
}

// PublisherRegistry resolves which key may anchor a publisher's sidechains.
//
// Authorisation is the difference between "a sidechain" and "anybody's
// sidechain": without it one publisher could anchor another's identity and
// commit a history they do not own. Registration binds a PublisherID to a key
// once; every anchor afterwards is checked against it.
type PublisherRegistry struct {
	mu   sync.RWMutex
	keys map[uint64]string // publisher id -> public key PEM
}

// NewPublisherRegistry creates an empty registry.
func NewPublisherRegistry() *PublisherRegistry {
	return &PublisherRegistry{keys: map[uint64]string{}}
}

// Register binds a publisher id to a public key.
//
// A publisher id may be claimed once. Allowing re-registration would let whoever
// registered last take over an existing publisher's sidechains, which is the same
// account-takeover shape this project already had to fix once at the API layer.
func (r *PublisherRegistry) Register(publisherID uint64, publicKeyPEM string) error {
	if publisherID == 0 {
		return fmt.Errorf("%w: publisher id must not be zero", ErrInvalidSidechainID)
	}
	if publicKeyPEM == "" {
		return fmt.Errorf("%w: public key is empty", ErrInvalidSidechainID)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, taken := r.keys[publisherID]; taken {
		if existing == publicKeyPEM {
			return nil // idempotent
		}
		return fmt.Errorf("publisher %d is already registered to a different key", publisherID)
	}

	r.keys[publisherID] = publicKeyPEM
	return nil
}

// PublicKey returns a publisher's registered key.
func (r *PublisherRegistry) PublicKey(publisherID uint64) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	key, ok := r.keys[publisherID]
	return key, ok
}

// AnchorLedger tracks what has been anchored, per sidechain.
//
// This is the main chain's view. It holds no game data -- only the commitments --
// which is what keeps the main chain's cost proportional to the number of
// publishers rather than to how much their players do.
type AnchorLedger struct {
	mu       sync.RWMutex
	registry *PublisherRegistry
	latest   map[string]SidechainAnchor
}

// NewAnchorLedger creates a ledger backed by a publisher registry.
func NewAnchorLedger(registry *PublisherRegistry) *AnchorLedger {
	if registry == nil {
		registry = NewPublisherRegistry()
	}
	return &AnchorLedger{registry: registry, latest: map[string]SidechainAnchor{}}
}

// Registry returns the ledger's publisher registry.
func (l *AnchorLedger) Registry() *PublisherRegistry { return l.registry }

// Latest returns the most recent anchor for a sidechain.
func (l *AnchorLedger) Latest(id SidechainID) (SidechainAnchor, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	anchor, ok := l.latest[id.String()]
	return anchor, ok
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
	previous, hasPrevious := l.latest[id.String()]
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

	publicKey, registered := l.registry.PublicKey(anchor.PublisherID)
	if !registered {
		return fmt.Errorf("%w: publisher %d is not registered",
			ErrAnchorUnauthorised, anchor.PublisherID)
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
func (l *AnchorLedger) Record(anchor SidechainAnchor, signature []byte) error {
	if err := l.Verify(anchor, signature); err != nil {
		return err
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	l.latest[anchor.SidechainID().String()] = anchor
	return nil
}

// VerifyAgainstChain checks that a sidechain's blocks actually produce an
// anchor's commitment.
//
// This is the audit an anchor makes possible: given the blocks, confirm the
// history is the one that was committed. It is the reason the anchor is worth
// anything -- a commitment nobody can check against the data is decoration.
func VerifyAgainstChain(anchor SidechainAnchor, chain *Sidechain) error {
	if chain == nil {
		return fmt.Errorf("%w: chain is nil", ErrInvalidAnchor)
	}
	if !chain.ID().Equal(anchor.SidechainID()) {
		return fmt.Errorf("%w: anchor commits %s, chain is %s",
			ErrSidechainMismatch, anchor.SidechainID(), chain.ID())
	}

	tip, ok := chain.BlockAt(anchor.ToHeight)
	if !ok {
		return fmt.Errorf("%w: the chain has no block at height %d",
			ErrInvalidAnchor, anchor.ToHeight)
	}
	if !bytes.Equal(tip.Hash, anchor.TipHash) {
		return fmt.Errorf("%w: the block at height %d does not hash to the anchored "+
			"commitment; this history is not the one that was anchored",
			ErrInvalidAnchor, anchor.ToHeight)
	}

	// Walk the range and confirm it is a chain. The tip hash covers the prefix
	// only if the links are intact, so this is what turns one hash into a
	// statement about every block below it.
	for height := anchor.ToHeight; height > 0; height-- {
		block, ok := chain.BlockAt(height)
		if !ok {
			return fmt.Errorf("%w: the chain has a gap at height %d",
				ErrInvalidAnchor, height)
		}
		parent, ok := chain.BlockAt(height - 1)
		if !ok {
			return fmt.Errorf("%w: the chain has a gap at height %d",
				ErrInvalidAnchor, height-1)
		}
		if err := block.Follows(parent); err != nil {
			return fmt.Errorf("%w: broken link at height %d: %w",
				ErrInvalidAnchor, height, err)
		}
	}

	return nil
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
func AnchorSidechain(chain *Sidechain, identity *PeerIdentity, ledger *AnchorLedger) (*SidechainAnchor, error) {
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

	if err := ledger.Record(*anchor, signature); err != nil {
		return nil, err
	}

	chain.markAnchored(anchor.ToHeight)
	return anchor, nil
}

// anchorRestore remembers a sidechain's anchor as it was before a block was
// applied, distinguishing "had none" from "had one".
type anchorRestore struct {
	Key      string
	Previous SidechainAnchor
	Existed  bool
}

// recordForUndo verifies and records an anchor, returning what to restore if the
// block carrying it is later rolled back.
//
// A reorg that dropped an anchor without restoring the prior watermark would
// leave the ledger claiming a range was committed when the block committing it
// is no longer on the chain -- and the next honest anchor would then fail
// contiguity forever.
func (l *AnchorLedger) recordForUndo(anchor SidechainAnchor, signature []byte) (anchorRestore, error) {
	if err := l.Verify(anchor, signature); err != nil {
		return anchorRestore{}, err
	}

	key := anchor.SidechainID().String()

	l.mu.Lock()
	defer l.mu.Unlock()

	previous, existed := l.latest[key]
	l.latest[key] = anchor
	return anchorRestore{Key: key, Previous: previous, Existed: existed}, nil
}

// restore puts a sidechain's anchor back to a previous state.
func (l *AnchorLedger) restore(r anchorRestore) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if r.Existed {
		l.latest[r.Key] = r.Previous
		return
	}
	delete(l.latest, r.Key)
}
