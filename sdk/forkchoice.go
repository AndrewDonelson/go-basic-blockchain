// Package sdk is a software development kit for building blockchain applications.
// File sdk/forkchoice.go - fork choice and chain reorganisation
//
// Before this existed, AcceptBlock took only blocks that extended the current
// head. A block on a competing branch was refused outright, so two nodes that
// mined at the same time diverged permanently and nothing could bring them back
// together: sync closed gaps, but it could not resolve competing histories.
//
// The rule implemented here is heaviest-chain: a branch wins when the cumulative
// proof-of-work behind it exceeds the current chain's. Work is derived from each
// block's own declared difficulty, so a peer cannot claim a heavier branch
// without having actually mined it.
package sdk

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
)

var (
	// ErrOrphanBlock is returned for a block whose parent we do not have. The
	// block is retained: the syncer rewinds and fetches the missing ancestors.
	ErrOrphanBlock = errors.New("block parent is unknown")

	// ErrKnownBlock is returned for a block we already have.
	ErrKnownBlock = errors.New("block is already known")

	// ErrReorgTooDeep is returned when a branch would rewrite more history than
	// maxReorgDepth allows.
	ErrReorgTooDeep = errors.New("reorganisation exceeds the maximum depth")

	// ErrWeakerBranch is returned when a valid branch carries less work than the
	// chain we are on. The block is kept in case the branch is later extended.
	ErrWeakerBranch = errors.New("branch has less cumulative work than the current chain")
)

// maxTotalWork is 2^256, the size of the hash space.
var maxTotalWork = new(big.Int).Lsh(big.NewInt(1), 256)

// BlockWork returns the expected number of hash attempts behind one block.
//
// A block at difficulty d must produce a hash <= 2^(256-d), so on average
// 2^256 / 2^(256-d) = 2^d attempts are needed. Summing this across a branch gives
// a measure that stays meaningful when difficulty varies, which chain length does
// not: ten easy blocks must not outweigh three hard ones.
//
// Bitcoin divides by (target+1) to avoid a division by zero when the target is
// 2^256-1. Here the target is always an exact power of two and never zero, so the
// +1 would only introduce truncation error -- at difficulty 1 it makes the work 1
// instead of 2, and the doubling per difficulty step stops holding.
func BlockWork(difficulty uint32) *big.Int {
	d := int(difficulty)
	if d < minAcceptableDifficulty {
		d = minAcceptableDifficulty
	}

	return new(big.Int).Div(maxTotalWork, difficultyTarget(d))
}

// blockWorkOf returns the work behind a block, defaulting the difficulty for
// blocks written before the header field was populated.
func blockWorkOf(block *Block) *big.Int {
	return BlockWork(uint32(blockDifficulty(block, proofOfWorkDifficulty)))
}

// ChainWork returns the cumulative work behind a sequence of blocks.
func ChainWork(blocks []*Block) *big.Int {
	total := new(big.Int)
	for _, b := range blocks {
		if b == nil {
			continue
		}
		total.Add(total, blockWorkOf(b))
	}
	return total
}

// ChainWork returns the cumulative work behind the current main chain.
func (bc *Blockchain) ChainWork() *big.Int {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	return ChainWork(bc.Blocks)
}

// rememberBlockLocked records a block in the index of everything we have seen.
func (bc *Blockchain) rememberBlockLocked(block *Block) {
	if bc.blockIndex == nil {
		bc.blockIndex = map[string]*Block{}
	}
	bc.blockIndex[block.Hash] = block
}

// indexMainChainLocked (re)builds the index from the main chain. Called after a
// load, so blocks read from disk are reachable as branch ancestors.
func (bc *Blockchain) indexMainChainLocked() {
	if bc.blockIndex == nil {
		bc.blockIndex = map[string]*Block{}
	}
	for _, b := range bc.Blocks {
		bc.blockIndex[b.Hash] = b
	}
}

// mainChainHeightOfLocked returns the position of a hash on the main chain, or -1.
func (bc *Blockchain) mainChainHeightOfLocked(hash string) int {
	for i, b := range bc.Blocks {
		if b.Hash == hash {
			return i
		}
	}
	return -1
}

// branch is a candidate sequence of blocks rooted at a fork point.
type branch struct {
	// forkAt is the index into bc.Blocks of the last common block.
	forkAt int
	// blocks are the candidate blocks after the fork point, in order.
	blocks []*Block
}

// assembleBranchLocked walks back from tip through known blocks until it reaches
// the main chain, returning the branch that would replace everything after the
// fork point.
func (bc *Blockchain) assembleBranchLocked(tip *Block) (branch, error) {
	var collected []*Block

	current := tip
	for depth := 0; depth <= maxReorgDepth; depth++ {
		collected = append(collected, current)

		parentHash := current.Header.PreviousHash

		// Reached the main chain: this is the fork point.
		if at := bc.mainChainHeightOfLocked(parentHash); at >= 0 {
			// Reverse into chain order.
			blocks := make([]*Block, len(collected))
			for i, b := range collected {
				blocks[len(collected)-1-i] = b
			}
			return branch{forkAt: at, blocks: blocks}, nil
		}

		parent, known := bc.blockIndex[parentHash]
		if !known || parent == nil {
			return branch{}, ErrOrphanBlock
		}

		// A cycle would otherwise loop until the depth guard fires; catching it
		// here gives a clearer error.
		if parent.Hash == current.Hash {
			return branch{}, errors.New("block references itself as its parent")
		}
		current = parent
	}

	return branch{}, ErrReorgTooDeep
}

