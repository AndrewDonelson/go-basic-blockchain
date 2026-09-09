package sdk

import (
	"errors"
	"math/big"
	"testing"
	"time"
)

// difficultyTestChain builds a chain with a controlled retarget window and
// block spacing, anchored in the past so it can be extended forwards.
func difficultyTestChain(t *testing.T, window, blockTimeSeconds int) *Blockchain {
	t.Helper()

	bc := forkTestChain(t, 0, uint32(genesisDifficulty))
	bc.cfg.DifficultyWindow = window
	bc.cfg.BlockTime = blockTimeSeconds
	return bc
}

// -----------------------------------------------------------------------------
// Scale conversion
// -----------------------------------------------------------------------------

// TestDifficultyExponentRoundTrip guards the conversion that makes retargeting
// meaningful at all.
//
// Header.Difficulty is an exponent (work = 2^d); the adjuster works in linear
// difficulty. Feeding the exponent straight in would take difficulty 4 to 8 on a
// doubling -- 16x the work, not 2x.
func TestDifficultyExponentRoundTrip(t *testing.T) {
	for d := minAcceptableDifficulty; d <= 64; d++ {
		work := BlockWork(uint32(d))
		if got := difficultyFromWork(work); got != d {
			t.Fatalf("exponent %d -> work %s -> exponent %d", d, work, got)
		}
	}

	// Degenerate input must not produce a nonsense exponent.
	if got := difficultyFromWork(nil); got != minAcceptableDifficulty {
		t.Fatalf("nil work gave exponent %d", got)
	}
	if got := difficultyFromWork(big.NewInt(0)); got != minAcceptableDifficulty {
		t.Fatalf("zero work gave exponent %d", got)
	}
	if got := difficultyFromWork(big.NewInt(-5)); got != minAcceptableDifficulty {
		t.Fatalf("negative work gave exponent %d", got)
	}
}

func TestClampDifficulty(t *testing.T) {
	cases := map[int]int{
		-100:                        minAcceptableDifficulty,
		0:                           minAcceptableDifficulty,
		minAcceptableDifficulty:     minAcceptableDifficulty,
		8:                           8,
		maxAcceptableDifficulty:     maxAcceptableDifficulty,
		maxAcceptableDifficulty + 1: maxAcceptableDifficulty,
		100000:                      maxAcceptableDifficulty,
	}
	for in, want := range cases {
		if got := clampDifficulty(in); got != want {
			t.Fatalf("clampDifficulty(%d) = %d, want %d", in, got, want)
		}
	}
}

// -----------------------------------------------------------------------------
// Retargeting
// -----------------------------------------------------------------------------

// TestDifficultyHoldsBetweenRetargets: difficulty only moves on a window
// boundary, so most blocks simply inherit it.
func TestDifficultyHoldsBetweenRetargets(t *testing.T) {
	bc := difficultyTestChain(t, 10, 60)

	blocks := extendValidChain(bc, bc.Blocks, 8, time.Minute, "")
	for _, block := range blocks {
		if int(block.Header.Difficulty) != genesisDifficulty {
			t.Fatalf("block %s changed difficulty to %d away from a retarget boundary",
				block.Index.String(), block.Header.Difficulty)
		}
	}
}

// TestFastBlocksRaiseDifficulty is the core feedback loop.
func TestFastBlocksRaiseDifficulty(t *testing.T) {
	bc := difficultyTestChain(t, 2, 600) // target 10 minutes per block

	// Blocks arriving one second apart are far faster than the target.
	blocks := extendValidChain(bc, bc.Blocks, 6, time.Second, "")

	last := int(blocks[len(blocks)-1].Header.Difficulty)
	if last <= genesisDifficulty {
		t.Fatalf("difficulty did not rise for fast blocks: started at %d, ended at %d",
			genesisDifficulty, last)
	}
}

// TestSlowBlocksLowerDifficulty is the same loop in the other direction.
func TestSlowBlocksLowerDifficulty(t *testing.T) {
	bc := difficultyTestChain(t, 2, 1)
	// Start from a difficulty that has room to fall.
	bc.Blocks[0].Header.Difficulty = 12
	bc.Blocks[0].Hash = bc.Blocks[0].CalculateHash()

	// Blocks ten minutes apart against a one-second target.
	blocks := extendValidChain(bc, bc.Blocks, 6, 10*time.Minute, "")

	last := int(blocks[len(blocks)-1].Header.Difficulty)
	if last >= 12 {
		t.Fatalf("difficulty did not fall for slow blocks: started at 12, ended at %d", last)
	}
	if last < minAcceptableDifficulty {
		t.Fatalf("difficulty fell below the floor: %d", last)
	}
}

// TestRetargetIsBoundedPerStep: each exponent step doubles or halves the work, so
// one anomalous window must not make the chain unmineable or trivial.
func TestRetargetIsBoundedPerStep(t *testing.T) {
	bc := difficultyTestChain(t, 2, 3600) // target one hour

	// A single nanosecond between blocks is an extreme measurement.
	blocks := extendValidChain(bc, bc.Blocks, 4, time.Nanosecond, "")

	previous := genesisDifficulty
	for _, block := range blocks {
		step := int(block.Header.Difficulty) - previous
		if step > maxDifficultyStep || step < -maxDifficultyStep {
			t.Fatalf("block %s moved difficulty by %d, above the %d limit",
				block.Index.String(), step, maxDifficultyStep)
		}
		previous = int(block.Header.Difficulty)
	}
}

