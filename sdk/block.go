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

// Block is a block in the blockchain. Blocks are persisted to disk as seperate JSON files.
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

// Bytes returns the serialized byte representation of the transaction.
func (b *Block) Bytes() []byte {
	data, _ := json.Marshal(b)
	return data
}

// Hash returns the hash of the transaction as a string.
func (b *Block) hash() string {
	// make a copy and clear the hash property
	blockCopy := *b
	blockCopy.Hash = ""

	hash := sha256.Sum256(blockCopy.Bytes())
	return hex.EncodeToString(hash[:])
}

func (b *Block) blockExists(filename string) bool {
	_, err := os.Stat(filename)
	return !os.IsNotExist(err)
}

// save saves the block to disk as a JSON file.
func (b *Block) save() error {
	err := localStorage.Set("block", b)
	if err != nil {
		return err
	}

	// filename := fmt.Sprintf("%s/%s.json", blockFolder, b.Index.String())
	// file, err := os.Create(filename)
	// if err != nil {
	// 	return err
	// }
	// defer file.Close()
	// enc := json.NewEncoder(file)
	// enc.SetIndent("", " ")
	// if err := enc.Encode(b); err != nil {
	// 	return err
	// }

	fmt.Printf("[%s] Block [%s] saved to disk.\n", time.Now().Format(logDateTimeFormat), b.Index.String())

	return nil
}

// load loads the block from disk.
func (b *Block) load(blockNumber big.Int) error {
	b.Index = blockNumber
	err := localStorage.Get("block", b)
	if err != nil {
		return err
	}

	// if b.blockExists(file) {
	// 	blockFile, err := os.Open(file)
	// 	if err != nil {
	// 		return err
	// 	}
	// 	defer blockFile.Close()
	// 	dec := json.NewDecoder(blockFile)
	// 	if err := dec.Decode(b); err != nil {
	// 		return err
	// 	}
	// }
	return nil
}
