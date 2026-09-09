// Package sdk is a software development kit for building blockchain applications.
// File sdk/sidechain_block.go - Blocks in a sidechain's own block space.
package sdk

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

// sidechainBlockVersion is the header version these blocks are produced at.
const sidechainBlockVersion uint32 = 1

// maxSidechainPayloads bounds how many entries one block may carry.
//
// The payloads come from a game's own traffic, so the bound is what stops a
// misbehaving or compromised publisher node producing a block large enough to
// exhaust memory in anything that reads it -- including the tooling that will
// later verify anchors.
const maxSidechainPayloads = 65536

var (
	// ErrInvalidSidechainBlock is returned for a structurally invalid block.
	ErrInvalidSidechainBlock = errors.New("invalid sidechain block")

	// ErrSidechainNotContiguous is returned when a block does not follow its
	// predecessor.
	ErrSidechainNotContiguous = errors.New("sidechain block does not follow the tip")
)

// SidechainBlockHeader is the committed part of a sidechain block.
//
// Every field is fixed-width or a fixed-length hash, so the serialisation is
// unambiguous without delimiters and maps onto protobuf scalars directly. The
// timestamp is Unix nanoseconds rather than a time.Time: a wire format has no
// business carrying a monotonic clock reading or a timezone, and this project has
// already been bitten once by a Time whose String() form changed a block's hash.
type SidechainBlockHeader struct {
	Version           uint32
	PublisherID       uint64
	GameID            uint64
	Height            uint64
	PreviousHash      []byte // 32 bytes; empty only at height 0
	PayloadRoot       []byte // 32 bytes, Merkle root over Payloads
	TimestampUnixNano int64
	PayloadCount      uint32
}

// SidechainBlock is one block in a game's block space.
//
// Payloads are opaque here. What a game writes -- an achievement unlock, a
// leaderboard entry, an inventory change -- is the concern of the service that
// produced it; this layer's job is to order it, chain it, and make it committable
// to the main chain.
type SidechainBlock struct {
	Header   SidechainBlockHeader
	Hash     []byte
	Payloads [][]byte
}

// SidechainID returns the identity this block belongs to.
func (b *SidechainBlock) SidechainID() SidechainID {
	return SidechainID{PublisherID: b.Header.PublisherID, GameID: b.Header.GameID}
}

// headerBytes serialises the header for hashing.
//
// Length-prefixed and fixed-width throughout. Concatenating variable-length
// fields without prefixes is how two distinct headers end up with one preimage,
// which is the collision the main chain's header serialisation had to be fixed
// for.
func (h SidechainBlockHeader) headerBytes() []byte {
	var buf bytes.Buffer

	writeU32 := func(v uint32) {
		var tmp [4]byte
		binary.BigEndian.PutUint32(tmp[:], v)
		buf.Write(tmp[:])
	}
	writeU64 := func(v uint64) {
		var tmp [8]byte
		binary.BigEndian.PutUint64(tmp[:], v)
		buf.Write(tmp[:])
	}
	// A 64-bit length prefix. A 32-bit one would need a narrowing conversion
	// from int, and a value that wrapped would let two different fields share a
	// preimage -- exactly the collision the prefixes are here to prevent.
	writeBytes := func(v []byte) {
		writeU64(uint64(len(v)))
		buf.Write(v)
	}

	writeU32(h.Version)
	writeU64(h.PublisherID)
	writeU64(h.GameID)
	writeU64(h.Height)
	writeBytes(h.PreviousHash)
	writeBytes(h.PayloadRoot)
	// Reinterpreting the signed timestamp's bits, not converting its value: the
	// mapping is injective either way, which is all a preimage needs.
	writeU64(uint64(h.TimestampUnixNano)) //nolint:gosec // intentional bit reinterpretation
	writeU32(h.PayloadCount)

	return buf.Bytes()
}

// ComputeHash returns the block's hash, derived from its header alone.
//
// The payloads are committed through PayloadRoot, so the hash covers them without
// the hash function ever touching them -- which is what lets a verifier check a
// chain of headers without holding the data.
func (b *SidechainBlock) ComputeHash() []byte {
	digest := sha256.Sum256(b.Header.headerBytes())
	return digest[:]
}

// sidechainPayloadRoot computes the Merkle root over a block's payloads.
//
// Domain-separated leaves and interior nodes, for the reason the main chain's
// Merkle tree is: without separation, an interior node can be presented as a leaf
// and two different payload sets can produce one root.
func sidechainPayloadRoot(payloads [][]byte) []byte {
	if len(payloads) == 0 {
		empty := sha256.Sum256([]byte("gbb/sidechain/empty"))
		return empty[:]
	}

	level := make([][]byte, 0, len(payloads))
	for _, payload := range payloads {
		leaf := sha256.Sum256(append([]byte("gbb/sidechain/leaf"), payload...))
		level = append(level, leaf[:])
	}

	for len(level) > 1 {
		next := make([][]byte, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			left := level[i]
			// An odd node is paired with itself, and the domain byte keeps that
			// from being confusable with a genuine two-child node.
			right := left
			if i+1 < len(level) {
				right = level[i+1]
			}
			combined := make([]byte, 0, len(left)+len(right)+len("gbb/sidechain/node"))
			combined = append(combined, []byte("gbb/sidechain/node")...)
			combined = append(combined, left...)
			combined = append(combined, right...)
			parent := sha256.Sum256(combined)
			next = append(next, parent[:])
		}
		level = next
	}

	return level[0]
}

