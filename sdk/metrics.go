// Package sdk is a software development kit for building blockchain applications.
// File sdk/metrics.go - Node observability: counters, gauges and hash rate.
package sdk

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// hashRateWindow is how many recent block times feed the hash rate estimate.
//
// A single interval is far too noisy to be useful -- proof of work is a Poisson
// process, so consecutive blocks routinely differ by an order of magnitude. A
// short window smooths that without hiding a real change in network power.
const hashRateWindow = 20

// Metrics records what a node is doing, for the /metrics endpoint and the
// progress display.
//
// Everything here used to be either absent or hardcoded: the status struct
// reported `HashRate: 0, // TODO` and there was no counter of any kind, so a
// running node could not be measured at all.
type Metrics struct {
	mu sync.RWMutex

	startedAt time.Time

	// Counters only ever increase, which is what makes them meaningful to scrape:
	// a collector can take the difference between two samples without needing to
	// know whether it missed one.
	blocksMined     uint64
	blocksAccepted  uint64
	blocksRejected  uint64
	reorgs          uint64
	reorgBlocks     uint64
	txSubmitted     uint64
	txRejected      uint64
	txMined         uint64
	mempoolEvicted  uint64
	peersConnected  uint64
	peerAuthFailed  uint64
	syncPasses      uint64
	syncBlocksPulls uint64

	// blockTimes holds the most recent inter-block intervals, newest last.
	blockTimes []time.Duration
	// blockDifficulties parallels blockTimes.
	blockDifficulties []int
	lastBlockAt       time.Time
}

// NewMetrics returns an initialised metrics recorder.
func NewMetrics() *Metrics {
	return &Metrics{startedAt: time.Now()}
}

// counter increments a named counter. Unknown names are ignored rather than
// panicking: metrics must never be able to take a node down.
func (m *Metrics) counter(name string, delta uint64) {
	if m == nil {
		return
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	switch name {
	case "blocks_mined":
		m.blocksMined += delta
	case "blocks_accepted":
		m.blocksAccepted += delta
	case "blocks_rejected":
		m.blocksRejected += delta
	case "reorgs":
		m.reorgs += delta
	case "reorg_blocks":
		m.reorgBlocks += delta
	case "tx_submitted":
		m.txSubmitted += delta
	case "tx_rejected":
		m.txRejected += delta
	case "tx_mined":
		m.txMined += delta
	case "mempool_evicted":
		m.mempoolEvicted += delta
	case "peers_connected":
		m.peersConnected += delta
	case "peer_auth_failed":
		m.peerAuthFailed += delta
	case "sync_passes":
		m.syncPasses += delta
	case "sync_blocks_pulled":
		m.syncBlocksPulls += delta
	}
}

// Inc increments a counter by one.
func (m *Metrics) Inc(name string) { m.counter(name, 1) }

// Add increments a counter by delta.
func (m *Metrics) Add(name string, delta uint64) { m.counter(name, delta) }

// RecordBlock notes that a block joined the chain at a given difficulty.
func (m *Metrics) RecordBlock(at time.Time, difficulty int) {
	if m == nil || at.IsZero() {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.lastBlockAt.IsZero() {
		interval := at.Sub(m.lastBlockAt)
		// A non-positive interval carries no information about block rate and
		// would divide badly; skip it rather than poisoning the average.
		if interval > 0 {
			m.blockTimes = append(m.blockTimes, interval)
			m.blockDifficulties = append(m.blockDifficulties, difficulty)
			if len(m.blockTimes) > hashRateWindow {
				m.blockTimes = m.blockTimes[len(m.blockTimes)-hashRateWindow:]
				m.blockDifficulties = m.blockDifficulties[len(m.blockDifficulties)-hashRateWindow:]
			}
		}
	}
	m.lastBlockAt = at
}

// HashRate estimates hashes per second from recent blocks.
//
// Expected work for difficulty d is 2^d hashes, so the estimate is the mean of
// 2^d / interval across the window. Averaging the per-block rates rather than
// dividing total work by total time keeps a single very fast block from
// dominating, and handles a window whose difficulty changed partway through.
func (m *Metrics) HashRate() float64 {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.blockTimes) == 0 {
		return 0
	}

	var total float64
	var counted int
	for i, interval := range m.blockTimes {
		seconds := interval.Seconds()
		if seconds <= 0 {
			continue
		}
		difficulty := 0
		if i < len(m.blockDifficulties) {
			difficulty = m.blockDifficulties[i]
		}
		// 2^d as a float. Difficulty is clamped well below the range where this
		// overflows, but guard anyway: +Inf would make the whole average useless.
		work := exp2(difficulty)
		if work <= 0 {
			continue
		}
		rate := work / seconds
		// Belt and braces: a sample that is not a finite number must not be
		// allowed to make the whole average useless.
		if math.IsInf(rate, 0) || math.IsNaN(rate) {
			continue
		}
		total += rate
		counted++
	}
	if counted == 0 {
		return 0
	}
	return total / float64(counted)
}

// exp2 returns 2^n as a float64, saturating rather than overflowing to +Inf.
//
// The cap is maxAcceptableDifficulty rather than float64's limit of 1023. No
// valid block can declare more than that, and 2^1023 is close enough to the top
// of the range that dividing it by a sub-second interval overflows to +Inf --
// which would poison the whole average, not just that sample.
func exp2(n int) float64 {
	if n < 0 {
		return 0
	}
	if n > maxAcceptableDifficulty {
		n = maxAcceptableDifficulty
	}
	out := 1.0
	for i := 0; i < n; i++ {
		out *= 2
	}
	return out
}

// AverageBlockTime returns the mean interval over the window.
func (m *Metrics) AverageBlockTime() time.Duration {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.blockTimes) == 0 {
		return 0
	}
	var total time.Duration
	for _, d := range m.blockTimes {
		total += d
	}
	return total / time.Duration(len(m.blockTimes))
}

