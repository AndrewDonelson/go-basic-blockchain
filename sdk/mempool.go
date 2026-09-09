// Package sdk is a software development kit for building blockchain applications.
// File sdk/mempool.go - Mempool admission and block selection policy.
package sdk

import (
	"errors"
	"sort"
)

const (
	// defaultMaxMempoolTxs bounds how many transactions may wait in the mempool.
	//
	// Without a cap the queue is an unbounded, attacker-controlled slice: anyone
	// can post cheap transactions faster than blocks are mined and grow it until
	// the node runs out of memory. The cap alone is not enough, which is why
	// admission is fee-competitive -- see admitToMempoolLocked.
	defaultMaxMempoolTxs = 5000
)

// ErrMempoolFull is returned when a transaction cannot be admitted because the
// mempool is at capacity and the transaction does not outbid anything in it.
var ErrMempoolFull = errors.New("mempool is full and the transaction does not outbid its contents")

// txFeeRate returns a transaction's fee per byte.
//
// Fee per byte, not raw fee, is what a miner actually cares about: a 200-byte
// transaction paying 1 coin is worth more per unit of scarce block space than a
// 20,000-byte one paying 5. Ordering by raw fee lets a large transaction crowd
// out several small ones that together pay more.
func txFeeRate(tx Transaction) float64 {
	if tx == nil {
		return 0
	}
	size := tx.Size()
	if size <= 0 {
		// A zero size would divide by zero and rank the transaction infinitely
		// valuable, which is exactly what a crafted transaction would aim for.
		size = 1
	}
	return tx.GetFee() / float64(size)
}

// outranks reports whether a should be mined before b.
//
// Priority is consulted first, which is what finally makes SetPriority
// load-bearing: it was stored on every transaction and never read by anything.
func outranks(a, b Transaction) bool {
	if a.GetPriority() != b.GetPriority() {
		return a.GetPriority() > b.GetPriority()
	}
	return txFeeRate(a) > txFeeRate(b)
}

// mempoolCapacity returns the configured mempool limit.
func (bc *Blockchain) mempoolCapacity() int {
	if bc.cfg != nil && bc.cfg.MaxMempoolTxs > 0 {
		return bc.cfg.MaxMempoolTxs
	}
	return defaultMaxMempoolTxs
}

// admitToMempoolLocked adds a transaction to the mempool, enforcing the size cap.
//
// When the mempool is full the cheapest resident transaction is evicted to make
// room -- but only if the newcomer actually outbids it. Evicting unconditionally
// would let an attacker flush everyone else's transactions out with a stream of
// zero-fee ones; refusing unconditionally would let early cheap transactions
// hold the mempool hostage against later paying ones.
//
// The caller must hold bc.mux.
func (bc *Blockchain) admitToMempoolLocked(tx Transaction) error {
	if tx == nil {
		return errors.New("cannot admit a nil transaction")
	}

	capacity := bc.mempoolCapacity()
	if len(bc.TransactionQueue) < capacity {
		bc.TransactionQueue = append(bc.TransactionQueue, tx)
		return nil
	}

	cheapest := -1
	for i, queued := range bc.TransactionQueue {
		if queued == nil {
			cheapest = i
			break
		}
		if cheapest == -1 || outranks(bc.TransactionQueue[cheapest], queued) {
			cheapest = i
		}
	}

	if cheapest == -1 {
		return ErrMempoolFull
	}
	if resident := bc.TransactionQueue[cheapest]; resident != nil && !outranks(tx, resident) {
		return ErrMempoolFull
	}

	evicted := bc.TransactionQueue[cheapest]
	bc.TransactionQueue = append(bc.TransactionQueue[:cheapest], bc.TransactionQueue[cheapest+1:]...)
	bc.TransactionQueue = append(bc.TransactionQueue, tx)

	if evicted != nil {
		bc.Metrics().Inc("mempool_evicted")
		LogVerbosef("Evicted transaction %s (fee rate %.8f) to admit %s (fee rate %.8f)",
			evicted.GetID(), txFeeRate(evicted), tx.GetID(), txFeeRate(tx))
	}
	return nil
}

