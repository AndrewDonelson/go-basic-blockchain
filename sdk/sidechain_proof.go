// Package sdk is a software development kit for building blockchain applications.
// File sdk/sidechain_proof.go - Portable inclusion proofs for sidechain records.
package sdk

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// WHAT THESE PROOFS ARE FOR
//
// An anchor on the main chain commits a range of a sidechain's history, and the
// two Merkle roots underneath it -- HeaderRoot over block hashes, PayloadRoot
// over each block's payloads -- make any single payload provable in O(log n).
// The availability mechanism already relies on that internally. These types
// expose the same chain of evidence to anybody:
//
//	payload --PayloadProof--> block header --BlockProof--> anchor --> main chain
//
// so that a party holding ONE payload, a RecordProof and the anchor (which they
// can read from any node) can confirm the payload is part of the publisher's
// committed history, without trusting the publisher, the node that served the
// anchor's surrounding blocks, or anyone else.
//
// Nothing here is new cryptography. The proofs reuse the domain-separated,
// count-bound trees in merkle.go, which is what makes them safe to hand to a
// stranger: a payload proof cannot be replayed against a header root (different
// domain), and a proof built for a tree of n leaves cannot be presented against
// a tree of n+1 (the count is folded into the root).

// ErrInvalidSidechainProof is returned when a proof does not establish what it
// claims.
var ErrInvalidSidechainProof = errors.New("invalid sidechain proof")

// recordProofVersion is the wire version of RecordProof. A reader refuses any
// other value rather than guessing at a shape it was not written for.
const recordProofVersion uint32 = 1

// PayloadProof proves one payload sits in a block's PayloadRoot.
//
// Count is part of the statement, not a hint: the root commits to the number of
// leaves, so a proof that lied about it would fail to reproduce the root. It is
// carried because a verifier holding only the root has no other way to know the
// tree's shape.
type PayloadProof struct {
	// Index is the payload's position within the block.
	Index uint32
	// Count is how many payloads the block carries.
	Count uint32
	// Path is the sibling hashes from the leaf upwards.
	Path [][]byte
}

// payloadProofWire is PayloadProof's pinned JSON shape.
type payloadProofWire struct {
	Index uint32   `json:"index"`
	Count uint32   `json:"count"`
	Path  []string `json:"path"`
}

// MarshalJSON encodes the proof with hex sibling hashes.
func (p PayloadProof) MarshalJSON() ([]byte, error) {
	return json.Marshal(payloadProofWire{Index: p.Index, Count: p.Count, Path: hexPath(p.Path)})
}

// UnmarshalJSON decodes the proof from its pinned JSON shape.
func (p *PayloadProof) UnmarshalJSON(data []byte) error {
	var w payloadProofWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	path, err := unhexPath(w.Path)
	if err != nil {
		return err
	}
	*p = PayloadProof{Index: w.Index, Count: w.Count, Path: path}
	return nil
}

// ProvePayload returns the proof that the payload at index sits in this block's
// PayloadRoot.
//
// The block is validated first. Proving against a block whose root does not
// match its own payloads would produce a proof that fails for the person it is
// handed to, which is a worse place to discover the problem than here.
func (b *SidechainBlock) ProvePayload(index int) (PayloadProof, error) {
	if err := b.Validate(); err != nil {
		return PayloadProof{}, err
	}
	path, err := merkleProof(sidechainPayloadDomain, b.Payloads, index)
	if err != nil {
		return PayloadProof{}, fmt.Errorf("%w: %w", ErrInvalidSidechainProof, err)
	}
	return PayloadProof{
		// Both bounded by maxSidechainPayloads, which Validate enforced above.
		Index: uint32(index),           //nolint:gosec // bounded by maxSidechainPayloads
		Count: uint32(len(b.Payloads)), //nolint:gosec // bounded by maxSidechainPayloads
		Path:  path,
	}, nil
}

// VerifyPayloadProof checks that payload sits at proof.Index in the tree whose
// root is payloadRoot.
func VerifyPayloadProof(payload []byte, proof PayloadProof, payloadRoot []byte) error {
	if len(payloadRoot) != sha256.Size {
		return fmt.Errorf("%w: payload root is %d bytes, expected %d",
			ErrInvalidSidechainProof, len(payloadRoot), sha256.Size)
	}
	if proof.Count == 0 || proof.Count > maxSidechainPayloads {
		return fmt.Errorf("%w: a block of %d payloads cannot be proved against",
			ErrInvalidSidechainProof, proof.Count)
	}
	if !merkleVerify(sidechainPayloadDomain, payloadRoot, payload,
		int(proof.Index), int(proof.Count), proof.Path) {
		return fmt.Errorf("%w: the payload does not sit at index %d of the payload root",
			ErrInvalidSidechainProof, proof.Index)
	}
	return nil
}

// BlockProof proves one block's hash sits in an anchor's HeaderRoot.
//
// It carries the height and the path, and nothing derivable from the anchor.
// The block's position inside the tree and the tree's size both follow from the
// anchor's own range, so a proof cannot claim a position the anchor does not
// assign it.
type BlockProof struct {
	// Height is the sidechain height of the proved block.
	Height uint64
	// Path is the sibling hashes from the leaf upwards.
	Path [][]byte
}

