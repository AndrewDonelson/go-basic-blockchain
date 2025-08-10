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
	"log"
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
	log.Printf("Block [%s] saved to disk.\n", b.Index.String())
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
	if b.Header.PreviousHash != previousBlock.Hash {
		return errors.New("invalid previous hash")
	}
	if b.Header.Timestamp.After(time.Now()) {
		return errors.New("block timestamp is in the future")
	}
	for _, tx := range b.Transactions {
		if tx.GetStatus() != StatusConfirmed {
			return fmt.Errorf("invalid transaction status: %v", tx.GetStatus())
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

// AdjustDifficulty adjusts the mining difficulty based on the time taken to mine recent blocks.
func (b *Block) AdjustDifficulty(previousBlock *Block, targetBlockTime time.Duration) uint32 {
	if b.Header.Timestamp.Sub(previousBlock.Header.Timestamp) < targetBlockTime/2 {
		return previousBlock.Header.Difficulty + 1
	} else if b.Header.Timestamp.Sub(previousBlock.Header.Timestamp) > targetBlockTime*2 {
		return previousBlock.Header.Difficulty - 1
	}
	return previousBlock.Header.Difficulty
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

// updateWithHeliosProof updates the block with Helios proof data
func (b *Block) updateWithHeliosProof(proof *algorithm.HeliosProof) {
	// Update block with Helios proof information
	b.Header.Nonce = uint32(proof.Nonce)
	b.Header.Timestamp = proof.Timestamp
	b.Hash = proof.FinalHash

	// Store Helios proof data in block (you might want to add a field for this)
	// For now, we'll just update the hash
}

// CanAddTransaction checks if adding a new transaction would exceed the maximum block size.
func (b *Block) CanAddTransaction(tx Transaction) bool {
	blockSize, _ := b.Serialize()
	return len(blockSize)+tx.Size() <= MaxBlockSize
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
func (b *Block) Mine(difficulty uint) {
	// For difficulty n, we need the hash to start with n zeros in hex
	// This means the first n*4 bits must be zero
	prefix := strings.Repeat("0", int(difficulty))

	for {
		hash := b.CalculateHash()

		// Check if hash starts with the required number of zeros
		if strings.HasPrefix(hash, prefix) {
			b.Hash = hash // Update Hash for backwards compatibility
			return
		}
		b.Header.Nonce++
	}
}

// CalculateBlockReward calculates the block reward based on the current block height.
func (b *Block) CalculateBlockReward(currentBlockHeight int64) float64 {
	halvings := currentBlockHeight / BlockRewardHalvingInterval
	return InitialBlockReward * math.Pow(0.5, float64(halvings))
}

// CalculateHash calculates and returns the hash of the block.
func (b *Block) CalculateHash() string {
	record := fmt.Sprintf("%d%s%x%s%d%d",
		b.Header.Version,
		b.Header.PreviousHash,
		b.Header.MerkleRoot,
		b.Header.Timestamp.String(),
		b.Header.Difficulty,
		b.Header.Nonce)
	h := sha256.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
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
func NewMerkleTree(data [][]byte) *MerkleTree {
	var nodes []*MerkleNode

	if len(data) == 0 {
		// Return nil root for empty tree
		return &MerkleTree{Root: nil}
	}

	if len(data)%2 != 0 {
		data = append(data, data[len(data)-1])
	}

	for _, datum := range data {
		node := NewMerkleNode(nil, nil, datum)
		nodes = append(nodes, node)
	}

	for len(nodes) > 1 {
		var newLevel []*MerkleNode

		for i := 0; i < len(nodes); i += 2 {
			node := NewMerkleNode(nodes[i], nodes[i+1], nil)
			newLevel = append(newLevel, node)
		}

		nodes = newLevel
	}

	return &MerkleTree{Root: nodes[0]}
}

// NewMerkleNode creates a new Merkle node.
func NewMerkleNode(left, right *MerkleNode, data []byte) *MerkleNode {
	node := MerkleNode{}

	if left == nil && right == nil {
		hash := sha256.Sum256(data)
		node.Data = hash[:]
	} else {
		prevHashes := append(left.Data, right.Data...)
		hash := sha256.Sum256(prevHashes)
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

// Add adds data to the Bloom filter.
func (bf *BloomFilter) Add(data []byte) {
	h := sha256.Sum256(data)
	for i := uint(0); i < bf.k; i++ {
		idx := binary.BigEndian.Uint64(h[i*8:]) % uint64(len(bf.bitset)*8)
		bf.bitset[idx/8] |= 1 << (idx % 8)
	}
}

// Contains checks if the Bloom filter possibly contains the given data.
func (bf *BloomFilter) Contains(data []byte) bool {
	h := sha256.Sum256(data)
	for i := uint(0); i < bf.k; i++ {
		idx := binary.BigEndian.Uint64(h[i*8:]) % uint64(len(bf.bitset)*8)
		if bf.bitset[idx/8]&(1<<(idx%8)) == 0 {
			return false
		}
	}
	return true
}