// selectBlockTransactionsLocked chooses the transactions for the next block and
// removes exactly those from the mempool.
//
// Two things matter here. First, the block is bounded by MaxBlockSize:
// CanAddTransaction and MaxBlockSize both existed but createNewBlock drained the
// entire mempool into a block regardless, so blocks were unbounded and a large
// enough one could not be relayed or reloaded. Second, whatever does not fit
// stays queued rather than being dropped -- it is simply not this block's.
//
// The caller must hold bc.mux.
func (bc *Blockchain) selectBlockTransactionsLocked() []Transaction {
	if len(bc.TransactionQueue) == 0 {
		return nil
	}

	// Rank a copy by index so the mempool's own order is untouched and ties keep
	// arrival order, which makes block contents reproducible.
	order := make([]int, len(bc.TransactionQueue))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := bc.TransactionQueue[order[i]], bc.TransactionQueue[order[j]]
		if a == nil || b == nil {
			return b == nil && a != nil
		}
		return outranks(a, b)
	})

	selected := make([]Transaction, 0, len(order))
	chosen := make(map[int]struct{}, len(order))
	size := blockHeaderOverhead

	for _, idx := range order {
		tx := bc.TransactionQueue[idx]
		if tx == nil {
			// A nil entry is unusable; drop it rather than carrying it forever.
			chosen[idx] = struct{}{}
			continue
		}
		txSize := tx.Size()
		if size+txSize > MaxBlockSize {
			// Keep going rather than stopping: a smaller transaction further down
			// the ranking may still fit in the space this one cannot use.
			continue
		}
		size += txSize
		selected = append(selected, tx)
		chosen[idx] = struct{}{}
	}

	remaining := bc.TransactionQueue[:0]
	for i, tx := range bc.TransactionQueue {
		if _, taken := chosen[i]; taken {
			continue
		}
		remaining = append(remaining, tx)
	}
	// Clear the tail so the removed transactions are not kept alive by the
	// backing array.
	for i := len(remaining); i < len(bc.TransactionQueue); i++ {
		bc.TransactionQueue[i] = nil
	}
	bc.TransactionQueue = remaining

	return selected
}

// trimMempoolLocked drops the lowest-ranked transactions until the mempool is
// back within capacity.
//
// Requeue paths -- a failed mining attempt, or a reorg returning transactions
// from orphaned blocks -- reinsert transactions that were already admitted
// earlier, so they legitimately bypass admission control. They must not,
// however, be able to leave the mempool permanently over its bound.
//
// The caller must hold bc.mux.
func (bc *Blockchain) trimMempoolLocked() {
	capacity := bc.mempoolCapacity()
	if len(bc.TransactionQueue) <= capacity {
		return
	}

	order := make([]int, len(bc.TransactionQueue))
	for i := range order {
		order[i] = i
	}
	sort.SliceStable(order, func(i, j int) bool {
		a, b := bc.TransactionQueue[order[i]], bc.TransactionQueue[order[j]]
		if a == nil || b == nil {
			return b == nil && a != nil
		}
		return outranks(a, b)
	})

	keep := make(map[int]struct{}, capacity)
	for _, idx := range order[:capacity] {
		keep[idx] = struct{}{}
	}

	remaining := bc.TransactionQueue[:0]
	dropped := 0
	for i, tx := range bc.TransactionQueue {
		if _, ok := keep[i]; !ok {
			dropped++
			continue
		}
		remaining = append(remaining, tx)
	}
	for i := len(remaining); i < len(bc.TransactionQueue); i++ {
		bc.TransactionQueue[i] = nil
	}
	bc.TransactionQueue = remaining

	bc.Metrics().Add("mempool_evicted", uint64(dropped))
	LogVerbosef("Trimmed %d transaction(s) from the mempool to stay within %d", dropped, capacity)
}

// MempoolFeeRates returns the fee rate of every queued transaction, highest
// first. It exists for observability and tests.
func (bc *Blockchain) MempoolFeeRates() []float64 {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	rates := make([]float64, 0, len(bc.TransactionQueue))
	for _, tx := range bc.TransactionQueue {
		if tx != nil {
			rates = append(rates, txFeeRate(tx))
		}
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(rates)))
	return rates
}
