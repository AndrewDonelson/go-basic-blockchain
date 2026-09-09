package difficulty

import (
	"fmt"
	"math"
	"math/big"
	"time"
)

// DifficultyAdjustmentConfig holds configuration for difficulty adjustment
type DifficultyAdjustmentConfig struct {
	// Basic adjustment parameters
	TargetBlockTime     time.Duration `json:"target_block_time"`     // 10 seconds
	AdjustmentWindow    int           `json:"adjustment_window"`     // 2016 blocks
	MaxAdjustmentFactor float64       `json:"max_adjustment_factor"` // 4.0 (400%)
	MinAdjustmentFactor float64       `json:"min_adjustment_factor"` // 0.25 (25%)

	// Multi-factor parameters
	EnableMultiFactor bool    `json:"enable_multi_factor"` // false initially
	EnergyWeight      float64 `json:"energy_weight"`       // 0.3 (30%)
	NetworkWeight     float64 `json:"network_weight"`      // 0.3 (30%)
	TimeWeight        float64 `json:"time_weight"`         // 0.4 (40%)

	// Energy consumption parameters
	TargetEnergyUsage int64   `json:"target_energy_usage"` // CPU cycles
	EnergyTolerance   float64 `json:"energy_tolerance"`    // 0.1 (10%)

	// Network parameters
	TargetNetworkHashrate int64   `json:"target_network_hashrate"` // hashes per second
	NetworkTolerance      float64 `json:"network_tolerance"`       // 0.1 (10%)

	// Time parameters
	TimeTolerance float64 `json:"time_tolerance"` // 0.1 (10%)
}

// DefaultDifficultyAdjustmentConfig returns the default configuration
func DefaultDifficultyAdjustmentConfig() *DifficultyAdjustmentConfig {
	return &DifficultyAdjustmentConfig{
		TargetBlockTime:       10 * time.Second,
		AdjustmentWindow:      2016,
		MaxAdjustmentFactor:   4.0,
		MinAdjustmentFactor:   0.25,
		EnableMultiFactor:     false,
		EnergyWeight:          0.3,
		NetworkWeight:         0.3,
		TimeWeight:            0.4,
		TargetEnergyUsage:     1000000, // 1M CPU cycles
		EnergyTolerance:       0.1,
		TargetNetworkHashrate: 1000000, // 1M hashes/sec
		NetworkTolerance:      0.1,
		TimeTolerance:         0.1,
	}
}

// DifficultyAdjuster handles difficulty adjustment calculations
type DifficultyAdjuster struct {
	config *DifficultyAdjustmentConfig
}

// NewDifficultyAdjuster creates a new difficulty adjuster
func NewDifficultyAdjuster(config *DifficultyAdjustmentConfig) *DifficultyAdjuster {
	if config == nil {
		config = DefaultDifficultyAdjustmentConfig()
	}
	return &DifficultyAdjuster{config: config}
}

// BlockMetrics holds metrics for a block
type BlockMetrics struct {
	BlockTime       time.Duration `json:"block_time"`
	EnergyUsed      int64         `json:"energy_used"`
	NetworkHashrate int64         `json:"network_hashrate"`
	Difficulty      *big.Int      `json:"difficulty"`
}

// WindowMetrics holds aggregated metrics for the adjustment window
type WindowMetrics struct {
	AverageBlockTime       time.Duration `json:"average_block_time"`
	AverageEnergyUsed      int64         `json:"average_energy_used"`
	AverageNetworkHashrate int64         `json:"average_network_hashrate"`
	BlockCount             int           `json:"block_count"`
}

// CalculateNewDifficulty calculates the new difficulty based on recent block metrics
func (da *DifficultyAdjuster) CalculateNewDifficulty(
	currentDifficulty *big.Int,
	windowMetrics *WindowMetrics,
) *big.Int {

	if da.config.EnableMultiFactor {
		return da.calculateMultiFactorDifficulty(currentDifficulty, windowMetrics)
	}

	return da.calculateSimpleDifficulty(currentDifficulty, windowMetrics)
}

// UNITS: the big.Int handled by this package is a DIFFICULTY SCALAR, not a
// mining target. Larger means harder. This was never written down anywhere, which
// is what made the adjustment direction ambiguous to read; sdk.difficultyTarget
// converts a scalar into the 256-bit target used for the actual hash comparison.
//
// calculateSimpleDifficulty implements basic time-based difficulty adjustment.
//
//	blocks too fast (actual < target) -> factor > 1 -> harder -> slower blocks
//	blocks too slow (actual > target) -> factor < 1 -> easier -> faster blocks
//
// The direction (target/actual) is unchanged; what is fixed here is the arithmetic
// around it: a zero AverageBlockTime produced +Inf, and int64(+Inf) is undefined
// in Go, so an empty adjustment window yielded a garbage difficulty.
func (da *DifficultyAdjuster) calculateSimpleDifficulty(
	currentDifficulty *big.Int,
	windowMetrics *WindowMetrics,
) *big.Int {

	adjustmentFactor := da.clampFactor(safeRatio(
		float64(da.config.TargetBlockTime),
		float64(windowMetrics.AverageBlockTime),
	))

	return scaleDifficulty(currentDifficulty, adjustmentFactor)
}

// safeRatio divides two float64 values, returning 1.0 (no change) when the result
// would be undefined -- a zero denominator, a zero numerator, NaN or Inf.
func safeRatio(numerator, denominator float64) float64 {
	if denominator == 0 || numerator == 0 {
		return 1.0
	}
	ratio := numerator / denominator
	if math.IsNaN(ratio) || math.IsInf(ratio, 0) {
		return 1.0
	}
	return ratio
}

