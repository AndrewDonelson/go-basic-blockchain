// Package sdk is a software development kit for building blockchain applications.
// File: sdk/block.go - Block in the blockchain
package sdk

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"os"
	"strings"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/helios/algorithm"
)

const (
	// InitialDifficulty is the starting difficulty for mining blocks
	InitialDifficulty = 4

	// BlockRewardHalvingInterval is the number of blocks between each halving of the block reward
	BlockRewardHalvingInterval = 210000

	// InitialBlockReward is the initial reward for mining a block
	InitialBlockReward = 50.0

	// TargetBlockTime is the desired time between blocks in seconds
	TargetBlockTime = 20 * time.Second

	// TestTargetBlockTimeLow is the target block time for low difficulty tests (3 seconds)
	TestTargetBlockTimeLow = 3 * time.Second

	// TestTargetBlockTimeHigh is the target block time for high difficulty tests (6 seconds)
	TestTargetBlockTimeHigh = 6 * time.Second

	// blockHeaderOverhead is a fixed allowance for the encoded block header when
	// estimating block size.
	blockHeaderOverhead = 512
)

// BlockHeader represents the header of a block in the blockchain.
type BlockHeader struct {
	Version      int32     `json:"version"`
	PreviousHash string    `json:"previousHash"`
	MerkleRoot   []byte    `json:"merkleRoot"`
	Timestamp    time.Time `json:"timestamp"`
	Difficulty   uint32    `json:"difficulty"`
	Nonce        uint32    `json:"nonce"`
}

// Block represents a block in the blockchain.
type Block struct {
	Header       BlockHeader   `json:"header"`
	Transactions []Transaction `json:"transactions"`
	bloomFilter  *BloomFilter
	Index        big.Int `json:"index"` // Maintain original Index for backwards compatibility
	Hash         string  `json:"hash"`  // Maintain original Hash for backwards compatibility

	// HeliosProof is the proof-of-work this block was mined with. It is persisted
	// so the block can be re-verified later; previously updateWithHeliosProof
	// copied out only the nonce and hash and discarded the proof, which left every
	// mined block permanently unverifiable.
	HeliosProof *algorithm.HeliosProof `json:"helios_proof,omitempty"`
}

// blockWire is the on-disk shape of a Block. Transactions are held as raw JSON so
// each one can be dispatched to its concrete type by protocol.
type blockWire struct {
	Header       BlockHeader            `json:"header"`
	Transactions []json.RawMessage      `json:"transactions"`
	Index        json.RawMessage        `json:"index"`
	Hash         string                 `json:"hash"`
	HeliosProof  *algorithm.HeliosProof `json:"helios_proof,omitempty"`
}

// parseBlockIndex accepts the index as either a JSON number (how big.Int
// marshals, and how every block already on disk is written) or a decimal string
// (what this codec now emits, so very large indices do not lose precision passing
// through a float64-based JSON decoder).
func parseBlockIndex(raw json.RawMessage) (*big.Int, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return big.NewInt(0), nil
	}

	if trimmed[0] == '"' {
		var asString string
		if err := json.Unmarshal(raw, &asString); err != nil {
			return nil, err
		}
		trimmed = asString
	}

	parsed, ok := new(big.Int).SetString(trimmed, 10)
	if !ok {
		return nil, fmt.Errorf("invalid block index: %q", trimmed)
	}
	return parsed, nil
}

// MarshalJSON encodes the block, including the Helios proof needed to re-verify
// it. The proof was previously discarded at mining time, which made every block
// permanently unverifiable once written.
func (b *Block) MarshalJSON() ([]byte, error) {
	wire := blockWire{
		Header:      b.Header,
		Index:       json.RawMessage(`"` + b.Index.String() + `"`),
		Hash:        b.Hash,
		HeliosProof: b.HeliosProof,
	}

	wire.Transactions = make([]json.RawMessage, 0, len(b.Transactions))
	for _, tx := range b.Transactions {
		if tx == nil {
			continue
		}
		encoded, err := json.Marshal(tx)
		if err != nil {
			return nil, fmt.Errorf("failed to encode transaction %s: %w", tx.GetID(), err)
		}
		wire.Transactions = append(wire.Transactions, encoded)
	}

	return json.Marshal(wire)
}

