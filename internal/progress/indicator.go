package progress

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/schollz/progressbar/v3"
)

// BlockchainStatus represents the current status of the blockchain
type BlockchainStatus struct {
	IsMining    bool
	BlockCount  int
	TxQueueSize int
	Difficulty  int
	HashRate    float64
	LastBlock   string
	Peers       int
	IsSynced    bool
	Uptime      time.Duration
}

// ProgressIndicator provides visual feedback for blockchain operations
type ProgressIndicator struct {
	spinner    *spinner.Spinner
	statusBar  *progressbar.ProgressBar
	status     BlockchainStatus
	mutex      sync.RWMutex
	isRunning  bool
	startTime  time.Time
	updateChan chan BlockchainStatus
}

// NewProgressIndicator creates a new progress indicator
func NewProgressIndicator() *ProgressIndicator {
	// Check if we're in a terminal that supports colors and animations
	isTerminal := isTerminalSupported()

	pi := &ProgressIndicator{
		updateChan: make(chan BlockchainStatus, 10),
		startTime:  time.Now(),
	}

	if isTerminal {
		// Create spinner with professional, fixed-width, meaningful frames (all 18 chars)
		pi.spinner = spinner.New(
			[]string{
				"  Mining        ", // Mining a block
				"  Packing       ", // Packing transactions
				"  Linking       ", // Linking block to chain
				"  Securing      ", // Finalizing block
				"  Syncing       ", // Network sync
				"  Verifying     ", // Verifying transactions
				"  Validating    ", // Validating block
				"  Broadcasting  ", // Broadcasting block/tx
			},
			300*time.Millisecond, // Slower spinner to reduce flickering
			spinner.WithColor("cyan"),
			spinner.WithSuffix(" Blockchain Active"),
			spinner.WithFinalMSG("✅ Blockchain Ready"),
		)

		// Create progress bar for overall blockchain progress
		pi.statusBar = progressbar.NewOptions(100,
			progressbar.OptionEnableColorCodes(true),
			progressbar.OptionShowBytes(false),
			progressbar.OptionSetWidth(50),
			progressbar.OptionSetDescription("[cyan][1/1][reset] Blockchain Status"),
			progressbar.OptionSetTheme(progressbar.Theme{
				Saucer:        "[green]=[reset]",
				SaucerHead:    "[green]>[reset]",
				SaucerPadding: " ",
				BarStart:      "[",
				BarEnd:        "]",
			}),
		)
	}

	return pi
}

// Start begins the progress indicator
func (pi *ProgressIndicator) Start() {
	pi.mutex.Lock()
	defer pi.mutex.Unlock()

	if pi.isRunning {
		return
	}

	pi.isRunning = true

	if pi.spinner != nil {
		pi.spinner.Start()
	}

	// Start status update goroutine
	go pi.statusUpdateLoop()
}

// Stop stops the progress indicator
func (pi *ProgressIndicator) Stop() {
	pi.mutex.Lock()
	defer pi.mutex.Unlock()

	if !pi.isRunning {
		return
	}

	pi.isRunning = false

	if pi.spinner != nil {
		pi.spinner.Stop()
	}

	// Clear the line
	fmt.Print("\r\033[K")
}

// UpdateStatus updates the blockchain status
func (pi *ProgressIndicator) UpdateStatus(status BlockchainStatus) {
	pi.mutex.Lock()
	pi.status = status
	pi.mutex.Unlock()

	// Send update to channel (non-blocking)
	select {
	case pi.updateChan <- status:
	default:
		// Channel is full, skip this update
	}
}

// ShowMiningProgress shows mining progress with current hash attempts
func (pi *ProgressIndicator) ShowMiningProgress(blockIndex int, difficulty int, currentHash string) {
	pi.mutex.RLock()
	defer pi.mutex.RUnlock()

	if !pi.isRunning {
		return
	}

	// Create mining-specific spinner with professional formatting
	miningSpinner := spinner.New(
		[]string{
			"  Hashing...    ",
			"  Computing...  ",
			"  Targeting...  ",
			"  Finding...    ",
			"  Searching...  ",
			"  Processing... ",
		},
		100*time.Millisecond,
		spinner.WithColor("yellow"),
		spinner.WithSuffix(fmt.Sprintf(" Block #%-4d (Difficulty: %-2d)", blockIndex, difficulty)),
		spinner.WithFinalMSG(fmt.Sprintf("✅ Block #%d mined successfully", blockIndex)),
	)

	miningSpinner.Start()
	defer miningSpinner.Stop()

	// Show hash attempts with consistent formatting
	hashDisplay := currentHash
	if len(currentHash) > 16 {
		hashDisplay = currentHash[:16]
	}
	color.Yellow("Current Hash: %-16s...", hashDisplay)
}

// ShowTransactionProgress shows transaction processing progress
func (pi *ProgressIndicator) ShowTransactionProgress(txID string, status string) {
	pi.mutex.RLock()
	defer pi.mutex.RUnlock()

	if !pi.isRunning {
		return
	}

	// Handle short transaction IDs
	txDisplay := txID
	if len(txID) > 8 {
		txDisplay = txID[:8]
	}

	switch status {
	case "pending":
		color.Blue("📝 Transaction %-8s: Pending", txDisplay)
	case "validating":
		color.Yellow("🔍 Transaction %-8s: Validating", txDisplay)
	case "confirmed":
		color.Green("✅ Transaction %-8s: Confirmed", txDisplay)
	case "failed":
		color.Red("❌ Transaction %-8s: Failed", txDisplay)
	}
}

