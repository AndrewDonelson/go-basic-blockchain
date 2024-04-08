// Package sdk is a software development kit for building blockchain applications.
// File sdk/block.go - Block in the blockchain
package sdk

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"time"
)

// Block represents a block in the blockchain. Blocks are persisted to disk as separate JSON files.
// The Block struct contains the following fields:
//
// - Index: The index of the block in the blockchain.
// - Timestamp: The timestamp of when the block was created.
// - Transactions: The list of transactions included in the block.
// - Nonce: A value used in the proof-of-work algorithm to mine the block.
// - Hash: The hash of the block.
// - PreviousHash: The hash of the previous block in the blockchain.
type Block struct {
	Index        big.Int
	Timestamp    time.Time
	Transactions []Transaction
	Nonce        string
	Hash         string
	PreviousHash string
}

// / String returns a string representation of the block, including its index, timestamp, number of transactions, nonce, hash, and previous hash.
func (b *Block) String() string {
	return fmt.Sprintf("Index: %v, Timestamp: %s, Transactions: %d, Nonce: %s, Hash: %s, PreviousHash: %s", b.Index, b.Timestamp.Format(logDateTimeFormat), len(b.Transactions), b.Nonce, b.Hash, b.PreviousHash)
}

// Bytes returns the serialized byte representation of the block.
func (b *Block) Bytes() []byte {
	data, _ := json.Marshal(b)
	return data
}

// GetTransactions returns the transactions in the block that match the given transaction ID. If an ID is provided, it returns a slice containing only the transaction with the matching ID. If no ID is provided, it returns all the transactions in the block.
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

// hash returns the hash of the block as a string. It creates a copy of the block, clears the Hash field, and then calculates the SHA-256 hash of the serialized block data.
func (b *Block) hash() string {
	// make a copy and clear the hash property
	blockCopy := *b
	blockCopy.Hash = ""

	hash := sha256.Sum256(blockCopy.Bytes())
	return hex.EncodeToString(hash[:])
}

// blockExists checks if a block file with the given filename exists.
// It returns true if the file exists, and false otherwise.
func (b *Block) blockExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// save saves the block to disk as a JSON file. It uses the localStorage.Set function to persist the block data, and logs a message with the current time and the block index.
func (b *Block) save() error {
	err := localStorage.Set("block", b)
	if err != nil {
		return err
	}

	fmt.Printf("[%s] Block [%s] saved to disk.\n", time.Now().Format(logDateTimeFormat), b.Index.String())

	return nil
}

// load loads the block from disk. It sets the block's Index property to the provided blockNumber, and then uses the localStorage.Get function to retrieve the block data from disk. If an error occurs during the load, it is returned.
func (b *Block) load(blockNumber big.Int) error {
	b.Index = blockNumber
	err := localStorage.Get("block", b)
	if err != nil {
		return err
	}

	return nil
}
