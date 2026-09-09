package sdk

import (
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// Helpers
//
// These build chains by hand so branch shapes are explicit. Helios mining is
// disabled on the fixtures: the proof of work is exercised in its own package,
// and mining every block of every branch would make these tests take minutes.
// -----------------------------------------------------------------------------

// forkTestChain returns a chain of `height+1` blocks (indices 0..height).
func forkTestChain(t *testing.T, height int, difficulty uint32) *Blockchain {
	t.Helper()

	cfg := NewConfig()
	cfg.DataPath = t.TempDir()
	if err := NewLocalStorage(cfg.DataPath); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	t.Cleanup(func() { _ = NewLocalStorage("./test_data") })

	bc := &Blockchain{
		cfg:             cfg,
		TXLookup:        NewTXLookupManager(),
		Blocks:          []*Block{},
		useHeliosMining: false,
	}

	// Anchor the fixture well in the past. Tests extend these chains forward with
	// controlled spacing, and Block.Validate rejects timestamps in the future.
	base := time.Now().Add(-48 * time.Hour)

	previousHash := ""
	for i := 0; i <= height; i++ {
		b := forkTestBlock(i, previousHash, difficulty, "")
		b.Header.Timestamp = base.Add(time.Duration(i) * time.Minute)
		b.Hash = b.CalculateHash()
		bc.Blocks = append(bc.Blocks, b)
		previousHash = b.Hash
	}
	bc.CurrentBlockIndex = height
	bc.NextBlockIndex = height + 1
	return bc
}

// forkTestBlock builds a block with a deterministic, distinguishable hash.
//
// `salt` makes two blocks at the same height with the same parent differ, which
// is exactly what a fork is.
func forkTestBlock(index int, previousHash string, difficulty uint32, salt string) *Block {
	b := NewBlock(nil, previousHash)
	b.Index = *big.NewInt(int64(index))
	b.Header.Difficulty = difficulty
	// The nonce is a cheap way to make sibling blocks distinct without mining.
	b.Header.Nonce = uint32(len(salt)*1000 + index)
	if salt != "" {
		b.Header.MerkleRoot = []byte(salt)
	}
	b.Hash = b.CalculateHash()
	return b
}

// extendValidChain builds a branch whose blocks declare the difficulty their own
// ancestry requires, with a chosen spacing between timestamps.
//
// Spacing is what drives retargeting: blocks closer together than the target
// block time make the next window harder, further apart make it easier. This is
// how a branch legitimately becomes heavier -- it can no longer be done by simply
// declaring a high difficulty, which is the point of deriving difficulty from the
// chain.
func extendValidChain(bc *Blockchain, ancestry []*Block, count int, spacing time.Duration, salt string) []*Block {
	history := append([]*Block{}, ancestry...)
	blocks := make([]*Block, 0, count)

	parent := history[len(history)-1]
	previousHash := parent.Hash
	index := int(parent.Index.Int64())
	timestamp := parent.Header.Timestamp

	for i := 0; i < count; i++ {
		index++
		timestamp = timestamp.Add(spacing)

		b := NewBlock(nil, previousHash)
		b.Index = *big.NewInt(int64(index))
		b.Header.Difficulty = uint32(bc.ExpectedDifficulty(history))
		b.Header.Timestamp = timestamp
		b.Header.Nonce = uint32(len(salt)*1000 + index)
		if salt != "" {
			b.Header.MerkleRoot = []byte(salt)
		}
		b.Hash = b.CalculateHash()

		blocks = append(blocks, b)
		history = append(history, b)
		previousHash = b.Hash
	}
	return blocks
}

// extendChain appends `count` blocks to a parent, returning them in order.
func extendChain(parent *Block, count int, difficulty uint32, salt string) []*Block {
	blocks := make([]*Block, 0, count)
	previousHash := parent.Hash
	index := int(parent.Index.Int64())

	for i := 0; i < count; i++ {
		index++
		b := forkTestBlock(index, previousHash, difficulty, salt)
		blocks = append(blocks, b)
		previousHash = b.Hash
	}
	return blocks
}

// hashes lists a chain's block hashes, for comparing chain identity.
func hashes(blocks []*Block) []string {
	out := make([]string, len(blocks))
	for i, b := range blocks {
		out[i] = b.Hash
	}
	return out
}

// -----------------------------------------------------------------------------
// Work accounting
// -----------------------------------------------------------------------------

// TestBlockWorkGrowsWithDifficulty is the property fork choice rests on: work
// must be exponential in difficulty, so a long chain of easy blocks cannot
// outweigh a short chain of hard ones.
func TestBlockWorkGrowsWithDifficulty(t *testing.T) {
	previous := BlockWork(1)
	for d := uint32(2); d <= 32; d++ {
		current := BlockWork(d)
		if current.Cmp(previous) <= 0 {
			t.Fatalf("work at difficulty %d (%s) is not greater than at %d (%s)",
				d, current, d-1, previous)
		}
		// Each step of difficulty should roughly double the work.
		doubled := new(big.Int).Mul(previous, big.NewInt(2))
		if current.Cmp(doubled) != 0 {
			t.Fatalf("work at difficulty %d should be double difficulty %d: got %s, want %s",
				d, d-1, current, doubled)
		}
		previous = current
	}
}

// TestBlockWorkFloorsInvalidDifficulty guards a zero or negative difficulty
// producing zero or absurd work.
func TestBlockWorkFloorsInvalidDifficulty(t *testing.T) {
	floor := BlockWork(minAcceptableDifficulty)
	for _, d := range []uint32{0} {
		if got := BlockWork(d); got.Cmp(floor) != 0 {
			t.Fatalf("difficulty %d should be floored to %d, got work %s", d, minAcceptableDifficulty, got)
		}
	}
	if BlockWork(1).Sign() <= 0 {
		t.Fatal("work must always be positive")
	}
}

// TestChainWorkSums covers the aggregate, including mixed difficulties.
func TestChainWorkSums(t *testing.T) {
	blocks := []*Block{
		forkTestBlock(0, "", 4, ""),
		forkTestBlock(1, "a", 4, ""),
		forkTestBlock(2, "b", 8, ""),
	}

	want := new(big.Int).Add(
		new(big.Int).Mul(BlockWork(4), big.NewInt(2)),
		BlockWork(8),
	)
	if got := ChainWork(blocks); got.Cmp(want) != 0 {
		t.Fatalf("chain work = %s, want %s", got, want)
	}

	if got := ChainWork(nil); got.Sign() != 0 {
		t.Fatalf("empty chain should have zero work, got %s", got)
	}
	if got := ChainWork([]*Block{nil, blocks[0]}); got.Cmp(BlockWork(4)) != 0 {
		t.Fatal("a nil block should be skipped, not counted or panic")
	}
}

// TestFewerHarderBlocksOutweighManyEasyOnes is the case chain length gets wrong.
func TestFewerHarderBlocksOutweighManyEasyOnes(t *testing.T) {
	// Ten blocks at difficulty 4.
	easy := make([]*Block, 10)
	for i := range easy {
		easy[i] = forkTestBlock(i, "", 4, "")
	}
	// Three blocks at difficulty 8: each is 16x the work.
	hard := make([]*Block, 3)
	for i := range hard {
		hard[i] = forkTestBlock(i, "", 8, "")
	}

	if ChainWork(hard).Cmp(ChainWork(easy)) <= 0 {
		t.Fatalf("3 blocks at difficulty 8 (%s) should outweigh 10 at difficulty 4 (%s) -- "+
			"weighing by length would pick the wrong branch",
			ChainWork(hard), ChainWork(easy))
	}
}

// -----------------------------------------------------------------------------
// Extending the head
// -----------------------------------------------------------------------------

func TestAcceptBlockExtendsHead(t *testing.T) {
	bc := forkTestChain(t, 3, 4)
	next := extendChain(bc.Blocks[3], 1, 4, "")[0]

	result, err := bc.AcceptBlockWithResult(next)
	if err != nil {
		t.Fatalf("accept: %v", err)
	}

	if !result.Extended || result.Reorganised {
		t.Fatalf("expected a simple extension, got %+v", result)
	}
	if bc.Height() != 4 || bc.HeadHash() != next.Hash {
		t.Fatalf("chain did not extend: height %d head %s", bc.Height(), bc.HeadHash())
	}
}

func TestAcceptBlockRejectsDuplicates(t *testing.T) {
	bc := forkTestChain(t, 2, 4)

	if _, err := bc.AcceptBlockWithResult(bc.Blocks[1]); !errors.Is(err, ErrKnownBlock) {
		t.Fatalf("expected ErrKnownBlock for a block already on the chain, got %v", err)
	}

	next := extendChain(bc.Blocks[2], 1, 4, "")[0]
	if _, err := bc.AcceptBlockWithResult(next); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if _, err := bc.AcceptBlockWithResult(next); !errors.Is(err, ErrKnownBlock) {
		t.Fatalf("expected ErrKnownBlock on re-submission, got %v", err)
	}
	if bc.Height() != 3 {
		t.Fatalf("a duplicate must not extend the chain; height is %d", bc.Height())
	}
}

func TestAcceptBlockRejectsUnknownParent(t *testing.T) {
	bc := forkTestChain(t, 2, 4)

	orphan := forkTestBlock(9, "a-parent-we-have-never-seen", 4, "orphan")
	_, err := bc.AcceptBlockWithResult(orphan)
	if !errors.Is(err, ErrOrphanBlock) {
		t.Fatalf("expected ErrOrphanBlock, got %v", err)
	}

	// The block must be retained: a later block may complete the branch.
	bc.mux.Lock()
	_, kept := bc.blockIndex[orphan.Hash]
	bc.mux.Unlock()
	if !kept {
		t.Fatal("an orphan block must be retained, not discarded")
	}
}

func TestAcceptBlockRejectsBelowMinimumDifficulty(t *testing.T) {
	bc := forkTestChain(t, 2, 4)

	cheap := extendChain(bc.Blocks[2], 1, 0, "")[0] // difficulty 0
	if _, err := bc.AcceptBlockWithResult(cheap); err == nil {
		t.Fatal("a block below the minimum difficulty must be refused: fork choice " +
			"weighs work, so trivially mined blocks could otherwise outweigh honest ones")
	}
}

func TestAcceptBlockRejectsATamperedHash(t *testing.T) {
	bc := forkTestChain(t, 2, 4)

	bad := extendChain(bc.Blocks[2], 1, 4, "")[0]
	bad.Hash = "0000000000000000000000000000000000000000000000000000000000000000"

	if _, err := bc.AcceptBlockWithResult(bad); err == nil {
		t.Fatal("a block whose hash does not match its header must be refused")
	}
}

// -----------------------------------------------------------------------------
// Fork choice
// -----------------------------------------------------------------------------

// TestReorgOntoHeavierBranch is the headline case: a competing branch with more
// cumulative work replaces the current chain.
func TestReorgOntoHeavierBranch(t *testing.T) {
	// Chain: 0..5, all difficulty 4. Fork at block 2.
	bc := forkTestChain(t, 5, 4)
	originalHead := bc.HeadHash()

	// A competing branch from block 2, five blocks long (vs our three).
	rival := extendChain(bc.Blocks[2], 5, 4, "rival")

	var result ReorgResult
	for i, block := range rival {
		r, err := bc.AcceptBlockWithResult(block)
		// Blocks before the branch overtakes ours are valid but lighter.
		if err != nil && !errors.Is(err, ErrWeakerBranch) {
			t.Fatalf("rival block %d: %v", i, err)
		}
		if r.Reorganised {
			result = r
		}
	}

	if !result.Reorganised {
		t.Fatal("the chain never reorganised onto the heavier branch")
	}
	if result.ForkHeight != 2 {
		t.Fatalf("expected the fork at height 2, got %d", result.ForkHeight)
	}
	// The switch happens the moment the branch overtakes ours -- after its fourth
	// block, at which point it is 7 blocks against our 6. The fifth rival block
	// then arrives as an ordinary extension of the new head.
	if result.Disconnected != 3 {
		t.Fatalf("expected 3 blocks disconnected, got %d", result.Disconnected)
	}
	if result.Connected != 4 {
		t.Fatalf("expected 4 blocks connected at the moment of the switch, got %d", result.Connected)
	}

	if bc.Height() != 7 {
		t.Fatalf("expected height 7 after the reorg, got %d", bc.Height())
	}
	if bc.HeadHash() == originalHead {
		t.Fatal("the head did not change")
	}
	if bc.HeadHash() != rival[len(rival)-1].Hash {
		t.Fatal("the head is not the rival branch's tip")
	}

	// The chain must remain internally consistent.
	assertChainLinks(t, bc)
}

// TestReorgOntoShorterButHeavierBranch is the case that distinguishes work from
// length: a *shorter* branch wins because its blocks are harder.
//
// The branch cannot simply declare a high difficulty any more -- difficulty comes
// from the chain -- so it earns one: its blocks arrive far faster than the target
// interval, the retarget raises the requirement, and the resulting blocks carry
// enough work to outweigh a longer but slower chain.
func TestReorgOntoShorterButHeavierBranch(t *testing.T) {
	bc := forkTestChain(t, 1, 4)
	// Retarget every 2 blocks so the effect is reachable inside a short test.
	bc.cfg.DifficultyWindow = 2
	bc.cfg.BlockTime = 60 // target 60s per block

	// Our chain: slow blocks, so each retarget makes it easier.
	slow := extendValidChain(bc, bc.Blocks, 8, 10*time.Minute, "")
	for _, block := range slow {
		if _, err := bc.AcceptBlockWithResult(block); err != nil {
			t.Fatalf("slow block %s: %v", block.Index.String(), err)
		}
	}
	slowWork := bc.ChainWork()
	slowHeight := bc.Height()

	// A rival branch from block 1: fast blocks, so each retarget makes it harder.
	rival := extendValidChain(bc, bc.Blocks[:2], 5, time.Second, "fast")

	reorged := false
	for _, block := range rival {
		r, err := bc.AcceptBlockWithResult(block)
		if err != nil && !errors.Is(err, ErrWeakerBranch) {
			t.Fatalf("rival block %s: %v", block.Index.String(), err)
		}
		if r.Reorganised {
			reorged = true
		}
	}

	if !reorged {
		t.Fatalf("a shorter branch of harder blocks should have won "+
			"(chain work %s at height %d)", slowWork, slowHeight)
	}
	if bc.Height() >= slowHeight {
		t.Fatalf("expected the chain to shorten: height %d, was %d", bc.Height(), slowHeight)
	}
	if bc.ChainWork().Cmp(slowWork) <= 0 {
		t.Fatal("the chain switched to a branch with less work")
	}
	assertChainLinks(t, bc)
}

// TestLighterBranchIsRefusedButRetained: a valid competing branch that does not
// beat ours must not be applied, and must not be thrown away either.
func TestLighterBranchIsRefusedButRetained(t *testing.T) {
	bc := forkTestChain(t, 5, 4)
	originalHashes := hashes(bc.Blocks)

	rival := extendChain(bc.Blocks[2], 2, 4, "light") // 2 blocks vs our 3

	for _, block := range rival {
		_, err := bc.AcceptBlockWithResult(block)
		if err != nil && !errors.Is(err, ErrWeakerBranch) {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if got := hashes(bc.Blocks); len(got) != len(originalHashes) {
		t.Fatalf("the chain changed: %d blocks, want %d", len(got), len(originalHashes))
	}
	for i, h := range hashes(bc.Blocks) {
		if h != originalHashes[i] {
			t.Fatalf("block %d changed on a lighter branch", i)
		}
	}

	// Retained, so extending the branch later can still win.
	bc.mux.Lock()
	_, kept := bc.blockIndex[rival[1].Hash]
	bc.mux.Unlock()
	if !kept {
		t.Fatal("a lighter branch must be retained in case it is extended")
	}
}

// TestRetainedBranchWinsWhenExtended follows on: the branch refused above becomes
// the heaviest once more blocks arrive.
func TestRetainedBranchWinsWhenExtended(t *testing.T) {
	bc := forkTestChain(t, 5, 4)

	rival := extendChain(bc.Blocks[2], 2, 4, "later")
	for _, block := range rival {
		_, _ = bc.AcceptBlockWithResult(block)
	}
	if bc.Height() != 5 {
		t.Fatalf("the lighter branch should not have been applied yet, height %d", bc.Height())
	}

	// Two more blocks tip the balance.
	more := extendChain(rival[len(rival)-1], 2, 4, "later")
	reorged := false
	for _, block := range more {
		r, err := bc.AcceptBlockWithResult(block)
		if err != nil && !errors.Is(err, ErrWeakerBranch) {
			t.Fatalf("extending the branch: %v", err)
		}
		if r.Reorganised {
			reorged = true
		}
	}

	if !reorged {
		t.Fatal("the retained branch should have won once it was extended")
	}
	if bc.HeadHash() != more[len(more)-1].Hash {
		t.Fatal("head is not the extended branch's tip")
	}
	assertChainLinks(t, bc)
}

// TestEqualWorkKeepsTheCurrentChain: on a tie we must not switch, or nodes
// flip-flop between branches with every arriving block.
func TestEqualWorkKeepsTheCurrentChain(t *testing.T) {
	bc := forkTestChain(t, 4, 4)
	originalHead := bc.HeadHash()

	// A branch of exactly the same length and difficulty.
	rival := extendChain(bc.Blocks[2], 2, 4, "tie")
	for _, block := range rival {
		_, err := bc.AcceptBlockWithResult(block)
		if err != nil && !errors.Is(err, ErrWeakerBranch) {
			t.Fatalf("unexpected error: %v", err)
		}
	}

	if bc.HeadHash() != originalHead {
		t.Fatal("an equal-work branch must not displace the current chain")
	}
}

// TestReorgRestoresOrphanedTransactions: transactions that were only in the
// abandoned blocks must return to the mempool, not vanish.
func TestReorgRestoresOrphanedTransactions(t *testing.T) {
	bc := forkTestChain(t, 2, 4)

	from := newTestWallet(t, "reorg-from", 10000)
	to := newTestWallet(t, "reorg-to", 0)
	fundWalletForTest(t, bc, from, 10000)

	orphanedTx, err := NewBankTransaction(from, to, 42)
	if err != nil {
		t.Fatalf("create tx: %v", err)
	}
	orphanedTx.Signature, err = orphanedTx.Sign([]byte(from.PrivatePEM()))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// Put it in a block that extends our chain.
	doomed := NewBlock([]Transaction{orphanedTx}, bc.HeadHash())
	doomed.Index = *big.NewInt(3)
	doomed.Header.Difficulty = 4
	doomed.Header.MerkleRoot = doomed.CalculateMerkleRoot()
	doomed.Hash = doomed.CalculateHash()

	if _, err := bc.AcceptBlockWithResult(doomed); err != nil {
		t.Fatalf("accept doomed block: %v", err)
	}
	if !bc.HasTransactionID(orphanedTx.GetID()) {
		t.Fatal("the transaction should be on the chain")
	}

	// Now a heavier branch from block 2 that does NOT contain the transaction.
	rival := extendChain(bc.Blocks[2], 3, 4, "reorg")
	var result ReorgResult
	for _, block := range rival {
		r, err := bc.AcceptBlockWithResult(block)
		if err != nil && !errors.Is(err, ErrWeakerBranch) {
			t.Fatalf("rival block: %v", err)
		}
		if r.Reorganised {
			result = r
		}
	}

	if !result.Reorganised {
		t.Fatal("expected a reorganisation")
	}
	if result.RestoredTransactions != 1 {
		t.Fatalf("expected 1 transaction returned to the mempool, got %d", result.RestoredTransactions)
	}

	found := false
	for _, tx := range bc.GetPendingTransactions() {
		if tx.GetID() == orphanedTx.GetID() {
			found = true
		}
	}
	if !found {
		t.Fatal("the orphaned transaction was destroyed by the reorganisation instead of " +
			"being returned to the mempool")
	}
}

// TestReorgDoesNotRestoreTransactionsPresentInBothBranches guards double-spending
// the mempool: a transaction in both the old and new branch is already mined.
func TestReorgDoesNotRestoreTransactionsPresentInBothBranches(t *testing.T) {
	bc := forkTestChain(t, 2, 4)

	from := newTestWallet(t, "both-from", 10000)
	to := newTestWallet(t, "both-to", 0)
	fundWalletForTest(t, bc, from, 10000)
	tx, err := NewBankTransaction(from, to, 7)
	if err != nil {
		t.Fatalf("create tx: %v", err)
	}
	tx.Signature, _ = tx.Sign([]byte(from.PrivatePEM()))

	shared := []Transaction{tx}

	doomed := NewBlock(shared, bc.HeadHash())
	doomed.Index = *big.NewInt(3)
	doomed.Header.Difficulty = 4
	doomed.Header.MerkleRoot = doomed.CalculateMerkleRoot()
	doomed.Hash = doomed.CalculateHash()
	if _, err := bc.AcceptBlockWithResult(doomed); err != nil {
		t.Fatalf("accept: %v", err)
	}

	// A heavier branch that also includes the transaction.
	first := NewBlock(shared, bc.Blocks[2].Hash)
	first.Index = *big.NewInt(3)
	first.Header.Difficulty = 4
	first.Header.Nonce = 777
	first.Header.MerkleRoot = first.CalculateMerkleRoot()
	first.Hash = first.CalculateHash()

	rest := extendChain(first, 2, 4, "shared")

	var result ReorgResult
	for _, block := range append([]*Block{first}, rest...) {
		r, err := bc.AcceptBlockWithResult(block)
		if err != nil && !errors.Is(err, ErrWeakerBranch) {
			t.Fatalf("branch block: %v", err)
		}
		if r.Reorganised {
			result = r
		}
	}

	if !result.Reorganised {
		t.Fatal("expected a reorganisation")
	}
	if result.RestoredTransactions != 0 {
		t.Fatalf("a transaction present in the new branch must not be returned to the "+
			"mempool; %d were", result.RestoredTransactions)
	}
}

// TestReorgIsRefusedBeyondMaxDepth bounds how much history can be rewritten.
func TestReorgIsRefusedBeyondMaxDepth(t *testing.T) {
	bc := forkTestChain(t, maxReorgDepth+20, 4)

	// A branch forking right at genesis would rewrite everything.
	rival := extendChain(bc.Blocks[0], 3, 4, "deep")
	for _, block := range rival {
		_, err := bc.AcceptBlockWithResult(block)
		if err == nil {
			continue
		}
		if errors.Is(err, ErrWeakerBranch) || errors.Is(err, ErrReorgTooDeep) {
			continue
		}
		t.Fatalf("unexpected error: %v", err)
	}

	if bc.Height() != maxReorgDepth+20 {
		t.Fatalf("a too-deep branch must not rewrite the chain; height is %d", bc.Height())
	}
}

// TestFailedReorgLeavesTheChainIntact is the safety property that matters most:
// an invalid branch must not leave the chain half-rewritten.
func TestFailedReorgLeavesTheChainIntact(t *testing.T) {
	bc := forkTestChain(t, 4, 4)
	original := hashes(bc.Blocks)
	originalHeight := bc.Height()

	// Build a heavier branch, then corrupt a block in its middle so validation
	// fails partway through.
	rival := extendChain(bc.Blocks[1], 5, 4, "broken")

	// Register the branch's blocks in the index directly, so assembleBranch finds
	// them, then break the linkage of one of them.
	bc.mux.Lock()
	for _, b := range rival[:len(rival)-1] {
		bc.rememberBlockLocked(b)
	}
	bc.mux.Unlock()

	rival[2].Header.PreviousHash = "broken-link"

	if _, err := bc.AcceptBlockWithResult(rival[len(rival)-1]); err == nil {
		t.Fatal("expected the broken branch to be refused")
	}

	if bc.Height() != originalHeight {
		t.Fatalf("a failed reorganisation changed the chain height: %d -> %d",
			originalHeight, bc.Height())
	}
	for i, h := range hashes(bc.Blocks) {
		if h != original[i] {
			t.Fatalf("a failed reorganisation modified block %d", i)
		}
	}
	assertChainLinks(t, bc)
}

// TestReorgRemovesStaleBlockFiles: after switching to a shorter branch, files for
// indices that no longer exist must go, or a restart resurrects them.
func TestReorgRemovesStaleBlockFiles(t *testing.T) {
	bc := forkTestChain(t, 1, 4)
	bc.cfg.DifficultyWindow = 2
	bc.cfg.BlockTime = 60

	// Extend slowly, so the chain gets long and easy.
	slow := extendValidChain(bc, bc.Blocks, 8, 10*time.Minute, "")
	for _, block := range slow {
		if _, err := bc.AcceptBlockWithResult(block); err != nil {
			t.Fatalf("slow block: %v", err)
		}
	}

	for _, b := range bc.Blocks {
		if err := b.save(); err != nil {
			t.Fatalf("save: %v", err)
		}
	}

	blocksDir := filepath.Join(bc.cfg.DataPath, "blocks")
	topIndex := bc.Height()
	if _, err := os.Stat(filepath.Join(blocksDir, fmt.Sprintf("%d.json", topIndex))); err != nil {
		t.Fatalf("expected block %d on disk: %v", topIndex, err)
	}

	// Reorganise onto a shorter branch of harder blocks.
	rival := extendValidChain(bc, bc.Blocks[:2], 5, time.Second, "fast")
	for _, block := range rival {
		if _, err := bc.AcceptBlockWithResult(block); err != nil && !errors.Is(err, ErrWeakerBranch) {
			t.Fatalf("rival block: %v", err)
		}
	}

	if bc.Height() >= topIndex {
		t.Fatalf("expected the chain to shorten, height is %d", bc.Height())
	}

	for index := bc.Height() + 1; index <= topIndex; index++ {
		path := filepath.Join(blocksDir, fmt.Sprintf("%d.json", index))
		if _, err := os.Stat(path); err == nil {
			t.Fatalf("stale block file %d.json survived the reorganisation; a restart "+
				"would resurrect a block that is no longer on the chain", index)
		}
	}
}

// TestReorgIsSafeUnderConcurrency drives AcceptBlock from several goroutines.
func TestReorgIsSafeUnderConcurrency(t *testing.T) {
	bc := forkTestChain(t, 3, 4)

	branchA := extendChain(bc.Blocks[1], 4, 4, "A")
	branchB := extendChain(bc.Blocks[1], 6, 4, "B")

	var wg sync.WaitGroup
	for _, branch := range [][]*Block{branchA, branchB} {
		wg.Add(1)
		go func(blocks []*Block) {
			defer wg.Done()
			for _, b := range blocks {
				_, _ = bc.AcceptBlockWithResult(b)
				_ = bc.ChainWork()
				_ = bc.Height()
			}
		}(branch)
	}
	wg.Wait()

	// Whatever the interleaving, the chain must remain internally consistent.
	assertChainLinks(t, bc)
}

// assertChainLinks verifies index sequencing and hash linkage across the chain.
func assertChainLinks(t *testing.T, bc *Blockchain) {
	t.Helper()

	bc.mux.Lock()
	defer bc.mux.Unlock()

	for i, block := range bc.Blocks {
		if got := int(block.Index.Int64()); got != i {
			t.Fatalf("block at position %d has index %d", i, got)
		}
		if i == 0 {
			continue
		}
		if block.Header.PreviousHash != bc.Blocks[i-1].Hash {
			t.Fatalf("block %d does not link to block %d", i, i-1)
		}
		if block.Hash != block.CalculateHash() {
			t.Fatalf("block %d hash does not match its header", i)
		}
	}

	if len(bc.Blocks) > 0 {
		head := bc.Blocks[len(bc.Blocks)-1]
		if bc.CurrentBlockIndex != int(head.Index.Int64()) {
			t.Fatalf("CurrentBlockIndex %d does not match the head %s",
				bc.CurrentBlockIndex, head.Index.String())
		}
		if bc.NextBlockIndex != bc.CurrentBlockIndex+1 {
			t.Fatalf("NextBlockIndex %d is inconsistent with CurrentBlockIndex %d",
				bc.NextBlockIndex, bc.CurrentBlockIndex)
		}
	}
}

// TestBranchAssemblyRejectsSelfReferentialBlocks guards an infinite walk.
func TestBranchAssemblyRejectsSelfReferentialBlocks(t *testing.T) {
	bc := forkTestChain(t, 2, 4)

	loop := forkTestBlock(3, "placeholder", 4, "loop")
	loop.Header.PreviousHash = loop.Hash // points at itself

	bc.mux.Lock()
	bc.indexMainChainLocked()
	bc.rememberBlockLocked(loop)
	_, err := bc.assembleBranchLocked(loop)
	bc.mux.Unlock()

	if err == nil {
		t.Fatal("a self-referential block must be rejected, not walked forever")
	}
}