// NewSidechainBlock builds the next block for a sidechain.
//
// previous may be nil, which produces the sidechain's genesis block at height 0.
func NewSidechainBlock(id SidechainID, previous *SidechainBlock, payloads [][]byte, timestampUnixNano int64) (*SidechainBlock, error) {
	if err := id.Validate(); err != nil {
		return nil, err
	}
	if len(payloads) > maxSidechainPayloads {
		return nil, fmt.Errorf("%w: %d payloads exceeds the %d limit",
			ErrInvalidSidechainBlock, len(payloads), maxSidechainPayloads)
	}

	var height uint64
	var previousHash []byte

	if previous != nil {
		if !previous.SidechainID().Equal(id) {
			return nil, fmt.Errorf("%w: previous block belongs to %s, not %s",
				ErrSidechainMismatch, previous.SidechainID(), id)
		}
		height = previous.Header.Height + 1
		previousHash = append([]byte{}, previous.Hash...)
	}

	block := &SidechainBlock{
		Header: SidechainBlockHeader{
			Version:           sidechainBlockVersion,
			PublisherID:       id.PublisherID,
			GameID:            id.GameID,
			Height:            height,
			PreviousHash:      previousHash,
			PayloadRoot:       sidechainPayloadRoot(payloads),
			TimestampUnixNano: timestampUnixNano,
			// Bounded by maxSidechainPayloads above, which is far below the
			// uint32 ceiling.
			PayloadCount: uint32(len(payloads)), //nolint:gosec // bounded above
		},
		Payloads: payloads,
	}
	block.Hash = block.ComputeHash()

	return block, nil
}

// Validate checks a block's internal consistency.
//
// This is the parent-independent half: shape, self-consistency, and that the
// payloads are the ones the header commits to. Whether it follows the tip is a
// question for the chain that holds it.
func (b *SidechainBlock) Validate() error {
	if b == nil {
		return fmt.Errorf("%w: block is nil", ErrInvalidSidechainBlock)
	}
	if err := b.SidechainID().Validate(); err != nil {
		return err
	}
	if b.Header.Version == 0 {
		return fmt.Errorf("%w: header version is zero", ErrInvalidSidechainBlock)
	}
	if len(b.Payloads) > maxSidechainPayloads {
		return fmt.Errorf("%w: %d payloads exceeds the %d limit",
			ErrInvalidSidechainBlock, len(b.Payloads), maxSidechainPayloads)
	}
	if int(b.Header.PayloadCount) != len(b.Payloads) {
		return fmt.Errorf("%w: header declares %d payloads, block carries %d",
			ErrInvalidSidechainBlock, b.Header.PayloadCount, len(b.Payloads))
	}

	// The root must be the one these payloads produce, or the header commits to
	// data the block does not contain.
	if !bytes.Equal(b.Header.PayloadRoot, sidechainPayloadRoot(b.Payloads)) {
		return fmt.Errorf("%w: payload root does not match the payloads",
			ErrInvalidSidechainBlock)
	}

	if !bytes.Equal(b.Hash, b.ComputeHash()) {
		return fmt.Errorf("%w: hash does not match the header", ErrInvalidSidechainBlock)
	}

	if b.Header.Height == 0 {
		if len(b.Header.PreviousHash) != 0 {
			return fmt.Errorf("%w: the first block must have no predecessor",
				ErrInvalidSidechainBlock)
		}
		return nil
	}
	if len(b.Header.PreviousHash) != sha256.Size {
		return fmt.Errorf("%w: previous hash is %d bytes, expected %d",
			ErrInvalidSidechainBlock, len(b.Header.PreviousHash), sha256.Size)
	}

	return nil
}

// Follows reports whether this block extends the given tip.
func (b *SidechainBlock) Follows(tip *SidechainBlock) error {
	if tip == nil {
		if b.Header.Height != 0 {
			return fmt.Errorf("%w: height %d has no predecessor to follow",
				ErrSidechainNotContiguous, b.Header.Height)
		}
		return nil
	}

	if !b.SidechainID().Equal(tip.SidechainID()) {
		return fmt.Errorf("%w: %s cannot extend %s",
			ErrSidechainMismatch, b.SidechainID(), tip.SidechainID())
	}
	if b.Header.Height != tip.Header.Height+1 {
		return fmt.Errorf("%w: height %d does not follow %d",
			ErrSidechainNotContiguous, b.Header.Height, tip.Header.Height)
	}
	if !bytes.Equal(b.Header.PreviousHash, tip.Hash) {
		return fmt.Errorf("%w: previous hash does not match the tip",
			ErrSidechainNotContiguous)
	}
	return nil
}
