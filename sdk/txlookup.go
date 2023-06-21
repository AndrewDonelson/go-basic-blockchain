package sdk

import (
	"fmt"
	"math/big"
	"strings"
)

// Index is a map of Block Number/Index (Key) and Transaction ID (Value) that is stored in memory and persisted to disk.
// Example 1 => "dbc74a05-703b-49e2-b607-37153ec6ff9e"
// The Block index is a big.Int (8 bytes) and the Transaction ID is a UUID (16 bytes) for a table record size of 24 bytes.
// The last 64k Transactions will be stored in memory for fast lookup. Any Transactions beyond that will be looked up via disk.
// Uses indexCacheSize (defined in const.go) is the size of the block/transaction index cache (1,572,864 bytes or 1.5 MB)
type Index []string // Tx Lookup via BlockID (Key) and TxID (Value)

// IndexEntry is a struct that contains the blockNumber, txID and txHash for a transaction.
// It is used for both searching and returning results.
type IndexEntry struct {
	BlockNumber big.Int
	TxID        string
	TxHash      string
}

// TXLookupManager is a struct that contains the index and methods for manipulating/searching the index to find
// blocks and transactions by either ID or Hash.
type TXLookupManager struct {
	index      Index
	initalized bool
}

// NewTXLookupManager returns a new TXLookupManager instance.
func NewTXLookupManager() *TXLookupManager {
	return &TXLookupManager{
		index:      make(Index, indexCacheSize), // reserve memory for 64k transactions
		initalized: false,                       // this will be true after the first call to Load()
	}
}

// merge combines blockNumber, txID and txHash into a single string seperated by a colon for full text search
func (txlm *TXLookupManager) merge(blockNumber big.Int, txID string, txHash string) string {
	return fmt.Sprintf("%s:%s:%s", blockNumber.String(), txID, txHash)
}

// split splits a merged string into Indexentry object contining the blockNumber, txID and txHash
func (txlm *TXLookupManager) split(merged string) (entry *IndexEntry) {
	entry = &IndexEntry{}
	fmt.Sscanf(merged, "%s:%s:%s", &entry.BlockNumber, &entry.TxID, &entry.TxHash)

	return
}

// Exists tells whether the Index contains entry.
func (txlm *TXLookupManager) exists(entry string) bool {
	for _, n := range txlm.index {
		if entry == n {
			return true
		}
	}
	return false
}

// Initialized returns true if the index has been initialized and is ready for use
func (txlm *TXLookupManager) Initialized() bool {
	return txlm.initalized
}

// Find efficiently search the index to see if thr tx hash exists and if so returns a populated IndexEntry{}
func (txlm *TXLookupManager) Find(indexEntry *IndexEntry) (entry *IndexEntry, err error) {
	// make sure indexEntry has only one of the three fields populated for the search
	if indexEntry.BlockNumber.String() != "" && indexEntry.TxID != "" && indexEntry.TxHash != "" {
		return entry, fmt.Errorf("Find() requires only one of the three fields populated")
	} else if indexEntry.BlockNumber.String() != "" && indexEntry.TxID != "" {
		return entry, fmt.Errorf("Find() requires only one of the three fields populated")
	} else if indexEntry.BlockNumber.String() != "" && indexEntry.TxHash != "" {
		return entry, fmt.Errorf("Find() requires only one of the three fields populated")
	} else if indexEntry.TxID != "" && indexEntry.TxHash != "" {
		return entry, fmt.Errorf("Find() requires only one of the three fields populated")
	}

	find := ""
	if indexEntry.BlockNumber.String() != "" {
		find = fmt.Sprintf("%s:", indexEntry.BlockNumber.String())
	} else if indexEntry.TxID != "" {
		find = fmt.Sprintf(":%s:", indexEntry.TxID)
	} else if indexEntry.TxHash != "" {
		find = fmt.Sprintf(":%s", indexEntry.TxHash)
	}

	if len(find) == 0 {
		return entry, fmt.Errorf("Find() requires one of the three fields populated")
	}

	for _, entry := range txlm.index {
		if strings.Contains(entry, find) {
			return txlm.split(entry), nil
		}
	}

	return entry, fmt.Errorf("Find() failed to find entry")
}

// Set sets the index from BlockchainPersistData loaded from LocalStorage
func (txlm *TXLookupManager) Set(idx *Index) error {
	txlm.index = *idx
	txlm.initalized = true
	return nil
}

// Get returns the index for BlockchainPersistData to save to LocalStorage
func (txlm *TXLookupManager) Get() *Index {
	return &txlm.index
}

// Add adds a new entry to the index
func (txlm *TXLookupManager) Add(block *Block) error {

	for _, tx := range block.Transactions {
		value := txlm.merge(block.Index, tx.GetID(), tx.Hash())

		if !txlm.exists(value) {
			txlm.index = append(txlm.index, value)
		}
	}

	return nil
}
