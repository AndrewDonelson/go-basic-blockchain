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

// Block is a block in the blockchain.
type Block struct {
	Index        big.Int
	Timestamp    time.Time
	Transactions []Transaction
	Nonce        string
	Hash         string
	PreviousHash string
}

// String returns a string representation of the block.
func (b *Block) String() string {
	return fmt.Sprintf("Index: %v, Timestamp: %s, Transactions: %d, Nonce: %s, Hash: %s, PreviousHash: %s", b.Index, b.Timestamp.Format(logDateTimeFormat), len(b.Transactions), b.Nonce, b.Hash, b.PreviousHash)
}

// calculateHash calculates the hash of the block.
func (b *Block) calculateHash() string {
	// Convert the block to a string
	blockString := fmt.Sprintf("%v%s%s%s%s", b.Index, b.Timestamp.Format(logDateTimeFormat), b.Transactions, b.Nonce, b.PreviousHash)

	// Hash the string
	hash := sha256.Sum256([]byte(blockString))

	// Return the hash as a string

	return hex.EncodeToString(hash[:])
}

func (b *Block) blockExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// save saves the block to disk as a JSON file.
func (b *Block) save() error {
	filename := fmt.Sprintf("%s/%019v.json", blockFolder, b.Index)
	file, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer file.Close()
	enc := json.NewEncoder(file)
	enc.SetIndent("", " ")
	if err := enc.Encode(b); err != nil {
		return err
	}

	fmt.Printf("[%s] Block [%v] saved to disk.\n", time.Now().Format(logDateTimeFormat), b.Index)

	return nil
}

// load loads the block from disk.
func (b *Block) load(file string) error {
	if b.blockExists(file) {
		blockFile, err := os.Open(file)
		if err != nil {
			return err
		}
		defer blockFile.Close()
		dec := json.NewDecoder(blockFile)
		if err := dec.Decode(b); err != nil {
			return err
		}
	}
	return nil
}
