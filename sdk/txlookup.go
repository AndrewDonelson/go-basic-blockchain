package sdk

import (
	"fmt"
	"math/big"
	"strings"
	"sync"
)

// Index is a map of Block Number/Index (Key) and Transaction ID (Value) that is stored in memory and persisted to disk.
// Example 1 => "dbc74a05-703b-49e2-b607-37153ec6ff9e"
// The Block index is a big.Int (8 bytes) and the Transaction ID is a UUID (16 bytes) for a table record size of 24 bytes.
// The last 64k Transactions will be stored in memory for fast lookup. Any Transactions beyond that will be looked up via disk.
// Uses indexCacheSize (defined in const.go) is the size of the block/transaction index cache (1,572,864 bytes or 1.5 MB)
type Index []string // Tx Lookup via BlockID (Key) and TxID (Value)

// FIFOQueue is a bounded, insertion-ordered set of index entries.
//
// It is backed by a map plus a ring buffer rather than a plain slice. The old
// implementation called a linear Exists() scan on every Enqueue, making index
// construction O(n^2) at a capacity of 65,536, and Dequeue did
// `q.queue = q.queue[1:]`, which never releases the head of the backing array --
// so memory grew without bound despite the nominal capacity.
type FIFOQueue struct {
	mu       sync.RWMutex
	entries  []string
	position map[string]int
	head     int
	count    int
	capacity int
}

// NewFIFOQueue creates a new FIFO queue with the specified capacity.
func NewFIFOQueue(capacity int) *FIFOQueue {
	// if no capacity is specified, use the default
	if capacity <= 0 {
		capacity = indexCacheSize
	}

	return &FIFOQueue{
		entries:  make([]string, capacity),
		position: make(map[string]int, capacity),
		capacity: capacity,
	}
}

// Enqueue adds an element to the back of the queue in O(1).
// If the queue is at capacity, the oldest element is evicted first.
func (q *FIFOQueue) Enqueue(element string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if element == "" {
		return
	}
	if _, exists := q.position[element]; exists {
		// Already present. The old code evicted the oldest entry *before* this
		// check, so re-adding an existing element silently dropped an unrelated one.
		return
	}

	if q.count == q.capacity {
		q.dequeueLocked()
	}

	idx := (q.head + q.count) % q.capacity
	q.entries[idx] = element
	q.position[element] = idx
	q.count++
}

// Dequeue removes and returns the oldest element, or "" when empty.
func (q *FIFOQueue) Dequeue() string {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.dequeueLocked()
}

func (q *FIFOQueue) dequeueLocked() string {
	if q.count == 0 {
		return ""
	}

	element := q.entries[q.head]
	q.entries[q.head] = ""
	delete(q.position, element)
	q.head = (q.head + 1) % q.capacity
	q.count--
	return element
}

// Len returns the current number of elements in the queue.
func (q *FIFOQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.count
}

// IsEmpty checks if the queue is empty.
func (q *FIFOQueue) IsEmpty() bool {
	return q.Len() == 0
}

// Exists reports whether the queue contains an exact entry, in O(1).
func (q *FIFOQueue) Exists(entry string) bool {
	q.mu.RLock()
	defer q.mu.RUnlock()

	_, ok := q.position[entry]
	return ok
}

// Find returns the first entry containing s, or "" if there is no match.
func (q *FIFOQueue) Find(s string) string {
	q.mu.RLock()
	defer q.mu.RUnlock()

	for i := 0; i < q.count; i++ {
		entry := q.entries[(q.head+i)%q.capacity]
		if entry != "" && strings.Contains(entry, s) {
			return entry
		}
	}
	return ""
}

// Get returns a snapshot of the queue in insertion order.
func (q *FIFOQueue) Get() *Index {
	q.mu.RLock()
	defer q.mu.RUnlock()

	out := make(Index, 0, q.count)
	for i := 0; i < q.count; i++ {
		out = append(out, q.entries[(q.head+i)%q.capacity])
	}
	return &out
}

