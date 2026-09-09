// Package sdk is a software development kit for building blockchain applications.
// File sdk/blockchain_mining.go - Mining: proof of work, the mining loop and block production.
package sdk

import (
	"context"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"
)

// difficultyTarget converts an integer difficulty into a 256-bit target.
//
// A hash is valid when it is <= target, so a larger difficulty means a smaller
// target. The bounds matter: uint(256-difficulty) underflows for difficulty > 256
// and produces an astronomically large shift.
func difficultyTarget(difficulty int) *big.Int {
	if difficulty < 1 {
		difficulty = 1
	}
	if difficulty > 255 {
		difficulty = 255
	}
	//nolint:gosec // difficulty is clamped to [0,255] immediately above
	return new(big.Int).Lsh(big.NewInt(1), uint(256-difficulty))
}

// Mine attempts to mine a block, returning an error if no valid proof was found.
//
// Mine no longer mutates chain state. The simple-PoW path used to append to
// bc.Blocks and clear bc.TransactionQueue itself, without holding bc.mux and in
// addition to the append its caller already performed -- so every block was added
// twice and the mempool was cleared from an unsynchronised goroutine.
func (bc *Blockchain) Mine(block *Block, difficulty int) (*Block, error) {
	if bc.useHeliosMining {
		return bc.mineWithHelios(block, difficulty)
	}
	return bc.mineWithSimplePoW(block, difficulty)
}

// mineWithHelios mines a block using the Helios three-stage algorithm
func (bc *Blockchain) mineWithHelios(block *Block, difficulty int) (*Block, error) {
	LogVerbosef("Mining block [#%s] with Helios algorithm...", block.Index.String())

	targetDifficulty := difficultyTarget(difficulty)

	// Create block header for mining
	blockHeader := block.createBlockHeaderForMining()

	// Show Helios Stage 1: Proof Generation
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowHeliosProgress(1, "Proof Generation")
	}

	// Show mining progress
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowMiningProgress(int(block.Index.Int64()), difficulty, block.Hash)
	}

	// The parent is the block this one names, so the delay chains onto its
	// predecessor's rather than starting fresh.
	parent := bc.blockByHash(block.Header.PreviousHash)

	// Mine using Helios algorithm
	proof, err := bc.heliosAlgorithm.MineOnParent(blockHeader, parentDelayOutput(parent), targetDifficulty)
	if err != nil {
		// A mining failure (including the timeout) must abandon the block. The old
		// code logged the error and returned the *unmined* block, which the caller
		// then appended to the chain and persisted.
		return nil, fmt.Errorf("helios mining failed: %w", err)
	}

	// Show Helios Stage 2: Sidechain Routing
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowHeliosProgress(2, "Sidechain Routing")
	}

	// Show Helios Stage 3: Block Finalization
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowHeliosProgress(3, "Block Finalization")
	}

	// Update block with Helios proof
	if err := block.updateWithHeliosProof(proof); err != nil {
		return nil, fmt.Errorf("cannot record the proof on block %s: %w",
			block.Index.String(), err)
	}

	// Verify our own work before publishing it. bc.heliosValidator was constructed
	// and then never called anywhere, so nothing ever checked a proof.
	if err := bc.verifyHeliosProof(block, parent, difficulty); err != nil {
		return nil, fmt.Errorf("self-verification of freshly mined block failed: %w", err)
	}

	LogVerbosef("Helios mining successful: nonce=%d, hash=%s", proof.Nonce, proof.FinalHash)
	return block, nil
}

// blockDifficulty returns the difficulty a block declares, falling back to the
// chain default for blocks written before the field was populated.
func blockDifficulty(block *Block, fallback int) int {
	if block != nil && block.Header.Difficulty > 0 {
		return int(block.Header.Difficulty)
	}
	return fallback
}

// parentDelayOutput returns the delay output a block's child must chain onto.
//
// Genesis, and any block mined without Helios, contributes nothing -- an empty
// output is a well-defined starting point for the chain rather than a special
// case scattered through the callers.
func parentDelayOutput(parent *Block) []byte {
	if parent == nil || parent.HeliosProof == nil {
		return nil
	}
	return parent.HeliosProof.Stage2Result
}

