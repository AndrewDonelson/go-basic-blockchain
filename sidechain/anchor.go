// File sidechain/anchor.go - Committing a sidechain's tip to the main chain.
package sidechain

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/merkle"
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

	// ErrAnchorSpanTooLarge is returned when one anchor tries to commit more
	// blocks than the window rules allow.
	ErrAnchorSpanTooLarge = errors.New("sidechain anchor covers too many blocks")
)

// HeaderDomain separates the tree of block hashes inside an anchor from
// every other tree in the system.
const HeaderDomain = "gbb/sidechain/header"

// MaxAnchorSpan bounds how many sidechain blocks one anchor may commit.
//
// This is half of what bounds the mutable window. Until a range is anchored the
// publisher can reorder or drop within it, so an unbounded span would let one
// anchor commit a year of history that had been rewritable the entire time --
// technically a commitment, practically no guarantee at all.
//
// Exceeding it is not fatal to a publisher who has fallen behind: they submit
// several anchors instead of one, and each is contiguous with the last.
const MaxAnchorSpan uint64 = 4096

// MaxAnchorGap bounds how many main-chain blocks may pass between one anchor and
// the next before a sidechain is marked delinquent.
//
// This is the other half of the window bound. MaxAnchorSpan limits how much
// history one anchor may commit; this limits how long history may stay
// uncommitted. Together they say: at any moment, at most MaxAnchorSpan blocks,
// written within the last MaxAnchorGap main-chain blocks, are still rewritable.
// That is a bounded claim a player or a store can actually rely on, where "the
// publisher anchors when they feel like it" is not.
//
// A late anchor is still accepted. Refusing it would be perverse: a publisher
// whose node was down for an afternoon could never catch up, and their whole
// history would be permanently uncommittable. Lateness is recorded instead, and
// it is visible to anyone who asks -- which is the proportionate response to an
// outage, as against withholding data, which is a fault and is bonded.
const MaxAnchorGap int64 = 1000

// Anchor commits a range of a sidechain's history to the main chain.
//
// Protobuf-shaped: fixed-width scalars and fixed-length hashes only.
type Anchor struct {
	PublisherID uint64
	GameID      uint64

	// FromHeight and ToHeight are inclusive, and must be contiguous with the
	// previous anchor for this sidechain.
	FromHeight uint64
	ToHeight   uint64

	// TipHash is the hash of the block at ToHeight. It is the commitment: the
	// sidechain's chaining makes it cover every block below.
	TipHash []byte

	// HeaderRoot is a Merkle root over the block hashes in [FromHeight,
	// ToHeight].
	//
	// TipHash alone already commits the range -- the blocks chain by
	// PreviousHash -- but proving anything about one block against it means
	// producing every header between that block and the tip. That is O(range),
	// and an availability challenge that expensive is one nobody issues, which
	// would leave availability as a promise rather than an obligation.
	//
	// The header root makes any block in the range provable in O(log range), and
	// each block's own PayloadRoot then makes any single payload provable the
	// same way. Together they are what let a challenge be answered cheaply by an
	// honest publisher and not at all by one withholding data.
	HeaderRoot []byte

	// PayloadCount is what the publisher claims the range contains. It is
	// advisory -- nothing on the main chain can check it without the data -- and
	// is carried so an auditor holding the blocks can compare.
	PayloadCount uint64
}

// offsetOf locates a sidechain height within an anchor's range.
//
// Returning the index and the count together, behind a bounds check, is what
// lets every caller convert to int safely: MaxAnchorSpan is enforced by
// Validate, so a range that reaches here is small enough that the conversion
// cannot lose anything on any platform Go supports.
func (a Anchor) Offset(height uint64) (index, count int, ok bool) {
	if height < a.FromHeight || height > a.ToHeight {
		return 0, 0, false
	}
	span := a.ToHeight - a.FromHeight + 1
	if span > MaxAnchorSpan {
		return 0, 0, false
	}
	return int(height - a.FromHeight), int(span), true //nolint:gosec // bounded by MaxAnchorSpan
}

// SidechainID returns the identity this anchor commits.
func (a Anchor) SidechainID() ID {
	return ID{PublisherID: a.PublisherID, GameID: a.GameID}
}

// Validate checks an anchor's internal consistency.
func (a Anchor) Validate() error {
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
	if len(a.HeaderRoot) != sha256.Size {
		return fmt.Errorf("%w: header root is %d bytes, expected %d",
			ErrInvalidAnchor, len(a.HeaderRoot), sha256.Size)
	}
	if span := a.ToHeight - a.FromHeight + 1; span > MaxAnchorSpan {
		return fmt.Errorf("%w: an anchor covers %d blocks, the limit is %d",
			ErrAnchorSpanTooLarge, span, MaxAnchorSpan)
	}
	return nil
}