// Set replaces the queue contents.
func (q *FIFOQueue) Set(index *Index) {
	if index == nil {
		return
	}

	q.mu.Lock()
	capacity := q.capacity
	q.mu.Unlock()

	entries := make([]string, capacity)
	position := make(map[string]int, capacity)

	count := 0
	for _, entry := range *index {
		if entry == "" {
			continue
		}
		if _, dup := position[entry]; dup {
			continue
		}
		if count == capacity {
			// Keep the most recent `capacity` entries.
			delete(position, entries[0])
			copy(entries, entries[1:])
			count--
		}
		entries[count] = entry
		position[entry] = count
		count++
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	q.entries = entries
	q.position = position
	q.head = 0
	q.count = count
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
	index       *FIFOQueue
	initialized bool
}

// NewTXLookupManager returns a new TXLookupManager instance.
func NewTXLookupManager() *TXLookupManager {
	return &TXLookupManager{
		index:       NewFIFOQueue(0), // reserve memory for 64k transactions
		initialized: false,           // this will be true after the first call to Load()
	}
}

// merge combines blockNumber, txID and txHash into a single string separated by a colon for full text search
func (txlm *TXLookupManager) merge(blockNumber big.Int, txID string, txHash string) string {
	return fmt.Sprintf("%s:%s:%s", blockNumber.String(), txID, txHash)
}

// split parses a merged "blockNumber:txID:txHash" string.
//
// This used to be fmt.Sscanf(merged, "%s:%s:%s", ...) with both return values
// discarded. %s in Sscanf consumes up to whitespace, not up to a colon, so the
// first verb swallowed the whole string and the remaining two never matched --
// every IndexEntry it produced was empty.
func (txlm *TXLookupManager) split(merged string) (*IndexEntry, error) {
	parts := strings.SplitN(merged, ":", 3)
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed index entry: %q", merged)
	}

	blockNumber, ok := new(big.Int).SetString(parts[0], 10)
	if !ok {
		return nil, fmt.Errorf("malformed block number in index entry: %q", parts[0])
	}

	return &IndexEntry{
		BlockNumber: *blockNumber,
		TxID:        parts[1],
		TxHash:      parts[2],
	}, nil
}

// Exists tells whether the Index contains entry.
// This function is currently unused but kept for potential future use
//
//nolint:unused
func (txlm *TXLookupManager) exists(entry string) bool {
	return txlm.index.Exists(entry)
}

// Initialized returns true if the index has been initialized and is ready for use
func (txlm *TXLookupManager) Initialized() bool {
	return txlm.initialized
}

// Find searches the index for an entry matching exactly one populated field of
// the supplied IndexEntry.
//
// The presence test is now explicit. It used to be
// `indexEntry.BlockNumber.String() != ""`, but a zero-valued big.Int stringifies
// to "0", never "" -- so BlockNumber always looked populated and every lookup by
// TxID or TxHash was rejected with "requires only one of the three fields
// populated". The whole method was unreachable in practice.
func (txlm *TXLookupManager) Find(indexEntry *IndexEntry) (*IndexEntry, error) {
	if indexEntry == nil {
		return nil, fmt.Errorf("Find() requires an index entry")
	}

	hasBlock := indexEntry.BlockNumber.Sign() > 0
	hasTxID := indexEntry.TxID != ""
	hasHash := indexEntry.TxHash != ""

	populated := 0
	for _, set := range []bool{hasBlock, hasTxID, hasHash} {
		if set {
			populated++
		}
	}

	switch {
	case populated == 0:
		return nil, fmt.Errorf("Find() requires one of the three fields populated")
	case populated > 1:
		return nil, fmt.Errorf("Find() requires only one of the three fields populated")
	}

	var find string
	switch {
	case hasBlock:
		find = indexEntry.BlockNumber.String() + ":"
	case hasTxID:
		find = ":" + indexEntry.TxID + ":"
	default:
		find = ":" + indexEntry.TxHash
	}

	found := txlm.index.Find(find)
	if found == "" {
		return nil, fmt.Errorf("Find() failed to find entry")
	}

	return txlm.split(found)
}

// Set sets the index from BlockchainPersistData loaded from LocalStorage
func (txlm *TXLookupManager) Set(idx *Index) error {
	if idx == nil {
		return nil // or return an error if you prefer
	}
	txlm.index.Set(idx)
	txlm.initialized = true
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
