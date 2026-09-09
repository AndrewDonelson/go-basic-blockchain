// Package sdk is a software development kit for building blockchain applications.
// File sdk/blockchain_accept.go - Block acceptance and chain validation.
package sdk

import (
	"errors"
	"fmt"
	"math/big"
)

// VerifySignature verifies the signature of the given transaction.
func (bc *Blockchain) VerifySignature(tx Transaction) error {
	_, err := tx.Verify([]byte(tx.GetSenderWallet().PublicPEM()), tx.GetSignature())
	return err
}

// AcceptBlock validates a block received from a peer and adds it to the chain.
//
// Three outcomes:
//
//  1. It extends the current head -- appended directly.
//  2. It belongs to a branch whose cumulative work exceeds ours -- the chain is
//     reorganised onto that branch.
//  3. It is valid but on a lighter branch, or its parent is unknown -- retained
//     in the block index and reported. Nothing is discarded, because a later
//     block may make that branch the heaviest.
//
// This used to accept only blocks that extended the head and refuse everything
// else, so two nodes that mined simultaneously diverged permanently.
func (bc *Blockchain) AcceptBlock(block *Block) error {
	_, err := bc.AcceptBlockWithResult(block)
	return err
}

// verifyProofOfWorkAgainstParent checks a block's proof using its parent's delay
// output.
//
// Split out from validateStandalone because the two need different things.
// Standalone validation is parent-independent and runs for every block including
// orphans; this needs the parent, because the delay is chained onto it.
//
// It still runs OUTSIDE the chain lock. Resolving the parent takes the lock for a
// map lookup and releases it; the expensive part -- the memory-hard phase and the
// VDF proof, together around a hundred milliseconds -- then runs holding nothing.
// That is possible because a parent is immutable: the child names it by hash, so
// which block it is cannot change underneath us, and reading it early is safe.
//
// A block whose parent is unknown is left unverified here and checked when its
// branch is connected, where the ancestry is known. It cannot join the chain
// before that happens.
func (bc *Blockchain) verifyProofOfWorkAgainstParent(block *Block) error {
	if !bc.useHeliosMining {
		return nil
	}
	if block.Index.Sign() == 0 {
		return nil // genesis has no parent and no proof to chain
	}

	parent := bc.blockByHash(block.Header.PreviousHash)
	if parent == nil {
		// An orphan. validateBranchLocked verifies it against its real ancestry
		// before it can be connected.
		return nil
	}

	declared := blockDifficulty(block, bc.cfg.Difficulty)
	if err := bc.verifyHeliosProof(block, parent, declared); err != nil {
		return fmt.Errorf("proof-of-work validation failed: %w", err)
	}
	return nil
}

// AcceptBlockWithResult is AcceptBlock, reporting what it did.
func (bc *Blockchain) AcceptBlockWithResult(block *Block) (ReorgResult, error) {
	var result ReorgResult

	if block == nil {
		return result, errors.New("block is nil")
	}

	// Both of these run outside the lock. Verifying a block re-runs the
	// memory-hard phase and checks a delay proof; holding the chain lock across
	// that would stall mining and every reader.
	if err := bc.validateStandalone(block); err != nil {
		return result, err
	}
	if err := bc.verifyProofOfWorkAgainstParent(block); err != nil {
		return result, err
	}

	result, disconnected, connected, err := bc.acceptBlockLocked(block)
	if err != nil {
		// A duplicate or a lighter branch is not a rejection; counting either
		// would make the rejection rate meaningless on a network where every
		// block arrives from several peers.
		if !errors.Is(err, ErrKnownBlock) && !errors.Is(err, ErrWeakerBranch) {
			bc.Metrics().Inc("blocks_rejected")
		}
		return result, err
	}

	bc.Metrics().Inc("blocks_accepted")
	bc.Metrics().RecordBlock(block.Header.Timestamp, int(block.Header.Difficulty))
	bc.Metrics().Add("tx_mined", uint64(len(block.Transactions)))
	if result.Reorganised {
		bc.Metrics().Inc("reorgs")
		//nolint:gosec // a count of disconnected blocks, bounded by maxReorgDepth
		bc.Metrics().Add("reorg_blocks", uint64(result.Disconnected))
	}

	// Mempool restoration takes the lock itself, so it runs after acceptBlockLocked
	// has released it.
	if len(disconnected) > 0 {
		result.RestoredTransactions = bc.restoreDisconnectedTransactions(disconnected, connected)
		if result.RestoredTransactions > 0 {
			LogInfof("Returned %d transactions to the mempool after reorganisation",
				result.RestoredTransactions)
		}
	}

	return result, nil
}