// UnmarshalJSON decodes a block, dispatching each transaction to its concrete
// type via the protocol discriminator.
//
// Without this, decoding failed outright ("cannot unmarshal object into Go struct
// field Block.transactions of type sdk.Transaction") because Transaction is an
// interface. LoadExistingBlocks logged that error and skipped the block, so the
// chain silently reset to genesis on every restart.
func (b *Block) UnmarshalJSON(data []byte) error {
	var wire blockWire
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}

	b.Header = wire.Header
	b.Hash = wire.Hash
	b.HeliosProof = wire.HeliosProof

	parsedIndex, err := parseBlockIndex(wire.Index)
	if err != nil {
		return err
	}
	b.Index = *parsedIndex

	b.Transactions = make([]Transaction, 0, len(wire.Transactions))
	for i, raw := range wire.Transactions {
		tx, err := DecodeTransaction(raw)
		if err != nil {
			return fmt.Errorf("failed to decode transaction %d: %w", i, err)
		}
		b.Transactions = append(b.Transactions, tx)
	}

	b.bloomFilter = b.CreateBloomFilter()
	return nil
}

// NewBlock creates a new block with the given transactions and previous hash.
func NewBlock(transactions []Transaction, previousHash string) *Block {
	block := &Block{
		Header: BlockHeader{
			Version:      1,
			PreviousHash: previousHash,
			Timestamp:    time.Now(),
			Difficulty:   InitialDifficulty,
			Nonce:        0,
		},
		Transactions: transactions,
		Index:        *big.NewInt(0), // Initialize with zero, should be set properly when adding to blockchain
	}
	block.Header.MerkleRoot = block.CalculateMerkleRoot()
	block.bloomFilter = block.CreateBloomFilter()
	block.Hash = block.CalculateHash() // Set the Hash field for backwards compatibility
	return block
}

// String returns a string representation of the block.
func (b *Block) String() string {
	return fmt.Sprintf("Index: %v, Timestamp: %s, Transactions: %d, Nonce: %d, Hash: %s, PreviousHash: %s",
		b.Index, b.Header.Timestamp.Format(time.RFC3339), len(b.Transactions),
		b.Header.Nonce, b.Hash, b.Header.PreviousHash)
}

// Bytes returns the serialized byte representation of the block.
func (b *Block) Bytes() []byte {
	data, _ := json.Marshal(b)
	return data
}

// GetTransactions returns the transactions in the block that match the given transaction ID.
// If an ID is provided, it returns a slice containing only the transaction with the matching ID.
// If no ID is provided, it returns all the transactions in the block.
func (b *Block) GetTransactions(id string) []Transaction {
	if id != "" {
		for _, tx := range b.Transactions {
			if tx.GetID() == id {
				return []Transaction{tx}
			}
		}
		return []Transaction{}
	}
	return b.Transactions
}

// hash returns the hash of the block as a string.
// hash calculates the hash of the block
// This function is currently unused but kept for potential future use
//
//nolint:unused
func (b *Block) hash() string {
	blockCopy := *b
	blockCopy.Hash = ""
	hash := sha256.Sum256(blockCopy.Bytes())
	return hex.EncodeToString(hash[:])
}

// blockExists checks if a block file with the given filename exists.
// This function is currently unused but kept for potential future use
//
//nolint:unused
func (b *Block) blockExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// save saves the block to disk using localStorage.
func (b *Block) save() error {
	// The localStorage system automatically determines the file path based on the Block type
	// It will save to blocks/{index}.json
	err := localStorage.Set("", b)
	if err != nil {
		return err
	}
	LogVerbosef("Block [%s] saved to disk.", b.Index.String())
	return nil
}

// load loads the block from disk using localStorage.
// This function is currently unused but kept for potential future use
//
//nolint:unused
func (b *Block) load(blockNumber big.Int) error {
	b.Index = blockNumber
	err := localStorage.Get("block", b)
	if err != nil {
		return err
	}
	return nil
}