// ShowBlockProgress shows block creation progress
func (pi *ProgressIndicator) ShowBlockProgress(blockIndex int, txCount int) {
	pi.mutex.RLock()
	defer pi.mutex.RUnlock()

	if !pi.isRunning {
		return
	}

	// Create block progress bar
	blockBar := progressbar.NewOptions(txCount,
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionSetDescription(fmt.Sprintf("[cyan]Block #%d[reset]", blockIndex)),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)

	// Simulate progress for each transaction
	for i := 0; i < txCount; i++ {
		time.Sleep(50 * time.Millisecond)
		if err := blockBar.Add(1); err != nil {
			// Log error but continue
			_ = err // Suppress unused variable warning
		}
	}

	// Finish the progress bar
	if err := blockBar.Finish(); err != nil {
		// Log error but continue
		_ = err // Suppress unused variable warning
	}
}

// ShowNetworkStatus shows network connectivity status
func (pi *ProgressIndicator) ShowNetworkStatus(peers int, isSynced bool) {
	pi.mutex.RLock()
	defer pi.mutex.RUnlock()

	if !pi.isRunning {
		return
	}

	if isSynced {
		color.Green("🌐 Network: Connected (%-2d peers) - Synced", peers)
	} else {
		color.Yellow("🌐 Network: Connected (%-2d peers) - Syncing...", peers)
	}
}

// ShowHeliosProgress shows Helios consensus algorithm progress
func (pi *ProgressIndicator) ShowHeliosProgress(stage int, stageName string) {
	pi.mutex.RLock()
	defer pi.mutex.RUnlock()

	if !pi.isRunning {
		return
	}

	stages := []string{
		"🔐 Proof Generation   ",
		"🔄 Sidechain Routing  ",
		"✅ Block Finalization ",
	}

	if stage >= 0 && stage < len(stages) {
		color.Cyan("☀️  Helios Stage %d/%-2d: %-20s", stage+1, len(stages), stages[stage])
	}
}

// statusUpdateLoop handles periodic status updates
func (pi *ProgressIndicator) statusUpdateLoop() {
	ticker := time.NewTicker(5 * time.Second) // Reduced frequency to reduce flickering
	defer ticker.Stop()

	for {
		pi.mutex.RLock()
		isRunning := pi.isRunning
		pi.mutex.RUnlock()

		if !isRunning {
			break
		}

		select {
		case status := <-pi.updateChan:
			pi.displayStatus(status)
		case <-ticker.C:
			pi.mutex.RLock()
			status := pi.status
			pi.mutex.RUnlock()
			pi.displayStatus(status)
		}
	}
}

// displayStatus displays the current blockchain status
func (pi *ProgressIndicator) displayStatus(status BlockchainStatus) {
	pi.mutex.RLock()
	isRunning := pi.isRunning
	pi.mutex.RUnlock()

	if !isRunning {
		return
	}

	// Calculate uptime
	uptime := time.Since(pi.startTime)

	// Create a professional status line with fixed-width formatting
	statusLine := fmt.Sprintf(
		"📊 Status: Blocks=%-4d | TXs=%-3d | Difficulty=%-2d | Peers=%-2d | Uptime=%-8s",
		status.BlockCount,
		status.TxQueueSize,
		status.Difficulty,
		status.Peers,
		formatDuration(uptime),
	)

	// Add padding to ensure consistent width
	paddedStatusLine := fmt.Sprintf("%-80s", statusLine)

	// Update spinner suffix with status (only if it changed to reduce flickering)
	if pi.spinner != nil {
		currentSuffix := pi.spinner.Suffix
		newSuffix := " " + paddedStatusLine
		if currentSuffix != newSuffix {
			pi.spinner.Suffix = newSuffix
		}
	}

	// Update progress bar if mining
	if status.IsMining && pi.statusBar != nil {
		// Note: progressbar doesn't have a Set method, so we'll just update the description
		// The progress bar will be updated through Add() calls in other methods
		_ = status.BlockCount // Use block count for future implementation
	}
}

// formatDuration formats duration in a human-readable way
func formatDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm %ds", int(d.Minutes()), int(d.Seconds())%60)
	} else {
		return fmt.Sprintf("%dh %dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

// isTerminalSupported checks if the current terminal supports colors and animations
func isTerminalSupported() bool {
	// Check if we're in a terminal
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	// Check if it's a character device (terminal)
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}

// ShowError displays an error message with appropriate styling
func (pi *ProgressIndicator) ShowError(message string) {
	color.Red("❌ Error:   %s", message)
}

// ShowSuccess displays a success message with appropriate styling
func (pi *ProgressIndicator) ShowSuccess(message string) {
	color.Green("✅ Success: %s", message)
}

// ShowWarning displays a warning message with appropriate styling
func (pi *ProgressIndicator) ShowWarning(message string) {
	color.Yellow("⚠️  Warning: %s", message)
}

// ShowInfo displays an info message with appropriate styling
func (pi *ProgressIndicator) ShowInfo(message string) {
	color.Blue("ℹ️  Info:    %s", message)
}