// Uptime returns how long the recorder has existed.
func (m *Metrics) Uptime() time.Duration {
	if m == nil {
		return 0
	}
	m.mu.RLock()
	defer m.mu.RUnlock()
	return time.Since(m.startedAt)
}

// Snapshot returns every counter and gauge as a flat map.
func (m *Metrics) Snapshot() map[string]float64 {
	if m == nil {
		return map[string]float64{}
	}

	m.mu.RLock()
	out := map[string]float64{
		"blocks_mined":       float64(m.blocksMined),
		"blocks_accepted":    float64(m.blocksAccepted),
		"blocks_rejected":    float64(m.blocksRejected),
		"reorgs":             float64(m.reorgs),
		"reorg_blocks":       float64(m.reorgBlocks),
		"tx_submitted":       float64(m.txSubmitted),
		"tx_rejected":        float64(m.txRejected),
		"tx_mined":           float64(m.txMined),
		"mempool_evicted":    float64(m.mempoolEvicted),
		"peers_connected":    float64(m.peersConnected),
		"peer_auth_failed":   float64(m.peerAuthFailed),
		"sync_passes":        float64(m.syncPasses),
		"sync_blocks_pulled": float64(m.syncBlocksPulls),
		"uptime_seconds":     time.Since(m.startedAt).Seconds(),
	}
	m.mu.RUnlock()

	// Computed outside the lock above would race; these take their own.
	out["hash_rate"] = m.HashRate()
	out["average_block_time_seconds"] = m.AverageBlockTime().Seconds()
	return out
}

// metricHelp documents each series for the exposition format.
var metricHelp = map[string]string{
	"blocks_mined":               "Blocks mined by this node",
	"blocks_accepted":            "Blocks accepted onto the chain",
	"blocks_rejected":            "Blocks refused by validation",
	"reorgs":                     "Chain reorganisations performed",
	"reorg_blocks":               "Blocks rolled back by reorganisations",
	"tx_submitted":               "Transactions admitted to the mempool",
	"tx_rejected":                "Transactions refused admission",
	"tx_mined":                   "Transactions included in mined blocks",
	"mempool_evicted":            "Transactions evicted or trimmed from the mempool",
	"peers_connected":            "Peer connections established",
	"peer_auth_failed":           "Peer handshakes that failed authentication",
	"sync_passes":                "Synchronisation passes run",
	"sync_blocks_pulled":         "Blocks pulled from peers during synchronisation",
	"uptime_seconds":             "Seconds since the node started",
	"hash_rate":                  "Estimated hashes per second over recent blocks",
	"average_block_time_seconds": "Mean interval between recent blocks",
}

// counterMetrics names the series that only increase, so the exposition format
// can type them correctly. A collector treats a counter and a gauge differently:
// mislabelling a counter as a gauge loses rate() entirely.
var counterMetrics = map[string]bool{
	"blocks_mined": true, "blocks_accepted": true, "blocks_rejected": true,
	"reorgs": true, "reorg_blocks": true, "tx_submitted": true,
	"tx_rejected": true, "tx_mined": true, "mempool_evicted": true,
	"peers_connected": true, "peer_auth_failed": true,
	"sync_passes": true, "sync_blocks_pulled": true,
}

// Prometheus renders the snapshot in the Prometheus text exposition format.
//
// Written out rather than pulling in the client library: it is a well-specified
// line format, and the project deliberately keeps its dependency surface small.
func (m *Metrics) Prometheus(prefix string, extra map[string]float64) string {
	if prefix == "" {
		prefix = "gbb"
	}

	values := m.Snapshot()
	for name, value := range extra {
		values[name] = value
	}

	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	// Sorted so the output is stable and diffable between scrapes.
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		full := prefix + "_" + name
		if help, ok := metricHelp[name]; ok {
			fmt.Fprintf(&b, "# HELP %s %s\n", full, help)
		}
		kind := "gauge"
		if counterMetrics[name] {
			kind = "counter"
		}
		fmt.Fprintf(&b, "# TYPE %s %s\n", full, kind)
		fmt.Fprintf(&b, "%s %s\n", full, formatMetricValue(values[name]))
	}
	return b.String()
}

// formatMetricValue renders a float without an exponent for whole numbers, which
// keeps counters readable.
func formatMetricValue(v float64) string {
	if v == float64(int64(v)) {
		return fmt.Sprintf("%d", int64(v))
	}
	return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%.6f", v), "0"), ".")
}
