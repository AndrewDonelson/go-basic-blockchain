package progress

import (
	"fmt"
	"io"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	progressbar "github.com/schollz/progressbar/v3"
)

// InstallLogWriter routes the standard logger through the progress indicator so
// log output does not corrupt the status line.
//
// This is opt-in and idempotent. It must be called explicitly by the application
// (cmd/chaind does), never implicitly by a constructor.
func InstallLogWriter() {
	installLogWriterOnce.Do(func() {
		log.SetOutput(&statusAwareWriter{writer: os.Stderr})
	})
}

// BlockchainStatus represents the current status of the blockchain
type BlockchainStatus struct {
	Action      string
	IsMining    bool
	BlockCount  int
	TotalBlocks int
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
	spinner         *spinner.Spinner
	statusBar       *progressbar.ProgressBar
	status          BlockchainStatus
	statusReady     bool
	actionUpdatedAt time.Time
	frameIndex      int
	lastLine        string
	lastWidth       int
	mutex           sync.RWMutex
	outputMu        sync.Mutex
	isRunning       bool
	isPaused        bool
	isTerminal      bool
	doneChan        chan struct{}
	updateChan      chan BlockchainStatus
}

var (
	activeIndicatorMu    sync.RWMutex
	activeIndicator      *ProgressIndicator
	installLogWriterOnce sync.Once
)

type statusAwareWriter struct {
	writer io.Writer
}

func (w *statusAwareWriter) Write(p []byte) (int, error) {
	pi := currentActiveIndicator()
	if pi != nil {
		pi.prepareForExternalOutput()
	}

	n, err := w.writer.Write(p)

	if pi != nil {
		pi.renderStatus(false)
	}

	return n, err
}