// verifyHeliosProof checks a block's stored Helios proof against its header and
// the target difficulty.
func (bc *Blockchain) verifyHeliosProof(block, parent *Block, difficulty int) error {
	if block.HeliosProof == nil {
		return errors.New("block carries no Helios proof")
	}
	if bc.heliosAlgorithm == nil {
		return errors.New("helios algorithm is not initialized")
	}

	target := difficultyTarget(difficulty)

	// Recompute the proof hash from the block header: this is what makes the
	// proof binding rather than self-asserted. The parent's delay output goes in
	// too, which is what ties this block's delay to its predecessor's -- a
	// verifier that used the wrong parent would derive a different VDF input and
	// the proof would not check out.
	if err := bc.heliosAlgorithm.ValidateProofOnParent(
		block.HeliosProof,
		block.createBlockHeaderForMining(),
		parentDelayOutput(parent),
		target,
	); err != nil {
		return err
	}

	if bc.heliosValidator != nil {
		if err := bc.heliosValidator.ValidateFullProof(block.HeliosProof, target); err != nil {
			return err
		}
	}
	return nil
}

// mineWithSimplePoW mines a block using the original simple proof-of-work.
//
// It searches the nonce space and returns an error when it is exhausted. It does
// not touch chain state: appending the block and clearing the mempool is the
// caller's job, done once, under the lock.
func (bc *Blockchain) mineWithSimplePoW(block *Block, difficulty int) (*Block, error) {
	prefix := strings.Repeat("0", difficulty)

	LogVerbosef("Mining a new Block [#%s] with [%d] Txs...", block.Index.String(), len(block.Transactions))

	// Show mining progress
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowMiningProgress(int(block.Index.Int64()), difficulty, block.Hash)
	}

	for i := 0; i < maxMiningNonce; i++ {
		//nolint:gosec // i < maxMiningNonce == 1<<32, so it fits exactly
		block.Header.Nonce = uint32(i)
		block.Hash = block.CalculateHash()

		if strings.HasPrefix(block.Hash, prefix) {
			LogVerbosef("Mined a new Block [#%s] with [%d] TXs & Hash [%s]",
				block.Index.String(), len(block.Transactions), block.Hash)
			return block, nil
		}
	}

	return nil, fmt.Errorf("exhausted the nonce space without finding a proof at difficulty %d", difficulty)
}

// Run is a long-running function that manages the blockchain.
//
// It is a thin wrapper over RunContext for callers that do not manage a context.
func (bc *Blockchain) Run(difficulty int) {
	bc.RunContext(context.Background(), difficulty)
}

// RunContext starts the mining and status loops, stopping when ctx is cancelled.
//
// Both tickers used to run in goroutines with no exit path at all: they were
// never stopped, so every Blockchain leaked two goroutines and two tickers for
// the life of the process, and there was no way to shut mining down cleanly.
func (bc *Blockchain) RunContext(ctx context.Context, difficulty int) {
	LogInfof("Blockchain.Run started")

	// Start progress indicator
	if bc.progressIndicator != nil {
		bc.progressIndicator.Start()
	}

	blockTime := bc.cfg.BlockTime
	if blockTime <= 0 {
		blockTime = blockTimeInSec
	}

	go func() {
		statusTicker := time.NewTicker(time.Second)
		defer statusTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-statusTicker.C:
				bc.DisplayStatus()
			}
		}
	}()

	go func() {
		blockTicker := time.NewTicker(time.Duration(blockTime) * time.Second)
		defer blockTicker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-blockTicker.C:
				if bc.IsMenuActive() {
					continue // Skip block creation when the menu is open
				}

				LogVerbosef("Block ticker fired, creating new block (current=%d next=%d total=%d)",
					bc.CurrentBlockIndex, bc.NextBlockIndex, bc.GetBlockCount())
				bc.createNewBlock(difficulty)
			}
		}
	}()
}

