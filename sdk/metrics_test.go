package sdk

import (
	"math"
	"strings"
	"sync"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// Counters
// -----------------------------------------------------------------------------

func TestCountersOnlyIncrease(t *testing.T) {
	m := NewMetrics()

	m.Inc("blocks_mined")
	m.Inc("blocks_mined")
	m.Add("tx_submitted", 5)

	snap := m.Snapshot()
	if snap["blocks_mined"] != 2 {
		t.Fatalf("blocks_mined = %v, want 2", snap["blocks_mined"])
	}
	if snap["tx_submitted"] != 5 {
		t.Fatalf("tx_submitted = %v, want 5", snap["tx_submitted"])
	}
}

// TestUnknownCounterIsIgnored: metrics must never be able to take a node down.
func TestUnknownCounterIsIgnored(t *testing.T) {
	m := NewMetrics()
	m.Inc("no_such_metric")
	m.Add("also_not_real", 99)

	if _, ok := m.Snapshot()["no_such_metric"]; ok {
		t.Fatal("an unknown metric appeared in the snapshot")
	}
}

// TestNilMetricsIsSafe: a Blockchain built directly in a test has no recorder
// until Metrics() runs, and no call site should have to check.
func TestNilMetricsIsSafe(t *testing.T) {
	var m *Metrics

	m.Inc("blocks_mined")
	m.Add("tx_mined", 3)
	m.RecordBlock(time.Now(), 4)

	if got := m.HashRate(); got != 0 {
		t.Fatalf("nil recorder returned a hash rate of %v", got)
	}
	if got := m.AverageBlockTime(); got != 0 {
		t.Fatalf("nil recorder returned a block time of %v", got)
	}
	if got := m.Uptime(); got != 0 {
		t.Fatalf("nil recorder returned an uptime of %v", got)
	}
	if got := m.Snapshot(); len(got) != 0 {
		t.Fatalf("nil recorder returned %d metrics", len(got))
	}
	if got := m.Prometheus("gbb", nil); got != "" {
		t.Fatalf("nil recorder rendered output: %q", got)
	}
}

// -----------------------------------------------------------------------------
// Hash rate
// -----------------------------------------------------------------------------

// TestHashRateReflectsWorkAndTime is the headline number, which used to be a
// hardcoded zero.
func TestHashRateReflectsWorkAndTime(t *testing.T) {
	m := NewMetrics()

	// Difficulty 10 means 2^10 = 1024 expected hashes per block, one block a
	// second, so roughly 1024 hashes per second.
	base := time.Now().Add(-time.Hour)
	for i := 0; i < 5; i++ {
		m.RecordBlock(base.Add(time.Duration(i)*time.Second), 10)
	}

	rate := m.HashRate()
	if math.Abs(rate-1024) > 1 {
		t.Fatalf("hash rate = %v, want about 1024", rate)
	}
}

// TestHashRateScalesWithDifficulty: difficulty is an exponent, so one more step
// must double the estimated work, not add one hash.
func TestHashRateScalesWithDifficulty(t *testing.T) {
	rateAt := func(difficulty int) float64 {
		m := NewMetrics()
		base := time.Now().Add(-time.Hour)
		for i := 0; i < 5; i++ {
			m.RecordBlock(base.Add(time.Duration(i)*time.Second), difficulty)
		}
		return m.HashRate()
	}

	low, high := rateAt(10), rateAt(11)
	if math.Abs(high-2*low) > 1 {
		t.Fatalf("difficulty 11 gave %v and difficulty 10 gave %v; one step should double the work",
			high, low)
	}
}

// TestHashRateIgnoresDegenerateIntervals: identical or backwards timestamps carry
// no information about block rate and would divide badly.
func TestHashRateIgnoresDegenerateIntervals(t *testing.T) {
	m := NewMetrics()
	stamp := time.Now().Add(-time.Hour)

	for i := 0; i < 5; i++ {
		m.RecordBlock(stamp, 10) // all identical
	}
	if got := m.HashRate(); got != 0 {
		t.Fatalf("identical timestamps produced a hash rate of %v", got)
	}

	for i := 0; i < 5; i++ {
		m.RecordBlock(stamp.Add(-time.Duration(i)*time.Minute), 10) // backwards
	}
	rate := m.HashRate()
	if math.IsInf(rate, 0) || math.IsNaN(rate) || rate < 0 {
		t.Fatalf("backwards timestamps produced an unusable hash rate: %v", rate)
	}

	m.RecordBlock(time.Time{}, 10) // zero time is ignored outright
}

// TestHashRateWindowIsBounded keeps the recorder from growing without limit on a
// long-running node.
func TestHashRateWindowIsBounded(t *testing.T) {
	m := NewMetrics()
	base := time.Now().Add(-100 * time.Hour)
	for i := 0; i < hashRateWindow*10; i++ {
		m.RecordBlock(base.Add(time.Duration(i)*time.Second), 8)
	}

	m.mu.RLock()
	held := len(m.blockTimes)
	difficulties := len(m.blockDifficulties)
	m.mu.RUnlock()

	if held > hashRateWindow {
		t.Fatalf("the recorder holds %d intervals, above the %d window", held, hashRateWindow)
	}
	if difficulties != held {
		t.Fatalf("%d intervals but %d difficulties; they must stay in step",
			held, difficulties)
	}
}

// TestHashRateSaturatesRatherThanOverflowing: an absurd difficulty must not make
// the whole average +Inf.
func TestHashRateSaturatesRatherThanOverflowing(t *testing.T) {
	if got := exp2(5000); math.IsInf(got, 0) {
		t.Fatal("exp2 overflowed to +Inf instead of saturating")
	}
	if got := exp2(-5); got != 0 {
		t.Fatalf("exp2 of a negative exponent = %v, want 0", got)
	}

	m := NewMetrics()
	base := time.Now().Add(-time.Hour)
	for i := 0; i < 3; i++ {
		m.RecordBlock(base.Add(time.Duration(i)*time.Second), 5000)
	}
	if rate := m.HashRate(); math.IsInf(rate, 0) || math.IsNaN(rate) {
		t.Fatalf("hash rate is %v", rate)
	}
}

func TestAverageBlockTime(t *testing.T) {
	m := NewMetrics()
	if got := m.AverageBlockTime(); got != 0 {
		t.Fatalf("an empty recorder reported %v", got)
	}

	base := time.Now().Add(-time.Hour)
	for i := 0; i < 4; i++ {
		m.RecordBlock(base.Add(time.Duration(i)*10*time.Second), 4)
	}
	if got := m.AverageBlockTime(); got != 10*time.Second {
		t.Fatalf("average block time = %v, want 10s", got)
	}
}

// -----------------------------------------------------------------------------
// Exposition format
// -----------------------------------------------------------------------------

// TestPrometheusOutputIsWellFormed: a malformed line makes a scraper drop the
// whole payload, not just that series.
func TestPrometheusOutputIsWellFormed(t *testing.T) {
	m := NewMetrics()
	m.Inc("blocks_mined")
	m.Add("tx_submitted", 42)

	body := m.Prometheus("gbb", map[string]float64{"chain_height": 7})

	for _, want := range []string{
		"# TYPE gbb_blocks_mined counter",
		"gbb_blocks_mined 1",
		"gbb_tx_submitted 42",
		"gbb_chain_height 7",
		"# HELP gbb_blocks_mined",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("output is missing %q:\n%s", want, body)
		}
	}

	for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
		if strings.HasPrefix(line, "#") {
			continue
		}
		if len(strings.Fields(line)) != 2 {
			t.Fatalf("malformed sample line %q", line)
		}
	}
}