// NewProgressIndicator creates a new progress indicator
// NewProgressIndicator creates a new progress indicator.
//
// It no longer calls log.SetOutput. Redirecting the standard logger as a side
// effect of constructing an object silently took over logging for the whole
// process, including any host application that merely imports this package.
// Callers that want the status-aware writer opt in with InstallLogWriter.
func NewProgressIndicator() *ProgressIndicator {
	// Check if we're in a terminal that supports colors and animations
	isTerminal := isTerminalSupported()

	pi := &ProgressIndicator{
		updateChan: make(chan BlockchainStatus, 10),
		doneChan:   make(chan struct{}),
		isTerminal: isTerminal,
	}

	if isTerminal {
		// Keep a standalone spinner for mining helper output only.
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
			300*time.Millisecond,
			spinner.WithColor("cyan"),
			spinner.WithSuffix(""),
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
	pi.isPaused = false
	pi.statusReady = false
	pi.actionUpdatedAt = time.Time{}
	pi.frameIndex = 0
	pi.lastLine = ""
	pi.lastWidth = 0
	pi.doneChan = make(chan struct{})
	setActiveIndicator(pi)

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
	pi.isPaused = true
	select {
	case <-pi.doneChan:
	default:
		close(pi.doneChan)
	}

	pi.outputMu.Lock()
	defer pi.outputMu.Unlock()
	// Clear the active status line once on stop.
	fmt.Print("\r\033[K")
	pi.lastLine = ""
	if currentActiveIndicator() == pi {
		setActiveIndicator(nil)
	}
}

// Pause pauses the progress indicator
func (pi *ProgressIndicator) Pause() {
	pi.mutex.Lock()
	defer pi.mutex.Unlock()

	if !pi.isRunning {
		return
	}

	pi.isPaused = true

	pi.outputMu.Lock()
	defer pi.outputMu.Unlock()
	fmt.Print("\r\033[K")
	pi.lastLine = ""
	pi.lastWidth = 0
}

// Resume resumes the progress indicator
func (pi *ProgressIndicator) Resume() {
	pi.mutex.Lock()
	defer pi.mutex.Unlock()

	if !pi.isRunning {
		return
	}

	pi.isPaused = false
	if pi.doneChan == nil {
		pi.doneChan = make(chan struct{})
	}
	setActiveIndicator(pi)
}

// IsPaused returns true if the progress indicator is currently paused
func (pi *ProgressIndicator) IsPaused() bool {
	pi.mutex.RLock()
	defer pi.mutex.RUnlock()
	return pi.isPaused
}

// UpdateStatus updates the blockchain status
func (pi *ProgressIndicator) UpdateStatus(status BlockchainStatus) {
	pi.mutex.Lock()
	pi.status = status
	pi.statusReady = true
	pi.mutex.Unlock()

	// Send update to channel (non-blocking).
	select {
	case pi.updateChan <- status:
	default:
		// Channel is full, skip this update.
	}
}

// CurrentStatus returns the last cached blockchain status.
func (pi *ProgressIndicator) CurrentStatus() BlockchainStatus {
	pi.mutex.RLock()
	defer pi.mutex.RUnlock()
	return pi.normalizedStatusLocked(time.Now())
}

// UpdateAction updates only the action field of the cached status.
func (pi *ProgressIndicator) UpdateAction(action string) {
	pi.mutex.Lock()
	status := pi.normalizedStatusLocked(time.Now())
	status.Action = action
	pi.status = status
	pi.statusReady = true
	if action == "" || action == idleAction {
		pi.actionUpdatedAt = time.Time{}
	} else {
		pi.actionUpdatedAt = time.Now()
	}
	pi.mutex.Unlock()

	select {
	case pi.updateChan <- status:
	default:
	}
}

// ShowMiningProgress shows mining progress with current hash attempts
func (pi *ProgressIndicator) ShowMiningProgress(blockIndex int, difficulty int, currentHash string) {
	pi.mutex.RLock()
	isRunning := pi.isRunning
	pi.mutex.RUnlock()

	if !isRunning {
		return
	}

	pi.UpdateAction("Mining")

	// Show hash attempts with consistent formatting
	hashDisplay := currentHash
	if len(currentHash) > 16 {
		hashDisplay = currentHash[:16]
	}
	pi.printExternalMessage(func() {
		color.Yellow("Current Hash: %-16s...", hashDisplay)
	})
}

// ShowTransactionProgress shows transaction processing progress
func (pi *ProgressIndicator) ShowTransactionProgress(txID string, status string) {
	pi.mutex.RLock()
	isRunning := pi.isRunning
	pi.mutex.RUnlock()

	if !isRunning {
		return
	}

	// Handle short transaction IDs
	txDisplay := txID
	if len(txID) > 8 {
		txDisplay = txID[:8]
	}

	switch status {
	case "pending":
		pi.UpdateAction("Packing")
		pi.printExternalMessage(func() {
			color.Blue("📝 Transaction %-8s: Pending", txDisplay)
		})
	case "validating":
		pi.UpdateAction("Validating")
		pi.printExternalMessage(func() {
			color.Yellow("🔍 Transaction %-8s: Validating", txDisplay)
		})
	case "confirmed":
		pi.UpdateAction("Securing")
		pi.printExternalMessage(func() {
			color.Green("✅ Transaction %-8s: Confirmed", txDisplay)
		})
	case "failed":
		pi.printExternalMessage(func() {
			color.Red("❌ Transaction %-8s: Failed", txDisplay)
		})
	}
}

// ShowBlockProgress shows block creation progress
func (pi *ProgressIndicator) ShowBlockProgress(blockIndex int, txCount int) {
	pi.mutex.RLock()
	isRunning := pi.isRunning
	pi.mutex.RUnlock()

	if !isRunning {
		return
	}

	pi.UpdateAction("Packing")
	pi.prepareForExternalOutput()
	defer pi.renderStatus(false)

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
	isRunning := pi.isRunning
	pi.mutex.RUnlock()

	if !isRunning {
		return
	}

	if isSynced {
		pi.UpdateAction("Securing")
		pi.printExternalMessage(func() {
			color.Green("🌐 Network: Connected (%-2d peers) - Synced", peers)
		})
	} else {
		pi.UpdateAction("Syncing")
		pi.printExternalMessage(func() {
			color.Yellow("🌐 Network: Connected (%-2d peers) - Syncing...", peers)
		})
	}
}

// ShowHeliosProgress shows Helios consensus algorithm progress
func (pi *ProgressIndicator) ShowHeliosProgress(stage int, stageName string) {
	pi.mutex.RLock()
	isRunning := pi.isRunning
	pi.mutex.RUnlock()

	if !isRunning {
		return
	}

	stages := []string{
		"🔐 Proof Generation   ",
		"🔄 Sidechain Routing  ",
		"✅ Block Finalization ",
	}

	if stage >= 0 && stage < len(stages) {
		mappedActions := []string{"Mining", "Linking", "Securing"}
		pi.UpdateAction(mappedActions[stage])
		pi.printExternalMessage(func() {
			color.Cyan("☀️  Helios Stage %d/%-2d: %-20s", stage+1, len(stages), stages[stage])
		})
	}
}

const statusRefreshInterval = 150 * time.Millisecond
const actionDisplayTTL = 1500 * time.Millisecond
const idleAction = "IDLE"

// statusUpdateLoop handles periodic status updates and spinner animation.
func (pi *ProgressIndicator) statusUpdateLoop() {
	ticker := time.NewTicker(statusRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case status := <-pi.updateChan:
			pi.mutex.Lock()
			pi.status = status
			pi.statusReady = true
			pi.mutex.Unlock()
			pi.renderStatus(false)
		case <-ticker.C:
			pi.renderStatus(true)
		case <-pi.doneChan:
			return
		}
	}
}

