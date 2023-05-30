package sdk

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"
)

// Block is a block in the blockchain.
type Block struct {
	Index        int
	Timestamp    time.Time
	Transactions []Transaction
	Nonce        string
	Hash         string
	PreviousHash string
}

func (b *Block) String() string {
	return fmt.Sprintf("Index: %d, Timestamp: %s, Transactions: %d, Nonce: %s, Hash: %s, PreviousHash: %s", b.Index, b.Timestamp.Format(logDateTimeFormat), len(b.Transactions), b.Nonce, b.Hash, b.PreviousHash)
}

func (b *Block) calculateHash() string {
	// Convert the block to a string
	blockString := fmt.Sprintf("%d%s%s%s%s", b.Index, b.Timestamp.Format(logDateTimeFormat), b.Transactions, b.Nonce, b.PreviousHash)

	// Hash the string
	hash := sha256.Sum256([]byte(blockString))

	// Return the hash as a string

	return hex.EncodeToString(hash[:])
}

func (b *Block) save() error {
	filename := fmt.Sprintf("%s/%010d.json", dataFolder, b.Index)
	file, _ := json.MarshalIndent(b, "", " ")

	_ = ioutil.WriteFile(filename, file, 0644)
	fmt.Printf("[%s] Block [%d] saved to disk.\n", time.Now().Format(logDateTimeFormat), b.Index)

	return nil
}

func (b *Block) load(file string) error {
	blockData, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	json.Unmarshal(blockData, &b)

	return nil
}