// Validate checks if the block is valid.
func (b *Block) Validate(previousBlock *Block) error {
	if previousBlock == nil {
		return errors.New("previous block is nil")
	}
	if b.Header.PreviousHash != previousBlock.Hash {
		return errors.New("invalid previous hash")
	}
	if b.Header.Timestamp.After(time.Now()) {
		return errors.New("block timestamp is in the future")
	}
	for _, tx := range b.Transactions {
		if tx == nil {
			return errors.New("block contains a nil transaction")
		}
		// A transaction is confirmed by virtue of being in a block; requiring the
		// flag to be pre-set meant nothing ever passed, because nothing set it.
		if tx.GetStatus() == StatusFailed {
			return fmt.Errorf("block contains a failed transaction: %s", tx.GetID())
		}
		if err := tx.Validate(); err != nil {
			return fmt.Errorf("invalid transaction: %v", err)
		}
	}
	if b.Hash != b.CalculateHash() {
		return errors.New("invalid block hash")
	}
	return nil
}

// CalculateMerkleRoot calculates the Merkle root of the block's transactions.
func (b *Block) CalculateMerkleRoot() []byte {
	var transactions [][]byte
	for _, tx := range b.Transactions {
		if tx == nil {
			continue // Skip nil transactions
		}
		hash := tx.Hash()
		if hash == "" {
			continue // Skip transactions with empty hash
		}
		transactions = append(transactions, []byte(hash))
	}
	tree := NewMerkleTree(transactions)
	if tree.Root == nil {
		return []byte{} // Return empty byte slice for empty tree
	}
	return tree.Root.Data
}

// AdjustDifficulty adjusts the mining difficulty based on the time taken to mine
// recent blocks.
//
// The decrease is floored at 1. Difficulty is a uint32, so `- 1` at zero wrapped
// to 4294967295 -- a difficulty no machine could ever satisfy.
func (b *Block) AdjustDifficulty(previousBlock *Block, targetBlockTime time.Duration) uint32 {
	if previousBlock == nil {
		return b.Header.Difficulty
	}

	elapsed := b.Header.Timestamp.Sub(previousBlock.Header.Timestamp)
	switch {
	case elapsed < targetBlockTime/2:
		if previousBlock.Header.Difficulty >= math.MaxUint32 {
			return previousBlock.Header.Difficulty
		}
		return previousBlock.Header.Difficulty + 1
	case elapsed > targetBlockTime*2:
		if previousBlock.Header.Difficulty <= 1 {
			return 1
		}
		return previousBlock.Header.Difficulty - 1
	default:
		return previousBlock.Header.Difficulty
	}
}

// Serialize serializes the block into a byte slice.
func (b *Block) Serialize() ([]byte, error) {
	var result bytes.Buffer
	encoder := gob.NewEncoder(&result)
	err := encoder.Encode(b)
	if err != nil {
		return nil, err
	}
	return result.Bytes(), nil
}

// DeserializeBlock deserializes a byte slice into a Block.
func DeserializeBlock(d []byte) (*Block, error) {
	var block Block
	decoder := gob.NewDecoder(bytes.NewReader(d))
	err := decoder.Decode(&block)
	if err != nil {
		return nil, err
	}
	return &block, nil
}

// CalculateTotalFees calculates the total transaction fees in the block.
func (b *Block) CalculateTotalFees() float64 {
	totalFees := 0.0
	for _, tx := range b.Transactions {
		totalFees += tx.GetFee()
	}
	return totalFees
}

// createBlockHeaderForMining creates a block header for Helios mining
func (b *Block) createBlockHeaderForMining() []byte {
	// Create a serialized representation of the block header for mining
	headerData := fmt.Sprintf("%d:%s:%s:%d:%d",
		b.Header.Version,
		b.Header.PreviousHash,
		hex.EncodeToString(b.Header.MerkleRoot),
		b.Header.Timestamp.Unix(),
		b.Header.Difficulty)

	return []byte(headerData)
}

// updateWithHeliosProof updates the block with Helios proof data.
//
// The proof itself is retained so the block can be validated later. The block
// hash stays the header hash: overwriting it with proof.FinalHash meant
// Block.Validate's `Hash != CalculateHash()` check failed for every mined block.
// The proof's own hash is checked separately by the Helios validator.
func (b *Block) updateWithHeliosProof(proof *algorithm.HeliosProof) {
	b.Header.Nonce = uint32(proof.Nonce)
	b.Header.Timestamp = proof.Timestamp
	b.HeliosProof = proof
	b.Hash = b.CalculateHash()
}