// acceptBlockLocked performs the chain mutation. It returns the disconnected and
// connected blocks so the caller can restore the mempool without the lock.
func (bc *Blockchain) acceptBlockLocked(block *Block) (ReorgResult, []*Block, []*Block, error) {
	var result ReorgResult

	bc.mux.Lock()
	defer bc.mux.Unlock()

	if len(bc.Blocks) == 0 {
		return result, nil, nil, errors.New("cannot accept a block before the genesis block exists")
	}

	bc.indexMainChainLocked()

	if _, known := bc.blockIndex[block.Hash]; known {
		return result, nil, nil, ErrKnownBlock
	}

	head := bc.Blocks[len(bc.Blocks)-1]

	// Fast path: the block extends the head.
	if block.Header.PreviousHash == head.Hash {
		if want := head.Index.Int64() + 1; block.Index.Int64() != want {
			return result, nil, nil, fmt.Errorf("block index %s does not follow the head (expected %d)",
				block.Index.String(), want)
		}
		if err := block.Validate(head); err != nil {
			return result, nil, nil, fmt.Errorf("block validation failed: %w", err)
		}
		// A block must declare the difficulty its own history requires. Without
		// this a peer could simply pick an easy value, and fork choice weighs
		// branches by the work their blocks claim.
		if want := bc.expectedDifficultyForNextLocked(); int(block.Header.Difficulty) != want {
			return result, nil, nil, fmt.Errorf(
				"block %s declares difficulty %d but its history requires %d",
				block.Index.String(), block.Header.Difficulty, want)
		}

		// Apply to the UTXO set BEFORE committing the block. A block that cannot
		// be applied -- a double spend, or a spend of outputs that do not exist --
		// must not join the chain, and ApplyBlock is all-or-nothing.
		if _, err := bc.ensureUTXOSetLocked().ApplyBlock(block, bc.feeSplitFor()); err != nil {
			return result, nil, nil, fmt.Errorf("block rejected by the UTXO set: %w", err)
		}

		bc.Blocks = append(bc.Blocks, block)
		bc.CurrentBlockIndex = int(block.Index.Int64())
		bc.NextBlockIndex = bc.CurrentBlockIndex + 1
		bc.rememberBlockLocked(block)

		if err := bc.TXLookup.Add(block); err != nil {
			LogInfof("Error adding accepted block to TXLookup: %v", err)
		}
		if err := block.save(); err != nil {
			return result, nil, nil, fmt.Errorf("failed to persist accepted block: %w", err)
		}

		bc.removeMinedTransactionsLocked(block)

		result.Extended = true
		result.Connected = 1
		return result, nil, nil, bc.saveLocked()
	}

	// Otherwise the block is on some other branch. Keep it either way: even a
	// lighter branch may later be extended past ours.
	bc.rememberBlockLocked(block)

	candidate, err := bc.assembleBranchLocked(block)
	if err != nil {
		return result, nil, nil, err
	}

	currentWork := ChainWork(bc.Blocks)
	candidateWork := new(big.Int).Add(
		ChainWork(bc.Blocks[:candidate.forkAt+1]),
		ChainWork(candidate.blocks),
	)

	if candidateWork.Cmp(currentWork) <= 0 {
		// Ties keep the chain we already have. Switching on equal work would make
		// nodes flip-flop between branches with every arriving block.
		return result, nil, nil, fmt.Errorf("%w (branch %s vs chain %s)",
			ErrWeakerBranch, candidateWork.String(), currentWork.String())
	}

	// Rebuild the set across the switch before touching the chain. If the branch
	// contains a double spend, this fails and the chain is left as it was.
	newSet, err := bc.utxoSetForBranchLocked(candidate)
	if err != nil {
		return result, nil, nil, fmt.Errorf("branch rejected by the UTXO set: %w", err)
	}

	disconnected, err := bc.reorganiseLocked(candidate)
	if err != nil {
		return result, nil, nil, err
	}

	bc.utxos = newSet

	// Reindex transactions for the branch we switched to.
	for _, connected := range candidate.blocks {
		if err := bc.TXLookup.Add(connected); err != nil {
			LogInfof("Error indexing reorganised block %s: %v", connected.Index.String(), err)
		}
		bc.removeMinedTransactionsLocked(connected)
	}

	staleAbove := int(bc.Blocks[len(bc.Blocks)-1].Index.Int64()) + 1
	if err := bc.persistChainFrom(candidate.blocks, staleAbove); err != nil {
		LogInfof("Error persisting reorganised chain: %v", err)
	}

	result.Reorganised = true
	result.ForkHeight = candidate.forkAt
	result.Disconnected = len(disconnected)
	result.Connected = len(candidate.blocks)

	LogInfof("Chain reorganised at height %d: -%d blocks, +%d blocks (work %s -> %s)",
		candidate.forkAt, len(disconnected), len(candidate.blocks),
		currentWork.String(), candidateWork.String())

	if err := bc.saveLocked(); err != nil {
		LogInfof("Error saving blockchain state after reorganisation: %v", err)
	}

	return result, disconnected, candidate.blocks, nil
}