// TestCountersAndGaugesAreTypedCorrectly: mislabelling a counter as a gauge loses
// rate() entirely for whoever scrapes it.
func TestCountersAndGaugesAreTypedCorrectly(t *testing.T) {
	body := NewMetrics().Prometheus("gbb", nil)

	if !strings.Contains(body, "# TYPE gbb_blocks_mined counter") {
		t.Fatal("blocks_mined is not typed as a counter")
	}
	if !strings.Contains(body, "# TYPE gbb_hash_rate gauge") {
		t.Fatal("hash_rate is not typed as a gauge")
	}
	if !strings.Contains(body, "# TYPE gbb_uptime_seconds gauge") {
		t.Fatal("uptime_seconds is not typed as a gauge")
	}
}

// TestPrometheusOutputIsStable: unstable ordering makes two scrapes needlessly
// different and the output hard to diff.
func TestPrometheusOutputIsStable(t *testing.T) {
	m := NewMetrics()
	m.Inc("blocks_mined")

	// Compare the order of the series, not the whole payload: uptime_seconds
	// legitimately differs between renders.
	order := func() []string {
		body := m.Prometheus("gbb", map[string]float64{"a": 1, "b": 2, "c": 3})
		var names []string
		for _, line := range strings.Split(strings.TrimSpace(body), "\n") {
			if strings.HasPrefix(line, "#") {
				continue
			}
			names = append(names, strings.Fields(line)[0])
		}
		return names
	}

	first := order()
	for i := 0; i < 20; i++ {
		again := order()
		if len(again) != len(first) {
			t.Fatalf("series count varies: %d vs %d", len(again), len(first))
		}
		for j := range first {
			if again[j] != first[j] {
				t.Fatalf("metric ordering varies at %d: %s vs %s", j, again[j], first[j])
			}
		}
	}
}