// TestDifficultyNeverFallsBelowTheFloor guards against a chain becoming trivially
// mineable, which fork choice would then let a cheap branch dominate.
func TestDifficultyNeverFallsBelowTheFloor(t *testing.T) {
	bc := difficultyTestChain(t, 2, 1)

	blocks := extendValidChain(bc, bc.Blocks, 20, time.Hour, "")
	for _, block := range blocks {
		if int(block.Header.Difficulty) < minAcceptableDifficulty {
			t.Fatalf("block %s declares difficulty %d, below the floor of %d",
				block.Index.String(), block.Header.Difficulty, minAcceptableDifficulty)
		}
	}
}

// TestExpectedDifficultyIsDeterministic: every node must compute the same value
// for the same history, or they cannot agree on whether a block is valid.
func TestExpectedDifficultyIsDeterministic(t *testing.T) {
	bc := difficultyTestChain(t, 3, 120)
	blocks := extendValidChain(bc, bc.Blocks, 12, 30*time.Second, "")

	history := append(append([]*Block{}, bc.Blocks...), blocks...)

	first := bc.ExpectedDifficulty(history)
	for i := 0; i < 50; i++ {
		if got := bc.ExpectedDifficulty(history); got != first {
			t.Fatalf("ExpectedDifficulty is not deterministic: %d vs %d", got, first)
		}
	}
}

// TestExpectedDifficultyHandlesDegenerateHistory covers the boundaries.
func TestExpectedDifficultyHandlesDegenerateHistory(t *testing.T) {
	bc := difficultyTestChain(t, 2, 60)

	if got := bc.ExpectedDifficulty(nil); got != genesisDifficulty {
		t.Fatalf("an empty history should give the genesis difficulty, got %d", got)
	}

	// Identical timestamps carry no information about block rate, so difficulty
	// must hold rather than be derived from a meaningless measurement.
	flat := append([]*Block{}, bc.Blocks...)
	stamp := time.Now().Add(-time.Hour)
	for i := 0; i < 4; i++ {
		b := forkTestBlock(len(flat), flat[len(flat)-1].Hash, uint32(genesisDifficulty), "")
		b.Header.Timestamp = stamp
		b.Hash = b.CalculateHash()
		flat = append(flat, b)
	}
	if got := bc.ExpectedDifficulty(flat); got != genesisDifficulty {
		t.Fatalf("identical timestamps changed the difficulty to %d", got)
	}

	// A history shorter than the window cannot be measured either.
	short := bc.Blocks[:1]
	if got := bc.ExpectedDifficulty(short); got < minAcceptableDifficulty {
		t.Fatalf("a short history gave an unusable difficulty: %d", got)
	}
}

// TestBackwardsTimestampsDoNotBreakRetargeting guards a negative timespan.
func TestBackwardsTimestampsDoNotBreakRetargeting(t *testing.T) {
	bc := difficultyTestChain(t, 2, 60)

	history := append([]*Block{}, bc.Blocks...)
	stamp := time.Now().Add(-time.Hour)
	for i := 0; i < 4; i++ {
		b := forkTestBlock(len(history), history[len(history)-1].Hash, uint32(genesisDifficulty), "")
		// Each block earlier than the last.
		b.Header.Timestamp = stamp.Add(-time.Duration(i) * time.Minute)
		b.Hash = b.CalculateHash()
		history = append(history, b)
	}

	got := bc.ExpectedDifficulty(history)
	if got < minAcceptableDifficulty || got > maxAcceptableDifficulty {
		t.Fatalf("backwards timestamps produced an unusable difficulty: %d", got)
	}
}

// -----------------------------------------------------------------------------
// Enforcement
// -----------------------------------------------------------------------------

// TestBlockDeclaringTheWrongDifficultyIsRejected is what stops a peer choosing an
// easy value. Fork choice weighs branches by the work their blocks claim, so an
// unchecked declaration would let a cheap branch masquerade as heavy.
func TestBlockDeclaringTheWrongDifficultyIsRejected(t *testing.T) {
	bc := difficultyTestChain(t, 10, 60)
	expected := bc.CurrentDifficulty()

	for _, wrong := range []uint32{uint32(expected) + 1, uint32(expected) + 8, uint32(minAcceptableDifficulty)} {
		if int(wrong) == expected {
			continue
		}

		block := NewBlock(nil, bc.HeadHash())
		block.Index = *big.NewInt(int64(bc.Height() + 1))
		block.Header.Difficulty = wrong
		block.Header.Timestamp = bc.Blocks[bc.Height()].Header.Timestamp.Add(time.Minute)
		block.Hash = block.CalculateHash()

		if _, err := bc.AcceptBlockWithResult(block); err == nil {
			t.Fatalf("a block declaring difficulty %d was accepted when %d was required",
				wrong, expected)
		}
	}

	// The correct value is accepted.
	block := NewBlock(nil, bc.HeadHash())
	block.Index = *big.NewInt(int64(bc.Height() + 1))
	block.Header.Difficulty = uint32(expected)
	block.Header.Timestamp = bc.Blocks[bc.Height()].Header.Timestamp.Add(time.Minute)
	block.Hash = block.CalculateHash()

	if _, err := bc.AcceptBlockWithResult(block); err != nil {
		t.Fatalf("a block declaring the required difficulty was rejected: %v", err)
	}
}

