# Progress Indicator Package

A comprehensive progress indicator package for the Go Basic Blockchain project that provides real-time visual feedback for blockchain operations.

## Features

- **Real-time Status Updates**: Shows current blockchain status including block count, transaction queue, difficulty, and uptime
- **Mining Progress**: Displays mining progress with hash attempts and difficulty levels
- **Transaction Processing**: Shows transaction status from pending to confirmed
- **Block Creation**: Progress bars for block creation with transaction processing
- **Helios Consensus**: Special indicators for the three-stage Helios consensus algorithm
- **Network Status**: Shows peer connectivity and sync status
- **Error Handling**: Color-coded error, warning, success, and info messages
- **Terminal Detection**: Automatically detects terminal capabilities and adjusts output accordingly

## Popular Dependencies Used

- **[briandowns/spinner](https://github.com/briandowns/spinner)**: Animated spinners for loading states
- **[schollz/progressbar/v3](https://github.com/schollz/progressbar)**: Progress bars with themes and customization
- **[fatih/color](https://github.com/fatih/color)**: Colored terminal output with emoji support

## Usage

### Basic Usage

```go
package main

import (
    "time"
    "github.com/AndrewDonelson/go-basic-blockchain/internal/progress"
)

func main() {
    // Create progress indicator
    pi := progress.NewProgressIndicator()
    
    // Start the indicator
    pi.Start()
    defer pi.Stop()
    
    // Update blockchain status
    status := progress.BlockchainStatus{
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
    
    // Show mining progress
    pi.ShowMiningProgress(15, 4, "0000abc123def456")
    
    // Show transaction processing
    pi.ShowTransactionProgress("tx123456", "pending")
    pi.ShowTransactionProgress("tx123456", "validating")
    pi.ShowTransactionProgress("tx123456", "confirmed")
    
    // Show Helios consensus stages
    pi.ShowHeliosProgress(0, "Proof Generation")
    pi.ShowHeliosProgress(1, "Sidechain Routing")
    pi.ShowHeliosProgress(2, "Block Finalization")
    
    // Show messages
    pi.ShowInfo("Blockchain is running")
    pi.ShowSuccess("Block mined successfully")
    pi.ShowWarning("Network connection unstable")
    pi.ShowError("Transaction failed")
}
```

### Integration with Blockchain

The progress indicator is automatically integrated into the blockchain and node components:

```go
// In blockchain.go
type Blockchain struct {
    // ... other fields
    progressIndicator *progress.ProgressIndicator
}

// In node.go
type Node struct {
    // ... other fields
    ProgressIndicator *progress.ProgressIndicator
}
```

## API Reference

### ProgressIndicator

#### Methods

- `NewProgressIndicator() *ProgressIndicator`: Creates a new progress indicator
- `Start()`: Starts the progress indicator
- `Stop()`: Stops the progress indicator
- `UpdateStatus(status BlockchainStatus)`: Updates the blockchain status
- `ShowMiningProgress(blockIndex, difficulty int, currentHash string)`: Shows mining progress
- `ShowTransactionProgress(txID, status string)`: Shows transaction processing status
- `ShowBlockProgress(blockIndex, txCount int)`: Shows block creation progress
- `ShowNetworkStatus(peers int, isSynced bool)`: Shows network connectivity status
- `ShowHeliosProgress(stage int, stageName string)`: Shows Helios consensus progress
- `ShowInfo(message string)`: Shows an info message
- `ShowSuccess(message string)`: Shows a success message
- `ShowWarning(message string)`: Shows a warning message
- `ShowError(message string)`: Shows an error message

### BlockchainStatus

```go
type BlockchainStatus struct {
    IsMining     bool
    BlockCount   int
    TxQueueSize  int
    Difficulty   int
    HashRate     float64
    LastBlock    string
    Peers        int
    IsSynced     bool
    Uptime       time.Duration
}
```

## Demo

Run the progress indicator demo:

```bash
make demo
```

This will show all the different types of progress indicators and animations.

## Testing

Run the progress indicator tests:

```bash
go test ./internal/progress -v
```

## Features

### Terminal Detection

The package automatically detects if it's running in a terminal that supports colors and animations. If not, it falls back to simple text output.

### Thread Safety

All methods are thread-safe and can be called from multiple goroutines.

### Performance

The progress indicator is designed to be lightweight and not impact blockchain performance. Status updates are buffered and non-blocking.

### Customization

The package supports customization of:
- Spinner frames and colors
- Progress bar themes
- Update frequencies
- Message styling

## Examples

### Mining Progress
```
🔨 Hashing... Block #15 (Difficulty: 4)
Current Hash: 0000abc123def456...
```

### Transaction Processing
```
📝 Transaction tx123456: Pending
🔍 Transaction tx123456: Validating
✅ Transaction tx123456: Confirmed
```

### Helios Consensus
```
☀️  Helios Stage 1/3: 🔐 Proof Generation
☀️  Helios Stage 2/3: 🔄 Sidechain Routing
☀️  Helios Stage 3/3: ✅ Block Finalization
```

### Network Status
```
🌐 Network: Connected (3 peers) - Synced
🌐 Network: Connected (2 peers) - Syncing...
```

### Status Line
```
⛏️  Mining... 📊 Status: Blocks=15 | TXs=5 | Difficulty=4 | Peers=3 | Uptime=5m 30s
```

## Contributing

When adding new progress indicators:

1. Follow the existing naming conventions
2. Add appropriate tests
3. Update this documentation
4. Ensure thread safety
5. Add terminal capability detection

## License

This package is part of the Go Basic Blockchain project and follows the same license terms. 