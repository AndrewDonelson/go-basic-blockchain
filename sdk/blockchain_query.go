// Package sdk is a software development kit for building blockchain applications.
// File sdk/blockchain_query.go - Read-only queries over blocks and transactions.
package sdk

// HasTransaction checks if a transaction with the given ID exists in the blockchain.
func (bc *Blockchain) HasTransaction(id *PUID) bool {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	for _, tx := range bc.TransactionQueue {
		if tx.GetID() == id.String() {
			return true
		}
	}

	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.GetID() == id.String() {
				return true
			}
		}
	}

	return false
}

// HasTransactionID reports whether a transaction ID is already known, in the
// mempool or in any block.
func (bc *Blockchain) HasTransactionID(id string) bool {
	if id == "" {
		return false
	}

	bc.mux.Lock()
	defer bc.mux.Unlock()

	// The mempool is small and changes constantly, so it is scanned directly.
	for _, tx := range bc.TransactionQueue {
		if tx != nil && tx.GetID() == id {
			return true
		}
	}

	bc.indexTransactionsLocked()
	_, known := bc.txIDIndex[id]
	return known
}

// indexTransactionsLocked brings the mined-transaction index up to date.
//
// This function exists because HasTransactionID is called for every transaction
// arriving from the network -- the relay path uses it to avoid re-announcing
// what it already has. It used to walk every transaction in every block while
// holding the chain lock, so a peer sending transactions made each node re-scan
// its whole history per message, blocking mining and every reader for the
// duration. Cost is now a map lookup, with work proportional only to blocks that
// have arrived since the last call.
//
// The caller must hold bc.mux.
func (bc *Blockchain) indexTransactionsLocked() {
	if bc.txIDIndex == nil {
		bc.txIDIndex = map[string]struct{}{}
		bc.txIndexedBlocks = 0
	}

	// As with blockIndex, a reorganisation can replace blocks below the recorded
	// length. Unlike blockIndex, stale entries here would be wrong rather than
	// merely redundant -- a transaction that was rolled back must become
	// unknown again, or it can never be mined onto the new branch -- so the
	// index is discarded and rebuilt rather than extended.
	valid := bc.txIndexedBlocks <= len(bc.Blocks)
	if valid && bc.txIndexedBlocks > 0 {
		valid = bc.Blocks[bc.txIndexedBlocks-1].Hash == bc.txIndexedTipHash
	}
	if !valid {
		bc.txIDIndex = map[string]struct{}{}
		bc.txIndexedBlocks = 0
	}

	for _, block := range bc.Blocks[bc.txIndexedBlocks:] {
		for _, tx := range block.Transactions {
			if tx != nil {
				bc.txIDIndex[tx.GetID()] = struct{}{}
			}
		}
	}
	bc.txIndexedBlocks = len(bc.Blocks)
	bc.txIndexedTipHash = ""
	if n := len(bc.Blocks); n > 0 {
		bc.txIndexedTipHash = bc.Blocks[n-1].Hash
	}
}

// GetLatestBlock returns the latest block in the blockchain.
func (bc *Blockchain) GetLatestBlock() *Block {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	if len(bc.Blocks) == 0 {
		return nil
	}
	return bc.Blocks[len(bc.Blocks)-1]
}

// GetBlockByHash returns a block with the given hash.
func (bc *Blockchain) GetBlockByHash(hash string) *Block {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	for _, block := range bc.Blocks {
		if block.Hash == hash {
			return block
		}
	}
	return nil
}

// GetBlockByIndex returns a block at the given index.
func (bc *Blockchain) GetBlockByIndex(index int64) *Block {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	// Match on the block's own Index rather than its slice position: the two
	// diverge as soon as blocks are loaded from disk out of order, or a block is
	// ever pruned.
	for _, block := range bc.Blocks {
		if block.Index.Int64() == index {
			return block
		}
	}
	return nil
}

// GetTransactionByID returns a transaction with the given ID.
func (bc *Blockchain) GetTransactionByID(id string) Transaction {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	// First, check the transaction queue
	for _, tx := range bc.TransactionQueue {
		if tx.GetID() == id {
			return tx
		}
	}

	// Then, check all blocks
	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.GetID() == id {
				return tx
			}
		}
	}

	return nil
}

