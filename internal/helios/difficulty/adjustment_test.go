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

// TestSafeRatioHandlesDegenerateInput covers the arithmetic that previously
// produced +Inf: an empty adjustment window has AverageBlockTime == 0, and
// int64(+Inf) is undefined in Go, so the resulting difficulty was garbage.
func TestSafeRatioHandlesDegenerateInput(t *testing.T) {
	cases := []struct {
		name     string
		num, den float64
		want     float64
	}{
		{"zero denominator", 1, 0, 1.0},
		{"zero numerator", 0, 1, 1.0},
		{"both zero", 0, 0, 1.0},
		{"normal", 4, 2, 2.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := safeRatio(tc.num, tc.den); got != tc.want {
				t.Fatalf("safeRatio(%v, %v) = %v, want %v", tc.num, tc.den, got, tc.want)
			}
		})
	}
}

func TestCalculateDifficultyWithEmptyWindow(t *testing.T) {
	da := NewDifficultyAdjuster(nil)
	current := big.NewInt(1000)

	// An empty window must leave the difficulty usable rather than producing a
	// value derived from +Inf.
	got := da.CalculateNewDifficulty(current, &WindowMetrics{})
	if got.Sign() <= 0 {
		t.Fatalf("empty window produced a non-positive difficulty: %s", got)
	}
	if got.Cmp(current) != 0 {
		t.Fatalf("empty window should leave difficulty unchanged, got %s want %s", got, current)
	}
}

func TestScaleDifficultyNeverReturnsZero(t *testing.T) {
	// A tiny factor must not truncate the difficulty to zero, which would make
	// every possible hash valid.
	if got := scaleDifficulty(big.NewInt(1), 0.0000001); got.Sign() <= 0 {
		t.Fatalf("scaleDifficulty produced a non-positive difficulty: %s", got)
	}
	if got := scaleDifficulty(nil, 2.0); got.Sign() <= 0 {
		t.Fatalf("nil difficulty should yield a positive default, got %s", got)
	}
}

func TestNetworkHashrateRaisesDifficulty(t *testing.T) {
	cfg := DefaultDifficultyAdjustmentConfig()
	cfg.EnableMultiFactor = true
	cfg.TimeWeight = 0
	cfg.EnergyWeight = 0
	cfg.NetworkWeight = 1
	da := NewDifficultyAdjuster(cfg)

	current := big.NewInt(1000)
	// Hashrate well above target must make mining harder. The network factor was
	// inverted, so more hashrate made it easier -- a runaway feedback loop.
	got := da.CalculateNewDifficulty(current, &WindowMetrics{
		AverageNetworkHashrate: cfg.TargetNetworkHashrate * 3,
	})
	if got.Cmp(current) <= 0 {
		t.Fatalf("expected higher difficulty for above-target hashrate, got %s <= %s", got, current)
	}
}

func TestShouldAdjustDifficultyGuardsZeroWindow(t *testing.T) {
	cfg := DefaultDifficultyAdjustmentConfig()
	cfg.AdjustmentWindow = 0
	da := NewDifficultyAdjuster(cfg)

	// A zero window used to panic with an integer divide by zero.
	if da.ShouldAdjustDifficulty(10) {
		t.Fatal("a zero adjustment window must never request an adjustment")
	}
}

func TestValidateMetricsRejectsNilDifficulty(t *testing.T) {
	da := NewDifficultyAdjuster(nil)
	err := da.ValidateMetrics(&BlockMetrics{
		BlockTime:       time.Second,
		EnergyUsed:      1,
		NetworkHashrate: 1,
		Difficulty:      nil,
	})
	if err == nil {
		t.Fatal("expected an error for nil difficulty rather than a nil dereference")
	}
}
