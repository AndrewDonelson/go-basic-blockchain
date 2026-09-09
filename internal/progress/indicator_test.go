package progress

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestProgressIndicator(t *testing.T) {
	// Create a new progress indicator
	pi := NewProgressIndicator()
	if pi == nil {
		t.Fatal("Failed to create progress indicator")
	}

	// Test basic functionality
	t.Run("Basic Functionality", func(t *testing.T) {
		// Start the indicator
		pi.Start()

		// Update status
		status := BlockchainStatus{
			IsMining:    true,
			BlockCount:  10,
			TxQueueSize: 5,
			Difficulty:  4,
			HashRate:    1000.0,
			LastBlock:   "abc123...",
			Peers:       3,
			IsSynced:    true,
			Uptime:      time.Minute * 5,
		}

		pi.UpdateStatus(status)

		// Show various progress types
		pi.ShowMiningProgress(15, 4, "0000abc123def456")
		pi.ShowTransactionProgress("tx123456", "pending")
		pi.ShowTransactionProgress("tx123456", "validating")
		pi.ShowTransactionProgress("tx123456", "confirmed")
		pi.ShowBlockProgress(15, 5)
		pi.ShowNetworkStatus(3, true)
		pi.ShowHeliosProgress(0, "Proof Generation")
		pi.ShowHeliosProgress(1, "Sidechain Routing")
		pi.ShowHeliosProgress(2, "Block Finalization")

		// Show messages
		pi.ShowInfo("This is an info message")
		pi.ShowSuccess("This is a success message")
		pi.ShowWarning("This is a warning message")
		pi.ShowError("This is an error message")

		// Stop the indicator
		pi.Stop()
	})

	t.Run("Status Updates", func(t *testing.T) {
		pi := NewProgressIndicator()
		pi.Start()

		// Test multiple status updates
		for i := 0; i < 5; i++ {
			status := BlockchainStatus{
				IsMining:    true,
				BlockCount:  i,
				TxQueueSize: i * 2,
				Difficulty:  4,
				HashRate:    float64(i * 100),
				LastBlock:   fmt.Sprintf("block%d", i),
				Peers:       i,
				IsSynced:    i%2 == 0,
				Uptime:      time.Duration(i) * time.Minute,
			}
			pi.UpdateStatus(status)
			time.Sleep(100 * time.Millisecond)
		}

		pi.Stop()
	})

	t.Run("Mining Progress", func(t *testing.T) {
		pi := NewProgressIndicator()
		pi.Start()

		// Simulate mining progress
		for i := 0; i < 3; i++ {
			pi.ShowMiningProgress(i, 4, fmt.Sprintf("hash%d", i))
			time.Sleep(200 * time.Millisecond)
		}

		pi.Stop()
	})

	t.Run("Transaction Processing", func(t *testing.T) {
		pi := NewProgressIndicator()
		pi.Start()

		// Simulate transaction processing
		txIDs := []string{"tx1", "tx2", "tx3", "tx4", "tx5"}
		for _, txID := range txIDs {
			pi.ShowTransactionProgress(txID, "pending")
			time.Sleep(100 * time.Millisecond)
			pi.ShowTransactionProgress(txID, "validating")
			time.Sleep(100 * time.Millisecond)
			pi.ShowTransactionProgress(txID, "confirmed")
			time.Sleep(100 * time.Millisecond)
		}

		pi.Stop()
	})

	t.Run("Helios Consensus", func(t *testing.T) {
		pi := NewProgressIndicator()
		pi.Start()

		// Simulate Helios consensus stages
		stages := []string{"Proof Generation", "Sidechain Routing", "Block Finalization"}
		for i, stage := range stages {
			pi.ShowHeliosProgress(i, stage)
			time.Sleep(300 * time.Millisecond)
		}

		pi.Stop()
	})
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		duration time.Duration
		expected string
	}{
		{30 * time.Second, "30s"},
		{90 * time.Second, "1m 30s"},
		{2 * time.Hour, "2h 0m"},
		{2*time.Hour + 30*time.Minute, "2h 30m"},
	}

	for _, test := range tests {
		result := formatDuration(test.duration)
		if result != test.expected {
			t.Errorf("formatDuration(%v) = %s, expected %s", test.duration, result, test.expected)
		}
	}
}