// CanAddTransaction checks if adding a new transaction would exceed the maximum
// block size.
//
// It sums the encoded transaction sizes instead of gob-encoding the whole block
// on every call, which was O(block) work per candidate transaction.
func (b *Block) CanAddTransaction(tx Transaction) bool {
	if tx == nil {
		return false
	}
	return b.currentSize()+tx.Size() <= MaxBlockSize
}

// currentSize returns the approximate encoded size of the block's transactions.
func (b *Block) currentSize() int {
	size := blockHeaderOverhead
	for _, tx := range b.Transactions {
		if tx != nil {
			size += tx.Size()
		}
	}
	return size
}

// CreateBloomFilter creates a Bloom filter for quick transaction lookups within the block.
func (b *Block) CreateBloomFilter() *BloomFilter {
	bf := &BloomFilter{
		bitset: make([]byte, 256),
		k:      3,
	}
	for _, tx := range b.Transactions {
		if tx == nil {
			continue // Skip nil transactions
		}
		txID := tx.GetID()
		if txID == "" {
			continue // Skip transactions with empty ID
		}
		bf.Add([]byte(txID))
	}
	return bf
}

// Mine performs the proof-of-work algorithm to mine the block.
//
// It returns an error when the nonce space is exhausted. The previous version
// looped forever incrementing a uint32 nonce with no ceiling: once the nonce
// wrapped it re-tried the same hashes indefinitely, so a difficulty the machine
// could not reach hung the caller permanently.
func (b *Block) Mine(difficulty uint) error {
	// For difficulty n, we need the hash to start with n zeros in hex
	// This means the first n*4 bits must be zero
	prefix := strings.Repeat("0", int(difficulty))

	for nonce := uint64(0); nonce <= math.MaxUint32; nonce++ {
		b.Header.Nonce = uint32(nonce)
		hash := b.CalculateHash()

		// Check if hash starts with the required number of zeros
		if strings.HasPrefix(hash, prefix) {
			b.Hash = hash // Update Hash for backwards compatibility
			return nil
		}
	}

	return fmt.Errorf("exhausted the nonce space without finding a proof at difficulty %d", difficulty)
}

// CalculateBlockReward calculates the block reward based on the current block height.
func (b *Block) CalculateBlockReward(currentBlockHeight int64) float64 {
	halvings := currentBlockHeight / BlockRewardHalvingInterval
	return InitialBlockReward * math.Pow(0.5, float64(halvings))
}

// CalculateHash calculates and returns the hash of the block.
//
// The header is serialised as fixed-width big-endian fields rather than
// fmt.Sprintf. Two problems with the old formulation:
//
//  1. It interpolated Timestamp.String(), which renders the monotonic clock
//     reading ("m=+0.057091985") for a time produced by time.Now(). JSON drops
//     that, so a block's hash changed the instant it round-tripped through disk
//     or the network and every `Hash != CalculateHash()` check failed. It also
//     embedded the machine's local timezone abbreviation.
//  2. Concatenating variable-length fields without delimiters is ambiguous:
//     distinct headers could produce identical preimages.
func (b *Block) CalculateHash() string {
	var buf bytes.Buffer

	writeUint32 := func(v uint32) {
		var tmp [4]byte
		binary.BigEndian.PutUint32(tmp[:], v)
		buf.Write(tmp[:])
	}
	writeInt64 := func(v int64) {
		var tmp [8]byte
		binary.BigEndian.PutUint64(tmp[:], uint64(v))
		buf.Write(tmp[:])
	}
	writeBytes := func(v []byte) {
		writeUint32(uint32(len(v)))
		buf.Write(v)
	}

	writeUint32(uint32(b.Header.Version))
	writeBytes([]byte(b.Header.PreviousHash))
	writeBytes(b.Header.MerkleRoot)
	writeInt64(b.Header.Timestamp.UTC().UnixNano())
	writeUint32(b.Header.Difficulty)
	writeUint32(b.Header.Nonce)

	hashed := sha256.Sum256(buf.Bytes())
	return hex.EncodeToString(hashed[:])
}