// renderStatus renders the current blockchain status without clearing the line.
func (pi *ProgressIndicator) renderStatus(advanceSpinner bool) {
	// The spinner is advanced HERE, in the same critical section that reads it,
	// rather than after the output lock has been taken.
	//
	// It used to be advanced at the end of this function, while outputMu was
	// held -- so renderStatus acquired outputMu and then mutex, while Stop
	// acquires mutex and then outputMu. That is a lock-order inversion and it
	// deadlocks: Stop holds mutex waiting for outputMu, the render loop holds
	// outputMu waiting for mutex, and neither ever proceeds.
	//
	// The window is narrow, which is why it only showed up under load -- a
	// shutdown landing between the two acquisitions. It would have hung node
	// shutdown in production for the same reason.
	pi.mutex.Lock()
	isRunning := pi.isRunning
	isPaused := pi.isPaused
	status := pi.normalizedStatusLocked(time.Now())
	statusReady := pi.statusReady
	frameIndex := pi.frameIndex
	if advanceSpinner {
		pi.frameIndex = (frameIndex + 1) % len(statusSpinnerFrames)
	}
	pi.mutex.Unlock()

	if !isRunning || isPaused || !statusReady {
		return
	}

	line := buildStatusLine(status, status.Uptime, statusSpinnerFrames[frameIndex])

	pi.outputMu.Lock()
	defer pi.outputMu.Unlock()

	padWidth := 0
	if pi.lastWidth > len(line) {
		padWidth = pi.lastWidth - len(line)
	}

	if line == pi.lastLine && !advanceSpinner {
		return
	}

	fmt.Printf("\r%s%s", line, strings.Repeat(" ", padWidth))
	pi.lastLine = line
	pi.lastWidth = len(line)
}

func (pi *ProgressIndicator) printExternalMessage(printFn func()) {
	pi.prepareForExternalOutput()
	printFn()
	pi.renderStatus(false)
}

func (pi *ProgressIndicator) prepareForExternalOutput() {
	pi.outputMu.Lock()
	defer pi.outputMu.Unlock()
	if pi.lastLine == "" {
		return
	}
	fmt.Fprint(os.Stdout, "\r\033[K")
	pi.lastLine = ""
	pi.lastWidth = 0
}

func setActiveIndicator(pi *ProgressIndicator) {
	activeIndicatorMu.Lock()
	defer activeIndicatorMu.Unlock()
	activeIndicator = pi
}

func currentActiveIndicator() *ProgressIndicator {
	activeIndicatorMu.RLock()
	defer activeIndicatorMu.RUnlock()
	return activeIndicator
}

func (pi *ProgressIndicator) normalizedStatusLocked(now time.Time) BlockchainStatus {
	status := pi.status
	if status.Action == "" {
		status.Action = idleAction
		return status
	}
	if status.Action == idleAction {
		return status
	}
	if pi.actionUpdatedAt.IsZero() || now.Sub(pi.actionUpdatedAt) > actionDisplayTTL {
		status.Action = idleAction
	}
	return status
}

func deriveAction(status BlockchainStatus) string {
	if status.Action != "" {
		return status.Action
	}
	return idleAction
}

var statusSpinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

func buildStatusLine(status BlockchainStatus, uptime time.Duration, spin string) string {
	if spin == "" {
		spin = statusSpinnerFrames[0]
	}

	action := status.Action
	if action == "" {
		action = deriveAction(status)
	}

	totalBlocks := status.TotalBlocks
	if totalBlocks < status.BlockCount {
		totalBlocks = status.BlockCount
	}

	return fmt.Sprintf(
		"%s Act:%s | Blk:%d/%d | Tx:%d | Diff:%d | Peers:%d | Up:%s",
		spin,
		action,
		status.BlockCount,
		totalBlocks,
		status.TxQueueSize,
		status.Difficulty,
		status.Peers,
		formatDuration(uptime),
	)
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
	pi.printExternalMessage(func() {
		color.Red("❌ Error:   %s", message)
	})
}

// ShowSuccess displays a success message with appropriate styling
func (pi *ProgressIndicator) ShowSuccess(message string) {
	pi.printExternalMessage(func() {
		color.Green("✅ Success: %s", message)
	})
}

// ShowWarning displays a warning message with appropriate styling
func (pi *ProgressIndicator) ShowWarning(message string) {
	pi.printExternalMessage(func() {
		color.Yellow("⚠️  Warning: %s", message)
	})
}

// ShowInfo displays an info message with appropriate styling
func (pi *ProgressIndicator) ShowInfo(message string) {
	pi.printExternalMessage(func() {
		color.Blue("ℹ️  Info:    %s", message)
	})
}
