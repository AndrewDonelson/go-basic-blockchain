package difficulty

import (
	"fmt"
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

// calculateSimpleDifficulty implements basic time-based difficulty adjustment
func (da *DifficultyAdjuster) calculateSimpleDifficulty(
	currentDifficulty *big.Int,
	windowMetrics *WindowMetrics,
) *big.Int {

	// Calculate time ratio
	timeRatio := float64(windowMetrics.AverageBlockTime) / float64(da.config.TargetBlockTime)

	// Calculate adjustment factor
	adjustmentFactor := 1.0 / timeRatio

	// Apply limits
	if adjustmentFactor > da.config.MaxAdjustmentFactor {
		adjustmentFactor = da.config.MaxAdjustmentFactor
	}
	if adjustmentFactor < da.config.MinAdjustmentFactor {
		adjustmentFactor = da.config.MinAdjustmentFactor
	}

	// Calculate new difficulty
	newDifficulty := new(big.Int).Set(currentDifficulty)
	newDifficulty.Mul(newDifficulty, big.NewInt(int64(adjustmentFactor*1000)))
	newDifficulty.Div(newDifficulty, big.NewInt(1000))

	return newDifficulty
}

// calculateMultiFactorDifficulty implements multi-factor difficulty adjustment
func (da *DifficultyAdjuster) calculateMultiFactorDifficulty(
	currentDifficulty *big.Int,
	windowMetrics *WindowMetrics,
) *big.Int {

	// Calculate time factor
	timeRatio := float64(windowMetrics.AverageBlockTime) / float64(da.config.TargetBlockTime)
	timeFactor := 1.0 / timeRatio

	// Calculate energy factor
	energyRatio := float64(windowMetrics.AverageEnergyUsed) / float64(da.config.TargetEnergyUsage)
	energyFactor := 1.0 / energyRatio

	// Calculate network factor
	networkRatio := float64(windowMetrics.AverageNetworkHashrate) / float64(da.config.TargetNetworkHashrate)
	networkFactor := 1.0 / networkRatio

	// Weighted combination
	totalWeight := da.config.TimeWeight + da.config.EnergyWeight + da.config.NetworkWeight
	adjustmentFactor := (timeFactor*da.config.TimeWeight +
		energyFactor*da.config.EnergyWeight +
		networkFactor*da.config.NetworkWeight) / totalWeight

	// Apply limits
	if adjustmentFactor > da.config.MaxAdjustmentFactor {
		adjustmentFactor = da.config.MaxAdjustmentFactor
	}
	if adjustmentFactor < da.config.MinAdjustmentFactor {
		adjustmentFactor = da.config.MinAdjustmentFactor
	}

	// Calculate new difficulty
	newDifficulty := new(big.Int).Set(currentDifficulty)
	newDifficulty.Mul(newDifficulty, big.NewInt(int64(adjustmentFactor*1000)))
	newDifficulty.Div(newDifficulty, big.NewInt(1000))

	return newDifficulty
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
	return blockNumber%da.config.AdjustmentWindow == 0
}

// GetAdjustmentWindowSize returns the size of the adjustment window
func (da *DifficultyAdjuster) GetAdjustmentWindowSize() int {
	return da.config.AdjustmentWindow
}

// UpdateConfig updates the difficulty adjustment configuration
func (da *DifficultyAdjuster) UpdateConfig(newConfig *DifficultyAdjustmentConfig) {
	da.config = newConfig
}

// GetConfig returns the current configuration
func (da *DifficultyAdjuster) GetConfig() *DifficultyAdjustmentConfig {
	return da.config
}