// SigningBytes returns the canonical payload an anchor's signature covers.
//
// Every field the anchor asserts is included. Leaving the range out would let one
// signature be reused to commit a different span of history, which is precisely
// the rewrite the anchor exists to prevent.
func (a Anchor) SigningBytes() []byte {
	var buf bytes.Buffer
	buf.WriteString("gbb/sidechain/anchor/v1")

	var tmp [8]byte
	for _, v := range []uint64{a.PublisherID, a.GameID, a.FromHeight, a.ToHeight, a.PayloadCount} {
		binary.BigEndian.PutUint64(tmp[:], v)
		buf.Write(tmp[:])
	}

	for _, field := range [][]byte{a.TipHash, a.HeaderRoot} {
		binary.BigEndian.PutUint64(tmp[:], uint64(len(field)))
		buf.Write(tmp[:])
		buf.Write(field)
	}

	return buf.Bytes()
}

// SignAnchor signs an anchor with a publisher's identity key (ECDSA P-256), the
// signature the main chain checks against the publisher's registered key.
func SignAnchor(key *Key, anchor Anchor) ([]byte, error) {
	if key == nil {
		return nil, fmt.Errorf("%w: no publisher key", ErrAnchorUnauthorised)
	}
	if err := anchor.Validate(); err != nil {
		return nil, err
	}
	return key.Sign(anchor.SigningBytes())
}

// VerifyAnchorSignature checks a publisher's signature over an anchor.
func VerifyAnchorSignature(publicKeyPEM string, anchor Anchor, signature []byte) error {
	if err := VerifySignature(publicKeyPEM, anchor.SigningBytes(), signature); err != nil {
		return fmt.Errorf("%w: %w", ErrAnchorUnauthorised, err)
	}
	return nil
}

// BuildAnchor produces the anchor committing a sidechain's pending range.
//
// It returns nil when there is nothing new to commit, which is the normal state
// between blocks rather than an error. A pending range longer than MaxAnchorSpan
// is committed MaxAnchorSpan blocks at a time: once this anchor is recorded and
// marked anchored, the next call builds the following range.
func BuildAnchor(chain *Chain) (*Anchor, error) {
	if chain == nil {
		return nil, fmt.Errorf("%w: chain is nil", ErrInvalidAnchor)
	}

	from, to, ok := chain.PendingAnchor()
	if !ok {
		return nil, nil
	}
	// One anchor covers at most MaxAnchorSpan blocks. A longer backlog is
	// committed in several anchors: anchor this one, mark it anchored, and call
	// BuildAnchor again for the next range.
	if to-from+1 > MaxAnchorSpan {
		to = from + MaxAnchorSpan - 1
	}

	tip, found := chain.BlockAt(to)
	if !found {
		return nil, fmt.Errorf("%w: no block at height %d", ErrInvalidAnchor, to)
	}

	var payloads uint64
	hashes := make([][]byte, 0, to-from+1)
	for height := from; height <= to; height++ {
		block, found := chain.BlockAt(height)
		if !found {
			return nil, fmt.Errorf("%w: the range has a gap at height %d",
				ErrInvalidAnchor, height)
		}
		payloads += uint64(block.Header.PayloadCount)
		hashes = append(hashes, block.Hash)
	}

	id := chain.ID()
	anchor := &Anchor{
		PublisherID:  id.PublisherID,
		GameID:       id.GameID,
		FromHeight:   from,
		ToHeight:     to,
		TipHash:      append([]byte{}, tip.Hash...),
		HeaderRoot:   merkle.Root(HeaderDomain, hashes),
		PayloadCount: payloads,
	}
	if err := anchor.Validate(); err != nil {
		return nil, err
	}
	return anchor, nil
}

// VerifyAgainstChain checks that a sidechain's blocks actually produce an
// anchor's commitment.
//
// This is the audit an anchor makes possible: given the blocks, confirm the
// history is the one that was committed. It is the reason the anchor is worth
// anything -- a commitment nobody can check against the data is decoration.
func VerifyAgainstChain(anchor Anchor, chain *Chain) error {
	if chain == nil {
		return fmt.Errorf("%w: chain is nil", ErrInvalidAnchor)
	}
	// A malformed anchor (a backwards range, short hashes, an oversized span) is
	// refused before its fields are used to size anything.
	if err := anchor.Validate(); err != nil {
		return err
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
	// statement about every block in the range.
	//
	// Only this anchor's own range is checked. Everything below it was covered
	// by the anchor before it, and contiguity is what makes that argument hold
	// -- which is why a gap between anchors is refused rather than tolerated.
	hashes := make([][]byte, 0, anchor.ToHeight-anchor.FromHeight+1)
	for height := anchor.FromHeight; height <= anchor.ToHeight; height++ {
		block, ok := chain.BlockAt(height)
		if !ok {
			return fmt.Errorf("%w: the chain has a gap at height %d",
				ErrInvalidAnchor, height)
		}
		hashes = append(hashes, block.Hash)

		if height == 0 {
			continue
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

	if !bytes.Equal(merkle.Root(HeaderDomain, hashes), anchor.HeaderRoot) {
		return fmt.Errorf("%w: the range does not match the anchored header root",
			ErrInvalidAnchor)
	}

	return nil
}