// GetTransactionHistory returns the transaction history for a given wallet address.
func (bc *Blockchain) GetTransactionHistory(address string) []Transaction {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	var history []Transaction

	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx == nil {
				continue
			}
			// transactionParties handles a nil sender or recipient wallet. The
			// previous form called GetSenderWallet().GetAddress() directly, which
			// nil-dereferences on a coinbase, and asserted tx.(*Bank) on the
			// strength of the protocol string alone -- so a transaction claiming
			// to be a BANK without being one panicked the node reading it. Both
			// arrive from disk and from peers.
			sender, recipient := transactionParties(tx)
			if sender == address || recipient == address {
				history = append(history, tx)
			}
		}
	}

	return history
}

// GetPendingTransactions returns all pending transactions in the queue.
func (bc *Blockchain) GetPendingTransactions() []Transaction {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	// Return a copy. Handing out the live slice let callers read (and append to)
	// mutex-guarded state after the lock was released.
	out := make([]Transaction, len(bc.TransactionQueue))
	copy(out, bc.TransactionQueue)
	return out
}

// GetBlockchainInfo returns general information about the blockchain.
func (bc *Blockchain) GetBlockchainInfo() BlockchainInfo {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	return BlockchainInfo{
		Version:    BlockchainVersion,
		Name:       bc.cfg.BlockchainName,
		Symbol:     bc.cfg.BlockchainSymbol,
		BlockTime:  bc.cfg.BlockTime,
		Difficulty: bc.cfg.Difficulty,
		Fee:        bc.cfg.TransactionFee,
	}
}

// GetMempoolSize returns the number of transactions in the mempool (transaction queue).
func (bc *Blockchain) GetMempoolSize() int {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	return len(bc.TransactionQueue)
}

// Height returns the index of the head block, or -1 for an empty chain.
//
// Height is the block's own Index, not its slice position, so it stays correct
// if the two ever diverge.
func (bc *Blockchain) Height() int {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	return bc.heightLocked()
}

func (bc *Blockchain) heightLocked() int {
	if len(bc.Blocks) == 0 {
		return -1
	}
	return int(bc.Blocks[len(bc.Blocks)-1].Index.Int64())
}

// HeadHash returns the hash of the head block, or "" for an empty chain.
func (bc *Blockchain) HeadHash() string {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	if len(bc.Blocks) == 0 {
		return ""
	}
	return bc.Blocks[len(bc.Blocks)-1].Hash
}

// GenesisHash returns the hash of block 0, or "" for an empty chain.
//
// Peers compare this before syncing: two nodes with different genesis blocks are
// on different networks, and pulling blocks across that boundary would be
// meaningless.
func (bc *Blockchain) GenesisHash() string {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	if len(bc.Blocks) == 0 {
		return ""
	}
	return bc.Blocks[0].Hash
}

// ChainStatus summarises this node's chain for a peer.
func (bc *Blockchain) ChainStatus() ChainStatus {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	status := ChainStatus{Height: bc.heightLocked()}
	if len(bc.Blocks) > 0 {
		status.HeadHash = bc.Blocks[len(bc.Blocks)-1].Hash
		status.GenesisHash = bc.Blocks[0].Hash
	}
	if n := GetNode(); n != nil {
		status.NodeID = n.ID
	}
	return status
}

// GetBlocksFrom returns up to count blocks starting at block index startIndex.
//
// Selection is by the block's own Index rather than its slice position, so a
// sync request for "everything from height N" means what it says.
func (bc *Blockchain) GetBlocksFrom(startIndex, count int) []*Block {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	if count <= 0 {
		return []*Block{}
	}

	out := make([]*Block, 0, count)
	for _, block := range bc.Blocks {
		if int(block.Index.Int64()) < startIndex {
			continue
		}
		out = append(out, block)
		if len(out) == count {
			break
		}
	}
	return out
}

// GetBlockRange returns a copy of the blocks in [start, end).
//
// Callers (notably the API handlers) used to index bc.Blocks directly while the
// mining goroutine appended to it. Returning a copy taken under the lock removes
// that race without handing out a reference into guarded state.
func (bc *Blockchain) GetBlockRange(start, end int) []*Block {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	if start < 0 {
		start = 0
	}
	if end > len(bc.Blocks) {
		end = len(bc.Blocks)
	}
	if start >= end {
		return []*Block{}
	}

	out := make([]*Block, end-start)
	copy(out, bc.Blocks[start:end])
	return out
}

// GetAllTransactions returns every transaction in the chain plus the mempool.
func (bc *Blockchain) GetAllTransactions() []Transaction {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	all := make([]Transaction, 0, len(bc.TransactionQueue))
	for _, block := range bc.Blocks {
		all = append(all, block.Transactions...)
	}
	return append(all, bc.TransactionQueue...)
}

// GetBlockCount returns the number of blocks in the blockchain.
func (bc *Blockchain) GetBlockCount() int {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	return len(bc.Blocks)
}