func TestIsTerminalSupported(t *testing.T) {
	// This test just ensures the function doesn't panic
	result := isTerminalSupported()
	_ = result // Use result to avoid unused variable warning
}

func TestDeriveAction(t *testing.T) {
	tests := []struct {
		name   string
		status BlockchainStatus
		want   string
	}{
		{name: "explicit action", status: BlockchainStatus{Action: "Broadcasting"}, want: "Broadcasting"},
		{name: "no active action", status: BlockchainStatus{IsMining: true}, want: "IDLE"},
		{name: "unsynced without explicit action", status: BlockchainStatus{IsSynced: false}, want: "IDLE"},
	}

	for _, tc := range tests {
		if got := deriveAction(tc.status); got != tc.want {
			t.Fatalf("%s: got %q want %q", tc.name, got, tc.want)
		}
	}
}

func TestCurrentStatusExpiresActionToIdle(t *testing.T) {
	pi := NewProgressIndicator()
	pi.mutex.Lock()
	pi.status = BlockchainStatus{Action: "Broadcasting"}
	pi.statusReady = true
	pi.actionUpdatedAt = time.Now().Add(-actionDisplayTTL - time.Millisecond)
	pi.mutex.Unlock()

	status := pi.CurrentStatus()
	if status.Action != idleAction {
		t.Fatalf("expected expired action to become %q, got %q", idleAction, status.Action)
	}
}

func TestBuildStatusLine(t *testing.T) {
	status := BlockchainStatus{
		Action:      "Linking",
		BlockCount:  12,
		TotalBlocks: 120,
		TxQueueSize: 7,
		Difficulty:  4,
		Peers:       3,
	}

	line := buildStatusLine(status, 95*time.Second, statusSpinnerFrames[0])
	for _, want := range []string{"Act:Linking", "Blk:12/120", "Tx:7", "Diff:4", "Peers:3", "Up:1m 35s"} {
		if !strings.Contains(line, want) {
			t.Fatalf("expected line to contain %q, got %q", want, line)
		}
	}
	if got := len(line); got > 90 {
		t.Fatalf("expected compact line, got length %d: %q", got, line)
	}
}

func TestBuildStatusLineUsesSpinnerFrame(t *testing.T) {
	status := BlockchainStatus{Action: "Mining"}
	line := buildStatusLine(status, time.Second, statusSpinnerFrames[3])
	if !strings.HasPrefix(line, statusSpinnerFrames[3]+" ") {
		t.Fatalf("expected line to start with spinner frame %q, got %q", statusSpinnerFrames[3], line)
	}
}

// Benchmark tests for performance
func BenchmarkProgressIndicator(b *testing.B) {
	pi := NewProgressIndicator()
	pi.Start()
	defer pi.Stop()

	status := BlockchainStatus{
		IsMining:    true,
		BlockCount:  100,
		TxQueueSize: 50,
		Difficulty:  4,
		HashRate:    1000.0,
		LastBlock:   "abc123...",
		Peers:       5,
		IsSynced:    true,
		Uptime:      time.Hour,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pi.UpdateStatus(status)
	}
}

// TestStopDoesNotDeadlockAgainstTheRenderLoop pins a lock-order inversion.
//
// renderStatus used to advance the spinner while holding outputMu, so it took
// outputMu and then mutex; Stop takes mutex and then outputMu. Stop would hold
// mutex waiting for outputMu while the render loop held outputMu waiting for
// mutex, and neither could proceed.
//
// The window is narrow -- it only appeared when the machine was loaded enough for
// a shutdown to land between the two acquisitions -- so this hammers Start/Stop
// against a running render loop to force it.
func TestStopDoesNotDeadlockAgainstTheRenderLoop(t *testing.T) {
	done := make(chan struct{})

	go func() {
		defer close(done)
		for i := 0; i < 200; i++ {
			pi := NewProgressIndicator()
			pi.Start()

			// Give the render loop something to do, so it is likely to be inside
			// renderStatus when Stop arrives.
			pi.UpdateStatus(BlockchainStatus{
				Action:     "Mining",
				BlockCount: i,
				IsMining:   true,
			})
			pi.ShowMiningProgress(i, 4, "hash")

			pi.Stop()
		}
	}()

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("Start/Stop deadlocked against the render loop; Stop holds mutex " +
			"waiting for outputMu while renderStatus holds outputMu waiting for mutex")
	}
}