func (bc *Blockchain) createNewBlock(difficulty int) {
	bc.mux.Lock()
	previousHash := ""
	if len(bc.Blocks) > 0 {
		previousHash = bc.Blocks[len(bc.Blocks)-1].Hash
	}

	// Take the highest-paying transactions that fit within MaxBlockSize, rather
	// than draining the whole mempool into an unbounded block. Anything that does
	// not fit stays queued for the next one.
	queuedTransactions := bc.selectBlockTransactionsLocked()
	nextBlockIndex := bc.NextBlockIndex
	// Difficulty is derived from the chain's own history, so every node agrees on
	// what this block was required to meet.
	blockDifficulty := bc.expectedDifficultyForNextLocked()
	bc.mux.Unlock()

	// The mempool already holds the real transactions, including the ones mirrored
	// to the sidechain, so there is nothing to merge in from the router.
	allTransactions := queuedTransactions

	newBlock := NewBlock(allTransactions, previousHash)
	newBlock.Index = *big.NewInt(int64(nextBlockIndex))
	// Stamp the difficulty this block is required to meet.
	//
	// Header.Difficulty used to be left at the InitialDifficulty constant while
	// verification used cfg.Difficulty, so the field was decorative and disagreed
	// with the work done. It is now the retargeted value derived from the chain,
	// which is also what peers will validate this block against.
	//nolint:gosec // expectedDifficultyForNextLocked clamps to [1, maxAcceptableDifficulty]
	newBlock.Header.Difficulty = uint32(blockDifficulty)
	newBlock.Header.MerkleRoot = newBlock.CalculateMerkleRoot()

	// Show block progress
	if bc.progressIndicator != nil {
		bc.progressIndicator.ShowBlockProgress(int(newBlock.Index.Int64()), len(allTransactions))
	}

	minedBlock, err := bc.Mine(newBlock, blockDifficulty)
	if err != nil {
		// Mining failed, so there is no block. Return the transactions to the
		// mempool rather than losing them: previously the unmined block was
		// appended and persisted regardless.
		LogInfof("Failed to mine block #%d: %v", nextBlockIndex, err)
		bc.requeueTransactions(queuedTransactions)
		return
	}
	newBlock = minedBlock

	if err := bc.TXLookup.Add(newBlock); err != nil {
		LogInfof("Error adding block to TXLookup: %v", err)
	}

	if err := newBlock.save(); err != nil {
		LogInfof("Error saving block: %v", err)
	}

	bc.commitMinedBlock(newBlock, len(queuedTransactions))

	// Relay the block we just mined. This happens after the lock is released, so
	// network I/O never blocks the chain.
	//
	// Only *mined* blocks are announced, never accepted ones. Re-announcing an
	// accepted block would echo it straight back to the peer that sent it; peers
	// that are further behind close the gap through the periodic Syncer, which
	// fetches the intervening blocks too. One-hop announcement plus periodic sync
	// is simpler than gossip with deduplication, and has no loop to get wrong.
	bc.announce(newBlock)
}

// commitMinedBlock appends a freshly mined block and persists chain state.
func (bc *Blockchain) commitMinedBlock(newBlock *Block, txCount int) {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	{
		if _, err := bc.ensureUTXOSetLocked().ApplyBlock(newBlock, bc.feeSplitFor()); err != nil {
			// The block cannot be applied, so it must not join the chain. Its
			// transactions were validated on the way into the mempool, so this
			// means they conflict with each other.
			LogInfof("Refusing to commit block %s: %v", newBlock.Index.String(), err)
			return
		}
	}

	bc.Blocks = append(bc.Blocks, newBlock)
	bc.CurrentBlockIndex = int(newBlock.Index.Int64())
	bc.NextBlockIndex = bc.CurrentBlockIndex + 1

	if err := bc.saveLocked(); err != nil {
		LogInfof("Error saving blockchain state: %v", err)
	}

	bc.Metrics().Inc("blocks_mined")
	bc.Metrics().RecordBlock(newBlock.Header.Timestamp, int(newBlock.Header.Difficulty))
	//nolint:gosec // a transaction count from len(), never negative
	bc.Metrics().Add("tx_mined", uint64(txCount))

	LogVerbosef("New block created: [#%s] Hash: %s with %d transactions",
		newBlock.Index.String(), newBlock.Hash, txCount)
	LogVerbosef("Blockchain state updated: CurrentBlockIndex=%d, NextBlockIndex=%d",
		bc.CurrentBlockIndex, bc.NextBlockIndex)
}

// requeueTransactions puts transactions back at the front of the mempool after a
// failed block, preserving their original ordering.
func (bc *Blockchain) requeueTransactions(txs []Transaction) {
	if len(txs) == 0 {
		return
	}

	bc.mux.Lock()
	defer bc.mux.Unlock()
	bc.TransactionQueue = append(txs, bc.TransactionQueue...)
	bc.trimMempoolLocked()
}