// validateBranchLocked checks that a branch links correctly and every block in it
// is internally valid.
//
// Proof of work is not re-verified here: a block only enters the index after
// AcceptBlock has verified it, so re-running the memory-hard phase for every
// block of every candidate branch would be wasted work.
func (bc *Blockchain) validateBranchLocked(b branch) error {
	if len(b.blocks) == 0 {
		return errors.New("branch is empty")
	}
	if b.forkAt < 0 || b.forkAt >= len(bc.Blocks) {
		return fmt.Errorf("fork point %d is outside the chain", b.forkAt)
	}

	previous := bc.Blocks[b.forkAt]
	for _, block := range b.blocks {
		if block == nil {
			return errors.New("branch contains a nil block")
		}
		if err := block.Validate(previous); err != nil {
			return fmt.Errorf("block %s in branch is invalid: %w", block.Index.String(), err)
		}
		if want := previous.Index.Int64() + 1; block.Index.Int64() != want {
			return fmt.Errorf("block %s breaks the index sequence (expected %d)",
				block.Index.String(), want)
		}
		previous = block
	}

	return nil
}

// reorganiseLocked swaps the main chain for a heavier branch.
//
// The replacement chain is assembled and validated in full *before* anything is
// swapped, so a branch that turns out to be invalid leaves the current chain
// untouched. A half-applied reorganisation would be far worse than a refused one.
func (bc *Blockchain) reorganiseLocked(b branch) (disconnected []*Block, err error) {
	if err := bc.validateBranchLocked(b); err != nil {
		return nil, err
	}

	depth := len(bc.Blocks) - (b.forkAt + 1)
	if depth > maxReorgDepth {
		return nil, fmt.Errorf("%w: %d blocks", ErrReorgTooDeep, depth)
	}

	// Build the replacement chain in a fresh slice; append would write into the
	// backing array the current chain is still using.
	replacement := make([]*Block, 0, b.forkAt+1+len(b.blocks))
	replacement = append(replacement, bc.Blocks[:b.forkAt+1]...)
	replacement = append(replacement, b.blocks...)

	// Blocks being removed from the main chain.
	disconnected = make([]*Block, len(bc.Blocks[b.forkAt+1:]))
	copy(disconnected, bc.Blocks[b.forkAt+1:])

	bc.Blocks = replacement
	bc.CurrentBlockIndex = int(replacement[len(replacement)-1].Index.Int64())
	bc.NextBlockIndex = bc.CurrentBlockIndex + 1

	// Everything that was on the chain stays in the index: a later block may
	// extend the branch we just abandoned and make it the heaviest again.
	for _, block := range disconnected {
		bc.rememberBlockLocked(block)
	}
	for _, block := range b.blocks {
		bc.rememberBlockLocked(block)
	}

	return disconnected, nil
}

// restoreDisconnectedTransactions returns transactions from disconnected blocks
// to the mempool, skipping any that the new branch already includes.
//
// Without this a reorganisation would silently destroy every transaction that
// was only in the abandoned blocks.
func (bc *Blockchain) restoreDisconnectedTransactions(disconnected, connected []*Block) int {
	included := map[string]struct{}{}
	for _, block := range connected {
		for _, tx := range block.Transactions {
			if tx != nil {
				included[tx.GetID()] = struct{}{}
			}
		}
	}

	var restored []Transaction
	for _, block := range disconnected {
		for _, tx := range block.Transactions {
			if tx == nil {
				continue
			}
			if _, ok := included[tx.GetID()]; ok {
				continue
			}
			restored = append(restored, tx)
		}
	}

	if len(restored) == 0 {
		return 0
	}

	bc.mux.Lock()
	defer bc.mux.Unlock()

	queued := map[string]struct{}{}
	for _, tx := range bc.TransactionQueue {
		queued[tx.GetID()] = struct{}{}
	}

	added := 0
	for _, tx := range restored {
		if _, ok := queued[tx.GetID()]; ok {
			continue
		}
		bc.TransactionQueue = append(bc.TransactionQueue, tx)
		added++
	}
	return added
}

// persistChainFrom writes the blocks from index onward and removes any stale
// block files left above the new tip.
func (bc *Blockchain) persistChainFrom(blocks []*Block, staleAbove int) error {
	for _, block := range blocks {
		if err := block.save(); err != nil {
			return fmt.Errorf("failed to persist block %s: %w", block.Index.String(), err)
		}
	}

	// A reorganisation onto a shorter-but-heavier branch leaves files for indices
	// that no longer exist. Removing them keeps a restart from resurrecting
	// blocks that are no longer part of the chain.
	if localStorage == nil {
		return nil
	}
	for index := staleAbove; ; index++ {
		path := filepath.Join(localStorage.dataPath, "blocks", fmt.Sprintf("%d.json", index))
		if _, err := os.Stat(path); err != nil {
			break
		}
		if err := os.Remove(path); err != nil {
			LogInfof("Could not remove stale block file %s: %v", path, err)
			break
		}
		LogVerbosef("Removed stale block file %s", path)
	}

	return nil
}

// ReorgResult describes what a call to AcceptBlock did.
type ReorgResult struct {
	// Extended is true when the block simply extended the current head.
	Extended bool
	// Reorganised is true when the main chain was switched to another branch.
	Reorganised bool
	// ForkHeight is the index of the last common block, for a reorganisation.
	ForkHeight int
	// Disconnected and Connected count the blocks removed and added.
	Disconnected int
	Connected    int
	// RestoredTransactions counts transactions returned to the mempool.
	RestoredTransactions int
}
