// Package sdk is a software development kit for building blockchain applications.
// File sdk/difficulty.go - retargeting difficulty from chain history
//
// Difficulty used to be a fixed number from the config. internal/helios/difficulty
// was implemented and well tested, and nothing called it -- so a chain always
// mined at whatever `DIFFICULTY` said, regardless of how fast blocks arrived, and
// the cumulative-work fork choice reduced in practice to "longest chain" because
// every block carried identical work.
//
// # Difficulty must be a function of the chain, not of local config
//
// Every node has to agree on what difficulty a given block was required to meet,
// or a peer could simply declare whatever suits it. So the expected difficulty is
// computed from the block's own ancestors -- the timestamps of the preceding
// retarget window -- and a block that declares anything else is rejected.
//
// This also means difficulty is per-branch: two competing branches can legitimately
// be at different difficulties, and each is validated against its own history.
//
// # Exponent versus linear scale
//
// A subtlety worth spelling out, because getting it wrong produces adjustments
// that are wrong by orders of magnitude.
//
// Header.Difficulty here is an EXPONENT: a block at difficulty d must hash below
// 2^(256-d), so it costs about 2^d attempts. internal/helios/difficulty works in
// LINEAR difficulty, where multiplying the value by F multiplies the work by F.
//
// Feeding the exponent straight into the adjuster would take difficulty 4 to 8 on
// a doubling -- which is 16x the work, not 2x. So the exponent is converted to
// linear work (2^d) before adjusting and back afterwards.
package sdk

import (
	"math/big"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/helios/difficulty"
)

const (
	// defaultDifficultyWindow is how many blocks each retarget looks back over.
	//
	// Bitcoin uses 2016 blocks (~2 weeks). This chain targets 20-second blocks and
	// is meant to be observable, so the window is short enough that a retarget can
	// actually be seen happening.
	defaultDifficultyWindow = 10

	// maxDifficultyStep bounds how far one retarget may move the exponent.
	//
	// Each step of the exponent doubles or halves the work, so even a single step
	// is a large move; without a clamp, one anomalous window could make the chain
	// unmineable or trivial.
	maxDifficultyStep = 2

	// maxAcceptableDifficulty is the highest exponent a block may declare.
	// difficultyTarget clamps at 255, above which the target underflows.
	maxAcceptableDifficulty = 240
)

// difficultyWindow returns the configured retarget window.
func (bc *Blockchain) difficultyWindow() int {
	if bc.cfg != nil && bc.cfg.DifficultyWindow > 0 {
		return bc.cfg.DifficultyWindow
	}
	return defaultDifficultyWindow
}

// targetBlockTime returns the block interval the chain aims for.
func (bc *Blockchain) targetBlockTime() time.Duration {
	if bc.cfg != nil && bc.cfg.BlockTime > 0 {
		return time.Duration(bc.cfg.BlockTime) * time.Second
	}
	return blockTimeInSec * time.Second
}

// difficultyFromWork converts linear work back to an exponent.
//
// Work is 2^d, so the exponent is the position of the highest set bit.
func difficultyFromWork(work *big.Int) int {
	if work == nil || work.Sign() <= 0 {
		return minAcceptableDifficulty
	}
	return work.BitLen() - 1
}

// clampDifficulty keeps an exponent inside the usable range.
func clampDifficulty(d int) int {
	if d < minAcceptableDifficulty {
		return minAcceptableDifficulty
	}
	if d > maxAcceptableDifficulty {
		return maxAcceptableDifficulty
	}
	return d
}

// ExpectedDifficulty returns the difficulty the next block after `chain` must
// declare.
//
// `chain` is the ancestry the candidate block builds on, so this works for the
// main chain and for a competing branch alike.
func (bc *Blockchain) ExpectedDifficulty(chain []*Block) int {
	if len(chain) == 0 {
		// The genesis block sets the starting point.
		return genesisDifficulty
	}

	previous := blockDifficulty(chain[len(chain)-1], genesisDifficulty)
	window := bc.difficultyWindow()

	// The height of the block being produced.
	height := int(chain[len(chain)-1].Index.Int64()) + 1

	// Only retarget on a window boundary; in between, difficulty carries forward.
	if height%window != 0 {
		return clampDifficulty(previous)
	}

	// Need a full window of history to measure a timespan.
	if len(chain) < window+1 {
		return clampDifficulty(previous)
	}

	recent := chain[len(chain)-(window+1):]
	elapsed := recent[len(recent)-1].Header.Timestamp.Sub(recent[0].Header.Timestamp)
	if elapsed <= 0 {
		// Non-monotonic or identical timestamps tell us nothing; leave it alone
		// rather than deriving an adjustment from a meaningless measurement.
		return clampDifficulty(previous)
	}

	average := elapsed / time.Duration(window)
	return bc.retarget(previous, average)
}

// retarget computes a new difficulty exponent from the observed block interval.
func (bc *Blockchain) retarget(previous int, averageBlockTime time.Duration) int {
	adjuster := bc.difficultyAdjuster
	if adjuster == nil {
		adjuster = difficulty.NewDifficultyAdjuster(bc.difficultyConfig())
	}

	// Convert the exponent to linear work, adjust, convert back. See the package
	// comment: the two scales are not interchangeable.
	//nolint:gosec // clampDifficulty bounds the value to [1, maxAcceptableDifficulty]
	currentWork := BlockWork(uint32(clampDifficulty(previous)))
	newWork := adjuster.CalculateNewDifficulty(currentWork, &difficulty.WindowMetrics{
		AverageBlockTime: averageBlockTime,
		BlockCount:       bc.difficultyWindow(),
	})

	next := difficultyFromWork(newWork)

	// Bound the per-retarget movement.
	if next > previous+maxDifficultyStep {
		next = previous + maxDifficultyStep
	}
	if next < previous-maxDifficultyStep {
		next = previous - maxDifficultyStep
	}

	next = clampDifficulty(next)
	if next != previous {
		LogInfof("Difficulty retarget: %d -> %d (average block time %s, target %s)",
			previous, next, averageBlockTime.Round(time.Millisecond), bc.targetBlockTime())
	}
	return next
}

// difficultyConfig builds the adjuster configuration from the chain config.
func (bc *Blockchain) difficultyConfig() *difficulty.DifficultyAdjustmentConfig {
	cfg := difficulty.DefaultDifficultyAdjustmentConfig()
	cfg.TargetBlockTime = bc.targetBlockTime()
	cfg.AdjustmentWindow = bc.difficultyWindow()
	return cfg
}

// expectedDifficultyForNextLocked returns the difficulty for a block extending
// the current main chain. The caller must hold bc.mux.
func (bc *Blockchain) expectedDifficultyForNextLocked() int {
	return bc.ExpectedDifficulty(bc.Blocks)
}

// CurrentDifficulty returns the difficulty the next mined block will use.
func (bc *Blockchain) CurrentDifficulty() int {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	return bc.expectedDifficultyForNextLocked()
}