// clampFactor bounds an adjustment factor to the configured limits.
func (da *DifficultyAdjuster) clampFactor(factor float64) float64 {
	if math.IsNaN(factor) {
		return 1.0
	}
	if factor > da.config.MaxAdjustmentFactor {
		return da.config.MaxAdjustmentFactor
	}
	if factor < da.config.MinAdjustmentFactor {
		return da.config.MinAdjustmentFactor
	}
	return factor
}

// scaleDifficulty multiplies a difficulty scalar by a factor using fixed-point
// arithmetic.
//
// The scale is six decimal places rather than the previous three, so small
// adjustments are no longer truncated to a no-op.
func scaleDifficulty(difficulty *big.Int, factor float64) *big.Int {
	if difficulty == nil {
		return big.NewInt(1)
	}

	const scale = 1000000
	scaled := int64(factor * scale)
	if scaled <= 0 {
		scaled = 1
	}

	out := new(big.Int).Set(difficulty)
	out.Mul(out, big.NewInt(scaled))
	out.Div(out, big.NewInt(scale))

	// Difficulty must stay at least 1; zero would accept any hash at all.
	if out.Sign() <= 0 {
		out.SetInt64(1)
	}
	return out
}

// calculateMultiFactorDifficulty implements multi-factor difficulty adjustment
func (da *DifficultyAdjuster) calculateMultiFactorDifficulty(
	currentDifficulty *big.Int,
	windowMetrics *WindowMetrics,
) *big.Int {

	// Each factor is oriented so that "> 1 means make it harder":
	//
	//   time:    blocks faster than target      -> harder  -> target/actual
	//   energy:  more energy than target        -> easier  -> target/actual
	//   network: more hashrate than target      -> harder  -> actual/target
	//
	// The network factor was previously 1/(actual/target), i.e. inverted: adding
	// hashrate to the network made mining *easier*, which is a runaway feedback
	// loop. Time and energy keep their original orientation.
	timeFactor := safeRatio(float64(da.config.TargetBlockTime), float64(windowMetrics.AverageBlockTime))
	energyFactor := safeRatio(float64(da.config.TargetEnergyUsage), float64(windowMetrics.AverageEnergyUsed))
	networkFactor := safeRatio(float64(windowMetrics.AverageNetworkHashrate), float64(da.config.TargetNetworkHashrate))

	// Weighted combination
	totalWeight := da.config.TimeWeight + da.config.EnergyWeight + da.config.NetworkWeight
	if totalWeight == 0 {
		return new(big.Int).Set(currentDifficulty)
	}

	adjustmentFactor := da.clampFactor((timeFactor*da.config.TimeWeight +
		energyFactor*da.config.EnergyWeight +
		networkFactor*da.config.NetworkWeight) / totalWeight)

	return scaleDifficulty(currentDifficulty, adjustmentFactor)
}

// ValidateMetrics validates that metrics are within acceptable ranges
func (da *DifficultyAdjuster) ValidateMetrics(metrics *BlockMetrics) error {
	// Validate block time
	if metrics.BlockTime <= 0 {
		return fmt.Errorf("block time must be positive")
	}

	// Validate energy usage
	if metrics.EnergyUsed <= 0 {
		return fmt.Errorf("energy usage must be positive")
	}

	// Validate network hashrate
	if metrics.NetworkHashrate <= 0 {
		return fmt.Errorf("network hashrate must be positive")
	}

	// Validate difficulty
	if metrics.Difficulty == nil {
		return fmt.Errorf("difficulty must not be nil")
	}
	if metrics.Difficulty.Cmp(big.NewInt(0)) <= 0 {
		return fmt.Errorf("difficulty must be positive")
	}

	return nil
}

// AggregateWindowMetrics aggregates metrics from a window of blocks
func (da *DifficultyAdjuster) AggregateWindowMetrics(blockMetrics []*BlockMetrics) *WindowMetrics {
	if len(blockMetrics) == 0 {
		return &WindowMetrics{}
	}

	var totalBlockTime time.Duration
	var totalEnergyUsed int64
	var totalNetworkHashrate int64

	for _, metrics := range blockMetrics {
		totalBlockTime += metrics.BlockTime
		totalEnergyUsed += metrics.EnergyUsed
		totalNetworkHashrate += metrics.NetworkHashrate
	}

	blockCount := len(blockMetrics)

	return &WindowMetrics{
		AverageBlockTime:       totalBlockTime / time.Duration(blockCount),
		AverageEnergyUsed:      totalEnergyUsed / int64(blockCount),
		AverageNetworkHashrate: totalNetworkHashrate / int64(blockCount),
		BlockCount:             blockCount,
	}
}

// ShouldAdjustDifficulty determines if difficulty should be adjusted
func (da *DifficultyAdjuster) ShouldAdjustDifficulty(blockNumber int) bool {
	// A zero window would panic with a division by zero.
	if da.config.AdjustmentWindow <= 0 {
		return false
	}
	return blockNumber > 0 && blockNumber%da.config.AdjustmentWindow == 0
}

// GetAdjustmentWindowSize returns the size of the adjustment window
func (da *DifficultyAdjuster) GetAdjustmentWindowSize() int {
	return da.config.AdjustmentWindow
}

// UpdateConfig updates the difficulty adjustment configuration
func (da *DifficultyAdjuster) UpdateConfig(newConfig *DifficultyAdjustmentConfig) {
	if newConfig == nil {
		return
	}
	da.config = newConfig
}

// GetConfig returns the current configuration
func (da *DifficultyAdjuster) GetConfig() *DifficultyAdjustmentConfig {
	return da.config
}