// blockProofWire is BlockProof's pinned JSON shape.
type blockProofWire struct {
	Height uint64   `json:"height"`
	Path   []string `json:"path"`
}

// MarshalJSON encodes the proof with hex sibling hashes.
func (p BlockProof) MarshalJSON() ([]byte, error) {
	return json.Marshal(blockProofWire{Height: p.Height, Path: hexPath(p.Path)})
}

// UnmarshalJSON decodes the proof from its pinned JSON shape.
func (p *BlockProof) UnmarshalJSON(data []byte) error {
	var w blockProofWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	path, err := unhexPath(w.Path)
	if err != nil {
		return err
	}
	*p = BlockProof{Height: w.Height, Path: path}
	return nil
}

// ProveBlock returns the proof that the block at height sits in the anchor's
// HeaderRoot.
//
// The chain's blocks across the anchor's whole range are hashed and their root
// compared with the anchor before any proof is produced. A chain that does not
// reproduce the anchor -- a rewritten block, the wrong sidechain, an anchor
// from somewhere else -- would otherwise yield a proof that silently fails for
// whoever receives it.
func ProveBlock(chain *Sidechain, anchor SidechainAnchor, height uint64) (BlockProof, error) {
	if chain == nil {
		return BlockProof{}, fmt.Errorf("%w: chain is nil", ErrInvalidSidechainProof)
	}
	if err := anchor.Validate(); err != nil {
		return BlockProof{}, err
	}
	if !chain.ID().Equal(anchor.SidechainID()) {
		return BlockProof{}, fmt.Errorf("%w: anchor commits %s, chain is %s",
			ErrSidechainMismatch, anchor.SidechainID(), chain.ID())
	}

	index, _, ok := anchor.offsetOf(height)
	if !ok {
		return BlockProof{}, fmt.Errorf("%w: height %d is outside the anchored range %d..%d",
			ErrInvalidSidechainProof, height, anchor.FromHeight, anchor.ToHeight)
	}

	hashes := make([][]byte, 0, anchor.ToHeight-anchor.FromHeight+1)
	for h := anchor.FromHeight; h <= anchor.ToHeight; h++ {
		block, found := chain.BlockAt(h)
		if !found {
			return BlockProof{}, fmt.Errorf("%w: the chain has no block at height %d",
				ErrInvalidSidechainProof, h)
		}
		hashes = append(hashes, block.Hash)
	}
	if !bytes.Equal(merkleRoot(sidechainHeaderDomain, hashes), anchor.HeaderRoot) {
		return BlockProof{}, fmt.Errorf("%w: the chain's blocks do not reproduce the "+
			"anchor's header root; this is not the history that was anchored",
			ErrInvalidSidechainProof)
	}

	path, err := merkleProof(sidechainHeaderDomain, hashes, index)
	if err != nil {
		return BlockProof{}, fmt.Errorf("%w: %w", ErrInvalidSidechainProof, err)
	}
	return BlockProof{Height: height, Path: path}, nil
}

// VerifyBlockProof checks that blockHash is the block at proof.Height in the
// range the anchor commits.
//
// Only the hash, the proof and the anchor are needed. The caller is expected to
// have computed blockHash from a header it holds (see VerifyRecord) rather than
// to have accepted one from the prover: a hash on its own says nothing about
// which header produced it.
func VerifyBlockProof(blockHash []byte, proof BlockProof, anchor SidechainAnchor) error {
	if err := anchor.Validate(); err != nil {
		return err
	}
	if len(blockHash) != sha256.Size {
		return fmt.Errorf("%w: block hash is %d bytes, expected %d",
			ErrInvalidSidechainProof, len(blockHash), sha256.Size)
	}

	index, count, ok := anchor.offsetOf(proof.Height)
	if !ok {
		return fmt.Errorf("%w: height %d is outside the anchored range %d..%d",
			ErrInvalidSidechainProof, proof.Height, anchor.FromHeight, anchor.ToHeight)
	}
	if !merkleVerify(sidechainHeaderDomain, anchor.HeaderRoot, blockHash, index, count, proof.Path) {
		return fmt.Errorf("%w: block %d does not sit in the anchor's header root",
			ErrInvalidSidechainProof, proof.Height)
	}
	return nil
}

// RecordProof is everything a third party needs, besides the payload itself and
// the on-chain anchor, to confirm one payload is part of anchored history.
//
// The header travels whole rather than as a hash. The verifier recomputes the
// block hash from it, so the PayloadRoot the payload is checked against is
// provably the one inside the block the anchor commits -- a prover cannot pair
// a hash that is in the anchor with header fields that are not. This is the
// same reasoning AvailabilityProof follows, for the same attack.
type RecordProof struct {
	// Header is the header of the block holding the payload.
	Header SidechainBlockHeader
	// Payload proves the payload sits in Header.PayloadRoot.
	Payload PayloadProof
	// Block proves Header's hash sits in the anchor's HeaderRoot.
	Block BlockProof
}