// TestBranchDifficultyIsValidatedAgainstItsOwnHistory: two branches can
// legitimately sit at different difficulties, and each must be judged on its own
// ancestry rather than against the main chain.
func TestBranchDifficultyIsValidatedAgainstItsOwnHistory(t *testing.T) {
	bc := difficultyTestChain(t, 2, 600)

	// Extend the main chain slowly.
	slow := extendValidChain(bc, bc.Blocks, 6, time.Hour, "")
	for _, block := range slow {
		if _, err := bc.AcceptBlockWithResult(block); err != nil {
			t.Fatalf("slow block: %v", err)
		}
	}

	// A branch from block 0 with fast blocks retargets differently.
	fast := extendValidChain(bc, bc.Blocks[:1], 8, time.Second, "fast")

	// Its difficulty must diverge from the main chain's, or the test proves nothing.
	// The fixture timestamps are fixed, so this is deterministic rather than a
	// coincidence worth skipping over.
	if fast[len(fast)-1].Header.Difficulty == slow[len(slow)-1].Header.Difficulty {
		t.Fatalf("both branches reached difficulty %d, so this test no longer "+
			"proves that branches are judged on their own ancestry",
			fast[len(fast)-1].Header.Difficulty)
	}

	// And it must be accepted despite differing, because it is correct for that
	// branch's own history.
	accepted := false
	for _, block := range fast {
		_, err := bc.AcceptBlockWithResult(block)
		if err == nil || errors.Is(err, ErrWeakerBranch) {
			accepted = true
			continue
		}
		t.Fatalf("a branch block valid for its own history was rejected: %v", err)
	}
	if !accepted {
		t.Fatal("no branch block was accepted")
	}
}

// TestMinedBlocksDeclareTheRequiredDifficulty closes the loop: what the miner
// stamps is what validation demands.
func TestMinedBlocksDeclareTheRequiredDifficulty(t *testing.T) {
	bc := difficultyTestChain(t, 4, 60)
	bc.useHeliosMining = false

	for i := 0; i < 6; i++ {
		expected := bc.CurrentDifficulty()
		bc.createNewBlock(expected)

		head := bc.GetLatestBlock()
		if head == nil {
			t.Fatal("no block was produced")
		}
		if int(head.Header.Difficulty) != expected {
			t.Fatalf("block %s was mined at difficulty %d but %d was required",
				head.Index.String(), head.Header.Difficulty, expected)
		}
	}
}

// TestCurrentDifficultyMatchesExpected keeps the convenience accessor honest.
func TestCurrentDifficultyMatchesExpected(t *testing.T) {
	bc := difficultyTestChain(t, 3, 60)
	blocks := extendValidChain(bc, bc.Blocks, 5, 10*time.Second, "")
	for _, block := range blocks {
		if _, err := bc.AcceptBlockWithResult(block); err != nil {
			t.Fatalf("accept: %v", err)
		}
	}

	if got, want := bc.CurrentDifficulty(), bc.ExpectedDifficulty(bc.Blocks); got != want {
		t.Fatalf("CurrentDifficulty() = %d, ExpectedDifficulty() = %d", got, want)
	}
}

// TestDifficultyWindowFallsBackWhenUnset guards a zero window causing a division
// by zero or a retarget on every block.
func TestDifficultyWindowFallsBackWhenUnset(t *testing.T) {
	bc := difficultyTestChain(t, 0, 60)

	if got := bc.difficultyWindow(); got != defaultDifficultyWindow {
		t.Fatalf("a zero window should fall back to %d, got %d", defaultDifficultyWindow, got)
	}

	bc.cfg.BlockTime = 0
	if got := bc.targetBlockTime(); got != blockTimeInSec*time.Second {
		t.Fatalf("a zero block time should fall back to %s, got %s",
			blockTimeInSec*time.Second, got)
	}
}

// TestRetargetedWorkIsReflectedInChainWork ties difficulty back to fork choice:
// harder blocks must actually count for more.
func TestRetargetedWorkIsReflectedInChainWork(t *testing.T) {
	easy := []*Block{forkTestBlock(0, "", 4, ""), forkTestBlock(1, "a", 4, "")}
	hard := []*Block{forkTestBlock(0, "", 6, ""), forkTestBlock(1, "a", 6, "")}

	if ChainWork(hard).Cmp(ChainWork(easy)) <= 0 {
		t.Fatal("a retarget upward must increase the chain's cumulative work, " +
			"or fork choice cannot tell the branches apart")
	}
}