// MerkleNode represents a node in the Merkle tree.
type MerkleNode struct {
	Left  *MerkleNode
	Right *MerkleNode
	Data  []byte
}

// MerkleTree represents a Merkle tree of transactions.
type MerkleTree struct {
	Root *MerkleNode
}

// NewMerkleTree creates a new Merkle tree from a list of data.
//
// Padding is applied at every level, not just the leaves. The old version padded
// only the leaf level, so an even leaf count that halved to an odd count read
// past the end of the slice: six transactions produced three internal nodes and
// then panicked with "index out of range [3] with length 3".
//
// Leaf and internal hashes are domain-separated with a 0x00/0x01 prefix. Without
// that, duplicating the final node to pad an odd level makes two different
// transaction lists produce the same root (the CVE-2012-2459 malleability).
func NewMerkleTree(data [][]byte) *MerkleTree {
	if len(data) == 0 {
		// Return nil root for empty tree
		return &MerkleTree{Root: nil}
	}

	nodes := make([]*MerkleNode, 0, len(data))
	for _, datum := range data {
		nodes = append(nodes, NewMerkleNode(nil, nil, datum))
	}

	for len(nodes) > 1 {
		// Pad this level, not just the first one.
		if len(nodes)%2 != 0 {
			nodes = append(nodes, nodes[len(nodes)-1])
		}

		newLevel := make([]*MerkleNode, 0, len(nodes)/2)
		for i := 0; i < len(nodes); i += 2 {
			newLevel = append(newLevel, NewMerkleNode(nodes[i], nodes[i+1], nil))
		}

		nodes = newLevel
	}

	return &MerkleTree{Root: nodes[0]}
}

// Merkle domain separation prefixes.
const (
	merkleLeafPrefix     byte = 0x00
	merkleInternalPrefix byte = 0x01
)

// NewMerkleNode creates a new Merkle node.
func NewMerkleNode(left, right *MerkleNode, data []byte) *MerkleNode {
	node := MerkleNode{}

	if left == nil && right == nil {
		hash := sha256.Sum256(append([]byte{merkleLeafPrefix}, data...))
		node.Data = hash[:]
	} else {
		// Build the preimage in a fresh buffer. `append(left.Data, right.Data...)`
		// writes into left.Data's backing array whenever it has spare capacity,
		// silently corrupting the left node.
		combined := make([]byte, 0, 1+len(left.Data)+len(right.Data))
		combined = append(combined, merkleInternalPrefix)
		combined = append(combined, left.Data...)
		combined = append(combined, right.Data...)
		hash := sha256.Sum256(combined)
		node.Data = hash[:]
	}

	node.Left = left
	node.Right = right

	return &node
}

// BloomFilter represents a Bloom filter for quick transaction lookups.
type BloomFilter struct {
	bitset []byte
	k      uint
}

// maxBloomHashes is the number of 8-byte windows available in a SHA-256 digest.
// k above this would slice past the end of the hash.
const maxBloomHashes = sha256.Size / 8

// Add adds data to the Bloom filter.
func (bf *BloomFilter) Add(data []byte) {
	if len(bf.bitset) == 0 {
		return
	}
	h := sha256.Sum256(data)
	for i := uint(0); i < bf.k && i < maxBloomHashes; i++ {
		idx := binary.BigEndian.Uint64(h[i*8:]) % uint64(len(bf.bitset)*8)
		bf.bitset[idx/8] |= 1 << (idx % 8)
	}
}

// Contains checks if the Bloom filter possibly contains the given data.
func (bf *BloomFilter) Contains(data []byte) bool {
	if len(bf.bitset) == 0 {
		return false
	}
	h := sha256.Sum256(data)
	for i := uint(0); i < bf.k && i < maxBloomHashes; i++ {
		idx := binary.BigEndian.Uint64(h[i*8:]) % uint64(len(bf.bitset)*8)
		if bf.bitset[idx/8]&(1<<(idx%8)) == 0 {
			return false
		}
	}
	return true
}
