package difficulty

import (
	"math/big"
	"testing"
	"time"
)

func TestDefaultDifficultyAdjustmentConfig(t *testing.T) {
	cfg := DefaultDifficultyAdjustmentConfig()
	if cfg == nil {
		t.Fatal("expected default config")
	}
	if cfg.TargetBlockTime <= 0 || cfg.AdjustmentWindow <= 0 {
		t.Fatalf("unexpected default config values: %+v", cfg)
	}
}

func TestNewDifficultyAdjusterUsesDefaultOnNil(t *testing.T) {
	da := NewDifficultyAdjuster(nil)
	if da == nil || da.GetConfig() == nil {
		t.Fatal("expected adjuster with config")
	}
}

func TestCalculateSimpleDifficultyIncreaseAndClamp(t *testing.T) {
	cfg := DefaultDifficultyAdjustmentConfig()
	cfg.EnableMultiFactor = false
	da := NewDifficultyAdjuster(cfg)

	current := big.NewInt(1000)
	metrics := &WindowMetrics{AverageBlockTime: cfg.TargetBlockTime / 10}
	newDiff := da.CalculateNewDifficulty(current, metrics)

	if newDiff.Cmp(current) <= 0 {
		t.Fatalf("expected increased difficulty, got %s <= %s", newDiff, current)
	}

	metrics2 := &WindowMetrics{AverageBlockTime: cfg.TargetBlockTime * 100}
	newDiff2 := da.CalculateNewDifficulty(current, metrics2)
	if newDiff2.Cmp(current) >= 0 {
		t.Fatalf("expected decreased difficulty, got %s >= %s", newDiff2, current)
	}
}

func TestCalculateMultiFactorDifficulty(t *testing.T) {
	cfg := DefaultDifficultyAdjustmentConfig()
	cfg.EnableMultiFactor = true
	cfg.TimeWeight = 0.5
	cfg.EnergyWeight = 0.25
	cfg.NetworkWeight = 0.25
	da := NewDifficultyAdjuster(cfg)

	current := big.NewInt(2000)
	metrics := &WindowMetrics{
		AverageBlockTime:       cfg.TargetBlockTime / 2,
		AverageEnergyUsed:      cfg.TargetEnergyUsage / 2,
		AverageNetworkHashrate: cfg.TargetNetworkHashrate / 2,
	}

	newDiff := da.CalculateNewDifficulty(current, metrics)
	if newDiff.Cmp(current) <= 0 {
		t.Fatalf("expected increased difficulty, got %s <= %s", newDiff, current)
	}
}

func TestValidateMetrics(t *testing.T) {
	da := NewDifficultyAdjuster(nil)
	valid := &BlockMetrics{
		BlockTime:       time.Second,
		EnergyUsed:      1,
		NetworkHashrate: 1,
		Difficulty:      big.NewInt(1),
	}
	if err := da.ValidateMetrics(valid); err != nil {
		t.Fatalf("expected valid metrics, got err: %v", err)
	}

	cases := []BlockMetrics{
		{BlockTime: 0, EnergyUsed: 1, NetworkHashrate: 1, Difficulty: big.NewInt(1)},
		{BlockTime: time.Second, EnergyUsed: 0, NetworkHashrate: 1, Difficulty: big.NewInt(1)},
		{BlockTime: time.Second, EnergyUsed: 1, NetworkHashrate: 0, Difficulty: big.NewInt(1)},
		{BlockTime: time.Second, EnergyUsed: 1, NetworkHashrate: 1, Difficulty: big.NewInt(0)},
	}
	for i, c := range cases {
		c := c
		if err := da.ValidateMetrics(&c); err == nil {
			t.Fatalf("expected validation error for case %d", i)
		}
	}
}

func TestAggregateWindowMetricsAndWindowHelpers(t *testing.T) {
	da := NewDifficultyAdjuster(nil)
	if wm := da.AggregateWindowMetrics(nil); wm == nil {
		t.Fatal("expected non-nil metrics for empty input")
	}

	in := []*BlockMetrics{
		{BlockTime: 2 * time.Second, EnergyUsed: 10, NetworkHashrate: 20},
		{BlockTime: 4 * time.Second, EnergyUsed: 30, NetworkHashrate: 40},
	}
	out := da.AggregateWindowMetrics(in)
	if out.BlockCount != 2 {
		t.Fatalf("expected block count 2, got %d", out.BlockCount)
	}
	if out.AverageBlockTime != 3*time.Second {
		t.Fatalf("expected avg block time 3s, got %s", out.AverageBlockTime)
	}
	if out.AverageEnergyUsed != 20 || out.AverageNetworkHashrate != 30 {
		t.Fatalf("unexpected averages: %+v", out)
	}

	if !da.ShouldAdjustDifficulty(da.GetAdjustmentWindowSize()) {
		t.Fatal("expected adjustment at exact window boundary")
	}
	if da.ShouldAdjustDifficulty(1) && da.GetAdjustmentWindowSize() != 1 {
		t.Fatal("unexpected adjustment for block 1")
	}
}

func TestUpdateConfig(t *testing.T) {
	da := NewDifficultyAdjuster(nil)
	cfg := DefaultDifficultyAdjustmentConfig()
	cfg.AdjustmentWindow = 99
	da.UpdateConfig(cfg)
	if da.GetConfig().AdjustmentWindow != 99 {
		t.Fatalf("expected updated window 99, got %d", da.GetConfig().AdjustmentWindow)
	}
}
