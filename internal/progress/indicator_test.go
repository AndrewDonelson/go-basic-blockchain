package progress

import (
	"fmt"
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