// utxoSetForBranchLocked returns what the UTXO set would be after switching to a
// branch, without modifying the live one.
//
// The candidate is built on a clone: a branch that turns out to contain a double
// spend leaves the live set untouched, exactly as a failed reorganisation leaves
// the chain untouched.
func (bc *Blockchain) utxoSetForBranchLocked(candidate branch) (*UTXOSet, error) {
	set := bc.ensureUTXOSetLocked().Clone()
	split := bc.feeSplitFor()

	// Revert the blocks being disconnected, newest first.
	for i := len(bc.Blocks) - 1; i > candidate.forkAt; i-- {
		if err := set.RevertBlock(bc.Blocks[i].Hash); err != nil {
			return nil, fmt.Errorf("reverting block %s: %w", bc.Blocks[i].Index.String(), err)
		}
	}

	// Apply the branch in order.
	for _, block := range candidate.blocks {
		if _, err := set.ApplyBlock(block, split); err != nil {
			return nil, fmt.Errorf("applying branch block %s: %w", block.Index.String(), err)
		}
	}

	return set, nil
}

// validateStandalone checks everything about a block that does not depend on
// where it sits in the chain.
func (bc *Blockchain) validateStandalone(block *Block) error {
	if block.Hash == "" {
		return errors.New("block has no hash")
	}
	if block.Hash != block.CalculateHash() {
		return errors.New("block hash does not match its header")
	}

	// Read the header field directly rather than through blockDifficulty(), whose
	// fallback exists for blocks already on our chain that predate the field.
	// Applying that fallback here would let a peer send difficulty 0 and have it
	// silently promoted to the chain default -- bypassing the floor entirely.
	declared := int(block.Header.Difficulty)
	if declared < minAcceptableDifficulty {
		// Fork choice weighs work, so a branch of trivially mined blocks must not
		// be able to outweigh honest work by being long.
		return fmt.Errorf("block difficulty %d is below the minimum of %d",
			declared, minAcceptableDifficulty)
	}

	for _, tx := range block.Transactions {
		if tx == nil {
			return errors.New("block contains a nil transaction")
		}
		if err := tx.Validate(); err != nil {
			return fmt.Errorf("invalid transaction %s: %w", tx.GetID(), err)
		}
	}

	// The subsidy amount is checked against the height schedule. Block.Validate
	// confines a subsidy to one, first; this is where the chain's configuration
	// is available to say how much it may be.
	if err := bc.validateSubsidyLocked(block); err != nil {
		return err
	}

	// The proof of work is NOT checked here.
	//
	// It depends on the parent's delay output, which this function does not have
	// -- by design, since it is also called for blocks whose parent has not
	// arrived. verifyProofOfWorkAgainstParent does that part, and the caller runs
	// it once the parent is resolved.

	return nil
}

// removeMinedTransactionsLocked drops from the mempool anything now in a block.
func (bc *Blockchain) removeMinedTransactionsLocked(block *Block) {
	if len(bc.TransactionQueue) == 0 || len(block.Transactions) == 0 {
		return
	}

	mined := map[string]struct{}{}
	for _, tx := range block.Transactions {
		if tx != nil {
			mined[tx.GetID()] = struct{}{}
		}
	}

	remaining := bc.TransactionQueue[:0]
	for _, tx := range bc.TransactionQueue {
		if _, ok := mined[tx.GetID()]; ok {
			continue
		}
		remaining = append(remaining, tx)
	}
	bc.TransactionQueue = remaining
}

// ValidateChain validates the entire blockchain.
func (bc *Blockchain) ValidateChain() error {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	for i := 1; i < len(bc.Blocks); i++ {
		currentBlock := bc.Blocks[i]
		previousBlock := bc.Blocks[i-1]

		if currentBlock.Header.PreviousHash != previousBlock.Hash {
			return fmt.Errorf("invalid previous hash at block %d", i)
		}

		if currentBlock.Hash != currentBlock.CalculateHash() {
			return fmt.Errorf("invalid hash at block %d", i)
		}

		if err := currentBlock.Validate(previousBlock); err != nil {
			return fmt.Errorf("invalid block at index %d: %w", i, err)
		}

		// Verify the proof of work. ValidateChain previously checked hash linkage
		// and transaction validity but never that any work had been done.
		if bc.useHeliosMining {
			// previousBlock is this block's parent, so the chained delay is
			// checked against the same output the miner built on.
			if err := bc.verifyHeliosProof(currentBlock, previousBlock,
				blockDifficulty(currentBlock, bc.cfg.Difficulty)); err != nil {
				return fmt.Errorf("invalid proof of work at block %d: %w", i, err)
			}
		}

		for _, tx := range currentBlock.Transactions {
			if err := tx.Validate(); err != nil {
				return fmt.Errorf("invalid transaction %s in block %d: %w", tx.GetID(), i, err)
			}
		}
	}

	return nil
}