func TestPrometheusDefaultsThePrefix(t *testing.T) {
	body := NewMetrics().Prometheus("", nil)
	if !strings.Contains(body, "gbb_blocks_mined") {
		t.Fatalf("an empty prefix did not default to gbb:\n%s", body)
	}
}

func TestFormatMetricValue(t *testing.T) {
	cases := map[float64]string{
		0: "0", 1: "1", 42: "42", -3: "-3",
		1.5: "1.5", 0.25: "0.25",
	}
	for in, want := range cases {
		if got := formatMetricValue(in); got != want {
			t.Fatalf("formatMetricValue(%v) = %q, want %q", in, got, want)
		}
	}
}

// -----------------------------------------------------------------------------
// Concurrency
// -----------------------------------------------------------------------------

// TestMetricsAreSafeUnderConcurrency: these are written from the mining loop, the
// accept path and the P2P handshake at once.
func TestMetricsAreSafeUnderConcurrency(t *testing.T) {
	m := NewMetrics()
	base := time.Now().Add(-time.Hour)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				m.Inc("blocks_accepted")
				m.RecordBlock(base.Add(time.Duration(worker*1000+j)*time.Millisecond), 6)
				_ = m.HashRate()
				_ = m.Snapshot()
				_ = m.Prometheus("gbb", nil)
			}
		}(i)
	}
	wg.Wait()

	if got := m.Snapshot()["blocks_accepted"]; got != 1600 {
		t.Fatalf("blocks_accepted = %v, want 1600", got)
	}
}

// -----------------------------------------------------------------------------
// Wiring
// -----------------------------------------------------------------------------

// TestChainMetricsRecordAcceptedBlocks proves the counters are actually connected
// to the code paths, not merely correct in isolation.
func TestChainMetricsRecordAcceptedBlocks(t *testing.T) {
	bc := forkTestChain(t, 0, uint32(genesisDifficulty))

	blocks := extendValidChain(bc, bc.Blocks, 3, time.Minute, "")
	for _, block := range blocks {
		if _, err := bc.AcceptBlockWithResult(block); err != nil {
			t.Fatalf("accept: %v", err)
		}
	}

	snap := bc.Metrics().Snapshot()
	if snap["blocks_accepted"] != 3 {
		t.Fatalf("blocks_accepted = %v, want 3", snap["blocks_accepted"])
	}
	if bc.Metrics().HashRate() <= 0 {
		t.Fatal("accepting blocks did not produce a hash rate")
	}
}

// TestChainMetricsRecordRejections, and specifically that a duplicate is not
// counted as one -- on a real network every block arrives from several peers.
func TestChainMetricsRecordRejections(t *testing.T) {
	bc := forkTestChain(t, 0, uint32(genesisDifficulty))

	blocks := extendValidChain(bc, bc.Blocks, 1, time.Minute, "")
	good := blocks[0]
	if _, err := bc.AcceptBlockWithResult(good); err != nil {
		t.Fatalf("accept: %v", err)
	}

	// The same block again is a duplicate, not a rejection.
	_, _ = bc.AcceptBlockWithResult(good)
	if got := bc.Metrics().Snapshot()["blocks_rejected"]; got != 0 {
		t.Fatalf("a duplicate block counted as a rejection (%v)", got)
	}

	// A genuinely invalid block does count.
	bad := extendValidChain(bc, bc.Blocks, 1, time.Minute, "bad")[0]
	bad.Header.Difficulty = 200 // not what its history requires
	bad.Hash = bad.CalculateHash()
	if _, err := bc.AcceptBlockWithResult(bad); err == nil {
		t.Fatal("an invalid block was accepted")
	}
	if got := bc.Metrics().Snapshot()["blocks_rejected"]; got != 1 {
		t.Fatalf("blocks_rejected = %v, want 1", got)
	}
}

// TestChainMetricsRecordMempoolActivity.
func TestChainMetricsRecordMempoolActivity(t *testing.T) {
	bc := mempoolTestChain(t, 2)

	// A coinbase can never be submitted, so this is a rejection.
	w := coinbaseTestWallet(t, "metrics-coinbase")
	cb, err := NewCoinbaseTransaction(w, w, bc.cfg)
	if err != nil {
		t.Fatalf("coinbase: %v", err)
	}
	if bc.AddTransactionLocal(cb) {
		t.Fatal("a coinbase was admitted")
	}

	if got := bc.Metrics().Snapshot()["tx_rejected"]; got != 1 {
		t.Fatalf("tx_rejected = %v, want 1", got)
	}
}