// recordProofWire is RecordProof's pinned JSON shape.
type recordProofWire struct {
	Version uint32              `json:"version"`
	Header  sidechainHeaderWire `json:"header"`
	Payload PayloadProof        `json:"payload_proof"`
	Block   BlockProof          `json:"block_proof"`
}

// MarshalJSON encodes the proof in its stable wire form.
//
//	{"version":1,
//	 "header":{...as in a SidechainBlock...},
//	 "payload_proof":{"index":0,"count":3,"path":["<hex>", ...]},
//	 "block_proof":{"height":7,"path":["<hex>", ...]}}
func (p *RecordProof) MarshalJSON() ([]byte, error) {
	return json.Marshal(recordProofWire{
		Version: recordProofVersion,
		Header:  headerToWire(p.Header),
		Payload: p.Payload,
		Block:   p.Block,
	})
}

// UnmarshalJSON decodes the proof, refusing a wire version it does not know.
func (p *RecordProof) UnmarshalJSON(data []byte) error {
	var w recordProofWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	if w.Version != recordProofVersion {
		return fmt.Errorf("%w: unsupported record proof version %d",
			ErrInvalidSidechainProof, w.Version)
	}
	header, err := headerFromWire(w.Header)
	if err != nil {
		return err
	}
	*p = RecordProof{Header: header, Payload: w.Payload, Block: w.Block}
	return nil
}

// ProveRecord builds the proof that the payload at index, in the block at
// height, is part of the history the anchor commits.
func ProveRecord(chain *Sidechain, anchor SidechainAnchor, height uint64, index int) (*RecordProof, error) {
	blockProof, err := ProveBlock(chain, anchor, height)
	if err != nil {
		return nil, err
	}

	// ProveBlock has confirmed the block exists and belongs to the anchor.
	block, _ := chain.BlockAt(height)
	payloadProof, err := block.ProvePayload(index)
	if err != nil {
		return nil, err
	}

	header := block.Header
	header.PreviousHash = append([]byte{}, header.PreviousHash...)
	header.PayloadRoot = append([]byte{}, header.PayloadRoot...)

	return &RecordProof{Header: header, Payload: payloadProof, Block: blockProof}, nil
}

// VerifyRecord checks that payload is part of the history anchor commits.
//
// The checks run from the outside in. The header must name the anchor's
// sidechain and the height the block proof claims; the block hash recomputed
// from that header must sit in the anchor's HeaderRoot at that height; and the
// payload must sit in that same header's PayloadRoot, in a tree of the size the
// header declares. Pass all of them and the payload is bound, through the
// anchor, to a mined main-chain block.
func VerifyRecord(payload []byte, proof *RecordProof, anchor SidechainAnchor) error {
	if proof == nil {
		return fmt.Errorf("%w: proof is nil", ErrInvalidSidechainProof)
	}
	if err := anchor.Validate(); err != nil {
		return err
	}

	header := proof.Header
	if header.PublisherID != anchor.PublisherID || header.GameID != anchor.GameID {
		return fmt.Errorf("%w: the header belongs to sidechain %d:%d, the anchor commits %s",
			ErrSidechainMismatch, header.PublisherID, header.GameID, anchor.SidechainID())
	}
	if header.Height != proof.Block.Height {
		return fmt.Errorf("%w: the header is for height %d, the block proof for %d",
			ErrInvalidSidechainProof, header.Height, proof.Block.Height)
	}

	// Recomputed, never supplied: see RecordProof.
	blockHash := (&SidechainBlock{Header: header}).ComputeHash()
	if err := VerifyBlockProof(blockHash, proof.Block, anchor); err != nil {
		return err
	}

	// The header is now known to be anchored, so its PayloadCount is a fact
	// about the block rather than the prover's claim. The payload proof must
	// agree with it, or it would be describing some other tree.
	if proof.Payload.Count != header.PayloadCount {
		return fmt.Errorf("%w: the payload proof is for a block of %d payloads, the header "+
			"declares %d", ErrInvalidSidechainProof, proof.Payload.Count, header.PayloadCount)
	}
	return VerifyPayloadProof(payload, proof.Payload, header.PayloadRoot)
}

// hexPath encodes a Merkle path for JSON. An empty path encodes as [] rather
// than null, so a reader in another language sees the same type either way.
func hexPath(path [][]byte) []string {
	out := make([]string, 0, len(path))
	for _, node := range path {
		out = append(out, hex.EncodeToString(node))
	}
	return out
}

// unhexPath decodes a Merkle path from JSON.
func unhexPath(path []string) ([][]byte, error) {
	out := make([][]byte, 0, len(path))
	for i, node := range path {
		decoded, err := hex.DecodeString(node)
		if err != nil {
			return nil, fmt.Errorf("%w: path element %d is not valid hex: %w",
				ErrInvalidSidechainProof, i, err)
		}
		out = append(out, decoded)
	}
	return out, nil
}
