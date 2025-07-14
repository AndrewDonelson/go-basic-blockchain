package main

import (
	"fmt"
	"log"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/progress"
)

func main() {
	fmt.Println("🚀 Go Basic Blockchain - Progress Indicator Demo")
	fmt.Println("================================================")

	// Create progress indicator
	pi := progress.NewProgressIndicator()
	if pi == nil {
		log.Fatal("Failed to create progress indicator")
	}

	// Start the indicator
	pi.Start()
	defer pi.Stop()

	// Demo blockchain startup
	fmt.Println("\n📋 Starting blockchain demo...")
	pi.ShowInfo("Initializing blockchain components...")
	time.Sleep(1 * time.Second)

	pi.ShowSuccess("Blockchain initialized successfully")
	time.Sleep(500 * time.Millisecond)

	pi.ShowInfo("Starting mining process...")
	time.Sleep(500 * time.Millisecond)

	// Demo mining progress
	fmt.Println("\n⛏️  Mining Demo:")
	for i := 0; i < 3; i++ {
		pi.ShowMiningProgress(i+1, 4, fmt.Sprintf("0000abc123def456%d", i))
		time.Sleep(800 * time.Millisecond)
	}

	// Demo transaction processing
	fmt.Println("\n📝 Transaction Processing Demo:")
	txIDs := []string{"tx12345678", "tx87654321", "txabcdef12", "tx12abcdef"}
	for _, txID := range txIDs {
		pi.ShowTransactionProgress(txID, "pending")
		time.Sleep(300 * time.Millisecond)
		pi.ShowTransactionProgress(txID, "validating")
		time.Sleep(300 * time.Millisecond)
		pi.ShowTransactionProgress(txID, "confirmed")
		time.Sleep(300 * time.Millisecond)
	}

	// Demo block creation
	fmt.Println("\n📦 Block Creation Demo:")
	pi.ShowBlockProgress(15, 5)
	time.Sleep(1 * time.Second)

	// Demo Helios consensus
	fmt.Println("\n☀️  Helios Consensus Demo:")
	stages := []string{"Proof Generation", "Sidechain Routing", "Block Finalization"}
	for i, stage := range stages {
		pi.ShowHeliosProgress(i, stage)
		time.Sleep(600 * time.Millisecond)
	}

	// Demo network status
	fmt.Println("\n🌐 Network Status Demo:")
	for i := 0; i < 3; i++ {
		pi.ShowNetworkStatus(i+1, i%2 == 0)
		time.Sleep(500 * time.Millisecond)
	}

	// Demo status updates
	fmt.Println("\n📊 Status Updates Demo:")
	for i := 0; i < 5; i++ {
		status := progress.BlockchainStatus{
			IsMining:    true,
			BlockCount:  i + 1,
			TxQueueSize: (i + 1) * 2,
			Difficulty:  4,
			HashRate:    float64((i + 1) * 100),
			LastBlock:   fmt.Sprintf("block%d", i+1),
			Peers:       i + 1,
			IsSynced:    i%2 == 0,
			Uptime:      time.Duration(i+1) * time.Minute,
		}
		pi.UpdateStatus(status)
		time.Sleep(1 * time.Second)
	}

	// Demo error handling
	fmt.Println("\n⚠️  Error Handling Demo:")
	pi.ShowWarning("Network connection unstable")
	time.Sleep(500 * time.Millisecond)
	pi.ShowError("Transaction validation failed")
	time.Sleep(500 * time.Millisecond)
	pi.ShowSuccess("Recovery successful")

	fmt.Println("\n✅ Demo completed successfully!")
	fmt.Println("The progress indicator shows real-time blockchain activity including:")
	fmt.Println("  • Mining progress with hash attempts")
	fmt.Println("  • Transaction processing status")
	fmt.Println("  • Block creation progress")
	fmt.Println("  • Helios consensus algorithm stages")
	fmt.Println("  • Network connectivity status")
	fmt.Println("  • Overall blockchain statistics")
}
