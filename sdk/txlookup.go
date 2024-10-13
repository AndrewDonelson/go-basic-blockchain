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

// FIFOQueue represents a FIFO queue with a maximum capacity.
type FIFOQueue struct {
	queue    Index
	capacity int
}

// NewFIFOQueue creates a new FIFO queue with the specified capacity.
func NewFIFOQueue(capacity int) *FIFOQueue {

	// if not capacity is specified, use the default
	if capacity <= 0 {
		capacity = indexCacheSize
	}

	return &FIFOQueue{
		queue:    make(Index, 0, capacity),
		capacity: capacity,
	}
}

// Enqueue adds an element to the back of the queue.
// If the queue is already at its maximum capacity, the oldest element is removed.
func (q *FIFOQueue) Enqueue(element string) {
	if len(q.queue) == q.capacity {
		q.Dequeue()
	}

	if !q.Exists(element) {
		q.queue = append(q.queue, element)
	}
}

// Dequeue removes and returns the oldest element from the front of the queue.
// If the queue is empty, an empty string is returned.
func (q *FIFOQueue) Dequeue() string {
	if len(q.queue) == 0 {
		return ""
	}

	element := q.queue[0]
	q.queue = q.queue[1:]
	return element
}

// Len returns the current number of elements in the queue.
func (q *FIFOQueue) Len() int {
	return len(q.queue)
}

// IsEmpty checks if the queue is empty.
func (q *FIFOQueue) IsEmpty() bool {
	return len(q.queue) == 0
}

// Exists tells whether the Index contains entry.
func (q *FIFOQueue) Exists(entry string) bool {
	for _, n := range q.queue {
		if len(n) > 0 && entry == n {
			return true
		}
	}

	return false
}

// Find returns the first entry in the Index that contains the string s, or an empty string if no match is found.
func (q *FIFOQueue) Find(s string) string {
	for _, entry := range q.queue {
		if len(entry) > 0 {
			if strings.Contains(entry, s) {
				return entry
			}
		}
	}
	return ""
}

// Get returns the entire Index.
func (q *FIFOQueue) Get() *Index {
	return &q.queue
}

// Set sets the entire Index.
func (q *FIFOQueue) Set(index *Index) {
	q.queue = *index
}

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
	index      *FIFOQueue
	initalized bool
}

// NewTXLookupManager returns a new TXLookupManager instance.
func NewTXLookupManager() *TXLookupManager {
	return &TXLookupManager{
		index:      NewFIFOQueue(0), // reserve memory for 64k transactions
		initalized: false,           // this will be true after the first call to Load()
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
	return txlm.index.Exists(entry)
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

	found := txlm.index.Find(find)
	if len(found) > 0 {
		return txlm.split(found), nil
	}

	return entry, fmt.Errorf("Find() failed to find entry")
}

// Set sets the index from BlockchainPersistData loaded from LocalStorage
func (txlm *TXLookupManager) Set(idx *Index) error {
	if idx == nil {
		return nil // or return an error if you prefer
	}
	txlm.index.Set(idx)
	txlm.initalized = true
	return nil
}

// Get returns the index for BlockchainPersistData to save to LocalStorage
func (txlm *TXLookupManager) Get() *Index {
	return txlm.index.Get()
}

// Add adds a new entry to the index
func (txlm *TXLookupManager) Add(block *Block) error {

	for _, tx := range block.Transactions {

		// make sure the tx has a hash
		if len(tx.GetHash()) == 0 {
			tx.Hash()
		}

		// add to the FIFO queue
		txlm.index.Enqueue(txlm.merge(block.Index, tx.GetID(), tx.GetHash()))
	}

	return nil
}
