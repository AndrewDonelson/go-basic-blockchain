// Package sdk is a software development kit for building blockchain applications.
// File sdk/sidechain_aliases.go - The sidechain primitives, defined in the sidechain package.
package sdk

import "github.com/AndrewDonelson/go-basic-blockchain/sidechain"

// Sidechain blocks, chains, anchors, proofs, their wire codec and the file store
// live in the dependency-free sidechain package, so an application can build,
// prove and anchor a sidechain without linking the node. The node uses the very
// same definitions through these names.

type (
	// SidechainID names one publisher's game: its own block space.
	SidechainID = sidechain.ID
	// SidechainBlockHeader is what a sidechain block's hash covers.
	SidechainBlockHeader = sidechain.BlockHeader
	// SidechainBlock is one block of a sidechain.
	SidechainBlock = sidechain.Block
	// Sidechain is one game's block space.
	Sidechain = sidechain.Chain
	// SidechainSet holds many sidechains.
	SidechainSet = sidechain.Set
	// SidechainAnchor commits a range of a sidechain's history to the main chain.
	SidechainAnchor = sidechain.Anchor
	// PayloadProof proves one payload against a block's payload root.
	PayloadProof = sidechain.PayloadProof
	// BlockProof proves one block against an anchor's header root.
	BlockProof = sidechain.BlockProof
	// RecordProof proves one payload against an anchor.
	RecordProof = sidechain.RecordProof
	// SidechainStore persists sidechain blocks and the anchored watermark.
	SidechainStore = sidechain.Store
	// FileSidechainStore keeps each sidechain in its own append-only file.
	FileSidechainStore = sidechain.FileStore
)

// Limits and domains.
const (
	MaxAnchorSpan = sidechain.MaxAnchorSpan
	MaxAnchorGap  = sidechain.MaxAnchorGap

	sidechainHeaderDomain  = sidechain.HeaderDomain
	sidechainPayloadDomain = sidechain.PayloadDomain
	maxSidechainPayloads   = sidechain.MaxPayloads
)

// Errors.
var (
	ErrInvalidSidechainBlock  = sidechain.ErrInvalidSidechainBlock
	ErrSidechainNotContiguous = sidechain.ErrSidechainNotContiguous
	ErrInvalidSidechainID     = sidechain.ErrInvalidSidechainID
	ErrSidechainMismatch      = sidechain.ErrSidechainMismatch
	ErrInvalidSidechainProof  = sidechain.ErrInvalidSidechainProof
	ErrInvalidAnchor          = sidechain.ErrInvalidAnchor
	ErrAnchorNotContiguous    = sidechain.ErrAnchorNotContiguous
	ErrAnchorUnauthorised     = sidechain.ErrAnchorUnauthorised
	ErrAnchorSpanTooLarge     = sidechain.ErrAnchorSpanTooLarge
	ErrSidechainStoreCorrupt  = sidechain.ErrStoreCorrupt
)

// NewSidechainID builds a sidechain identity; both components must be non-zero.
func NewSidechainID(publisherID, gameID uint64) (SidechainID, error) {
	return sidechain.NewID(publisherID, gameID)
}

// SidechainIDFromBytes decodes SidechainID.Bytes.
func SidechainIDFromBytes(data []byte) (SidechainID, error) { return sidechain.IDFromBytes(data) }

// NewSidechainBlock builds the next block for a sidechain (previous nil: genesis).
func NewSidechainBlock(id SidechainID, previous *SidechainBlock, payloads [][]byte, timestampUnixNano int64) (*SidechainBlock, error) {
	return sidechain.NewBlock(id, previous, payloads, timestampUnixNano)
}

// VerifyPayloadInclusion checks a payload against a block's committed payload root.
func VerifyPayloadInclusion(payloadRoot, payload []byte, index, count int, path [][]byte) bool {
	return sidechain.VerifyPayloadInclusion(payloadRoot, payload, index, count, path)
}

// NewSidechain creates an empty block space for an identity.
func NewSidechain(id SidechainID) (*Sidechain, error) { return sidechain.NewChain(id) }

// RestoreSidechain rebuilds a sidechain from persisted blocks, verifying every link.
func RestoreSidechain(id SidechainID, blocks []*SidechainBlock, anchoredThrough *uint64) (*Sidechain, error) {
	return sidechain.Restore(id, blocks, anchoredThrough)
}

// NewSidechainSet creates an empty set.
func NewSidechainSet() *SidechainSet { return sidechain.NewSet() }

// OpenFileSidechainStore opens (creating if needed) a store rooted at dir.
func OpenFileSidechainStore(dir string) (*FileSidechainStore, error) {
	return sidechain.OpenFileStore(dir)
}

// BuildAnchor produces the anchor committing a sidechain's pending range (nil: nothing new).
func BuildAnchor(chain *Sidechain) (*SidechainAnchor, error) { return sidechain.BuildAnchor(chain) }

// VerifyAgainstChain checks that a sidechain's blocks produce an anchor's commitment.
func VerifyAgainstChain(anchor SidechainAnchor, chain *Sidechain) error {
	return sidechain.VerifyAgainstChain(anchor, chain)
}

// VerifyPayloadProof checks a payload against a block's payload root.
func VerifyPayloadProof(payload []byte, proof PayloadProof, payloadRoot []byte) error {
	return sidechain.VerifyPayloadProof(payload, proof, payloadRoot)
}

// ProveBlock proves the block at height against the anchor covering it.
func ProveBlock(chain *Sidechain, anchor SidechainAnchor, height uint64) (BlockProof, error) {
	return sidechain.ProveBlock(chain, anchor, height)
}

// VerifyBlockProof checks a block hash against an anchor's header root.
func VerifyBlockProof(blockHash []byte, proof BlockProof, anchor SidechainAnchor) error {
	return sidechain.VerifyBlockProof(blockHash, proof, anchor)
}

// ProveRecord proves one payload of the block at height against an anchor.
func ProveRecord(chain *Sidechain, anchor SidechainAnchor, height uint64, index int) (*RecordProof, error) {
	return sidechain.ProveRecord(chain, anchor, height, index)
}

// VerifyRecord checks a payload against an anchor using its proof.
func VerifyRecord(payload []byte, proof *RecordProof, anchor SidechainAnchor) error {
	return sidechain.VerifyRecord(payload, proof, anchor)
}
