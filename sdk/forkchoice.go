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
	//nolint:gosec // the value originates in a uint32 header field
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
// indexMainChainLocked brings the block index up to date with the main chain.
//
// It used to re-insert every block on every call, and it is called once per
// accepted block -- so syncing n blocks did n(n+1)/2 map writes while holding the
// chain lock, which blocks mining and every reader. Only the blocks added since
// the last call are indexed now.
//
// A reorganisation can shorten the chain; when that happens the counter is no
// longer a valid prefix length, so the index is rebuilt. Entries for blocks no
// longer on the main chain are deliberately left in place -- blockIndex holds
// every block seen, on the main chain or not, so a later block can still find
// its parent on an abandoned branch.
func (bc *Blockchain) indexMainChainLocked() {
	if bc.blockIndex == nil {
		bc.blockIndex = map[string]*Block{}
		bc.indexedBlocks = 0
	}

	// The count alone is not enough. A reorganisation can replace blocks *below*
	// the previous length, so trusting the count would silently skip them --
	// their parents would then be unfindable and the next block on that branch
	// rejected. Re-index from scratch unless the block at the recorded position
	// is still the one that was recorded there.
	valid := bc.indexedBlocks <= len(bc.Blocks)
	if valid && bc.indexedBlocks > 0 {
		valid = bc.Blocks[bc.indexedBlocks-1].Hash == bc.indexedTipHash
	}
	if !valid {
		bc.indexedBlocks = 0
		// Heights shift when blocks are replaced, so this map cannot be extended
		// across a reorganisation the way blockIndex can.
		bc.mainChainHeight = nil
	}
	if bc.mainChainHeight == nil {
		bc.mainChainHeight = make(map[string]int, len(bc.Blocks))
	}

	for i := bc.indexedBlocks; i < len(bc.Blocks); i++ {
		b := bc.Blocks[i]
		bc.blockIndex[b.Hash] = b
		bc.mainChainHeight[b.Hash] = i
	}
	bc.indexedBlocks = len(bc.Blocks)
	bc.indexedTipHash = ""
	if n := len(bc.Blocks); n > 0 {
		bc.indexedTipHash = bc.Blocks[n-1].Hash
	}
}

// mainChainHeightOfLocked returns the position of a hash on the main chain, or -1.
//
// This is called once per step while walking a candidate branch back to a fork
// point, up to maxReorgDepth times per block. It used to scan the whole chain on
// each step, so one block from a peer whose parent is unknown cost
// maxReorgDepth * len(chain) comparisons under the chain lock -- work an
// attacker chooses by sending blocks that fork deeply.
func (bc *Blockchain) mainChainHeightOfLocked(hash string) int {
	if bc.mainChainHeight != nil {
		if height, ok := bc.mainChainHeight[hash]; ok {
			return height
		}
		// The index is authoritative once built: indexMainChainLocked runs at the
		// top of every acceptance, so a hash absent from it is not on the chain.
		return -1
	}

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

	// Difficulty is per-branch: a competing branch has its own history and may
	// legitimately be at a different difficulty, so each block is checked against
	// the ancestry it actually builds on rather than against the main chain.
	ancestry := make([]*Block, 0, b.forkAt+1+len(b.blocks))
	ancestry = append(ancestry, bc.Blocks[:b.forkAt+1]...)

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
		if err := bc.validateSubsidyLocked(block); err != nil {
			return fmt.Errorf("block %s in branch: %w", block.Index.String(), err)
		}

		// The proof of work is verified here, against this branch's own ancestry.
		// A block whose parent had not arrived was left unchecked at acceptance
		// -- there was nothing to chain its delay onto -- so this is where an
		// orphan's work is finally confirmed, before it can join the chain.
		if bc.useHeliosMining {
			if err := bc.verifyHeliosProof(block, previous,
				blockDifficulty(block, bc.cfg.Difficulty)); err != nil {
				return fmt.Errorf("block %s in branch has an invalid proof of work: %w",
					block.Index.String(), err)
			}
		}

		if want := bc.ExpectedDifficulty(ancestry); int(block.Header.Difficulty) != want {
			return fmt.Errorf("block %s in branch declares difficulty %d but its history requires %d",
				block.Index.String(), block.Header.Difficulty, want)
		}

		ancestry = append(ancestry, block)
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
	// Restored transactions bypass admission control, so re-apply the bound.
	bc.trimMempoolLocked()
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

	// A stale file that cannot be deleted is logged, not fatal. The chain has
	// already been reorganised in memory and the new blocks are written; failing
	// here would report a reorganisation that in fact succeeded, and the leftover
	// file is above the tip where nothing reads it.
	return nil //nolint:nilerr // cleanup failure must not fail a completed reorg
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
