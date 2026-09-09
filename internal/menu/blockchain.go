package menu

import (
	"fmt"
	"os"
	"strconv"

	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
	"golang.org/x/term"
)

// BlockchainMenu creates the main blockchain menu system
func CreateBlockchainMenu(blockchain *sdk.Blockchain) *MenuSystem {
	menuSystem := NewMenuSystem()

	// Set the progress indicator if available
	if blockchain != nil && blockchain.GetProgressIndicator() != nil {
		menuSystem.ProgressIndicator = blockchain.GetProgressIndicator()
	}

	// Set the blockchain for menu state management
	if blockchain != nil {
		menuSystem.Blockchain = blockchain
	}

	// Create root menu
	rootMenu := &Menu{
		Title: "Go Basic Blockchain - Main Menu",
	}

	// Blockchain Management
	blockchainMenu := rootMenu.AddSubMenu("blockchain", "Blockchain Management", "Manage blockchain operations")
	blockchainMenu.AddMenuItem("status", "View Blockchain Status", "Display current blockchain status and statistics", func() error {
		return showBlockchainStatus(blockchain)
	})
	blockchainMenu.AddMenuItem("blocks", "View All Blocks", "Display all blocks in the blockchain", func() error {
		return showAllBlocks(blockchain)
	})
	blockchainMenu.AddMenuItem("latest", "View Latest Block", "Display the most recent block", func() error {
		return showLatestBlock(blockchain)
	})
	blockchainMenu.AddMenuItem("validate", "Validate Blockchain", "Validate the entire blockchain", func() error {
		return validateBlockchain(blockchain)
	})

	// Wallet Management
	walletMenu := rootMenu.AddSubMenu("wallets", "Wallet Management", "Create and manage wallets")
	walletMenu.AddMenuItem("create", "Create New Wallet", "Create a new wallet with encryption", func() error {
		return createNewWallet(blockchain)
	})
	walletMenu.AddMenuItem("list", "List Wallets", "Display all available wallets", func() error {
		return listWallets(blockchain)
	})
	walletMenu.AddMenuItem("balance", "Check Balance", "Check wallet balance", func() error {
		return checkWalletBalance(blockchain)
	})
	walletMenu.AddMenuItem("unlock", "Unlock Wallet", "Unlock a wallet for transactions", func() error {
		return unlockWallet(blockchain)
	})

	// Transaction Management
	txMenu := rootMenu.AddSubMenu("transactions", "Transaction Management", "Create and manage transactions")
	txMenu.AddMenuItem("create", "Create Transaction", "Create a new transaction", func() error {
		return createTransaction(blockchain)
	})
	txMenu.AddMenuItem("pending", "View Pending Transactions", "Display pending transactions", func() error {
		return showPendingTransactions(blockchain)
	})
	txMenu.AddMenuItem("history", "Transaction History", "View transaction history", func() error {
		return showTransactionHistory(blockchain)
	})

	// Mining Operations
	miningMenu := rootMenu.AddSubMenu("mining", "Mining Operations", "Manage mining operations")
	miningMenu.AddMenuItem("status", "Mining Status", "Show current mining status", func() error {
		return showMiningStatus(blockchain)
	})
	miningMenu.AddMenuItem("difficulty", "Adjust Difficulty", "Adjust mining difficulty", func() error {
		return adjustMiningDifficulty(blockchain)
	})

	// Network Operations
	networkMenu := rootMenu.AddSubMenu("network", "Network Operations", "Manage network connections")
	networkMenu.AddMenuItem("peers", "View Peers", "Display connected peers", func() error {
		return showPeers(blockchain)
	})
	networkMenu.AddMenuItem("sync", "Sync Status", "Show synchronization status", func() error {
		return showSyncStatus(blockchain)
	})

	// Configuration
	configMenu := rootMenu.AddSubMenu("config", "Configuration", "Manage blockchain configuration")
	configMenu.AddMenuItem("view", "View Configuration", "Display current configuration", func() error {
		return showConfiguration(blockchain)
	})
	configMenu.AddMenuItem("update", "Update Configuration", "Update blockchain configuration", func() error {
		return updateConfiguration(blockchain)
	})

	// Exit option
	rootMenu.AddMenuItem("exit", "Exit Menu", "Exit the menu system", func() error {
		return nil
	})

	menuSystem.RootMenu = rootMenu
	menuSystem.CurrentMenu = rootMenu

	return menuSystem
}

// Menu action implementations
func showBlockchainStatus(blockchain *sdk.Blockchain) error {
	info := blockchain.GetBlockchainInfo()

	fmt.Printf("\n=== Blockchain Status ===\n")
	fmt.Printf("Name: %s\n", info.Name)
	fmt.Printf("Symbol: %s\n", info.Symbol)
	fmt.Printf("Version: %s\n", info.Version)
	fmt.Printf("Current Difficulty: %d\n", info.Difficulty)
	fmt.Printf("Block Time: %d seconds\n", info.BlockTime)
	fmt.Printf("Transaction Fee: %.2f\n", info.Fee)
	fmt.Printf("Total Blocks: %d\n", len(blockchain.Blocks))
	fmt.Printf("Pending Transactions: %d\n", len(blockchain.GetPendingTransactions()))

	latestBlock := blockchain.GetLatestBlock()
	if latestBlock != nil {
		fmt.Printf("Last Block Hash: %s\n", latestBlock.Hash)
	}

	return nil
}

func showAllBlocks(blockchain *sdk.Blockchain) error {
	blocks := blockchain.Blocks

	fmt.Printf("\n=== All Blocks (%d total) ===\n", len(blocks))

	for i, block := range blocks {
		fmt.Printf("\nBlock #%d:\n", i)
		fmt.Printf("  Hash: %s\n", block.Hash)
		fmt.Printf("  Previous Hash: %s\n", block.Header.PreviousHash)
		fmt.Printf("  Timestamp: %s\n", block.Header.Timestamp.Format("2006-01-02 15:04:05"))
		fmt.Printf("  Difficulty: %d\n", block.Header.Difficulty)
		fmt.Printf("  Nonce: %d\n", block.Header.Nonce)
		fmt.Printf("  Transactions: %d\n", len(block.Transactions))
	}

	return nil
}

func showLatestBlock(blockchain *sdk.Blockchain) error {
	latestBlock := blockchain.GetLatestBlock()
	if latestBlock == nil {
		fmt.Printf("\nNo blocks found in blockchain.\n")
		return nil
	}

	fmt.Printf("\n=== Latest Block ===\n")
	fmt.Printf("Index: %s\n", latestBlock.Index.String())
	fmt.Printf("Hash: %s\n", latestBlock.Hash)
	fmt.Printf("Previous Hash: %s\n", latestBlock.Header.PreviousHash)
	fmt.Printf("Timestamp: %s\n", latestBlock.Header.Timestamp.Format("2006-01-02 15:04:05"))
	fmt.Printf("Difficulty: %d\n", latestBlock.Header.Difficulty)
	fmt.Printf("Nonce: %d\n", latestBlock.Header.Nonce)
	fmt.Printf("Transactions: %d\n", len(latestBlock.Transactions))

	return nil
}

func validateBlockchain(blockchain *sdk.Blockchain) error {
	fmt.Printf("\nValidating blockchain...\n")

	err := blockchain.ValidateChain()
	if err != nil {
		fmt.Printf("❌ Blockchain validation failed: %v\n", err)
		return err
	}

	fmt.Printf("✅ Blockchain validation successful!\n")
	return nil
}

func createNewWallet(blockchain *sdk.Blockchain) error {
	fmt.Printf("\n=== Create New Wallet ===\n")

	// Get user input for wallet details
	fmt.Printf("Enter wallet name: ")
	var name string
	if _, err := fmt.Scanln(&name); err != nil {
		return err
	}

	fmt.Printf("Enter passphrase: ")
	var passphrase string
	if _, err := fmt.Scanln(&passphrase); err != nil {
		return err
	}

	// Create wallet options
	walletOpts := sdk.NewWalletOptions(
		sdk.NewBigInt(1), // organizationID
		sdk.NewBigInt(1), // appID
		sdk.NewBigInt(1), // userID
		sdk.NewBigInt(1), // assetID
		name,
		passphrase,
		[]string{"menu-created"},
	)

	wallet, err := sdk.NewWallet(walletOpts)
	if err != nil {
		fmt.Printf("❌ Failed to create wallet: %v\n", err)
		return err
	}

	fmt.Printf("✅ Wallet created successfully!\n")
	fmt.Printf("Address: %s\n", wallet.Address)

	return nil
}

func listWallets(blockchain *sdk.Blockchain) error {
	fmt.Printf("\n=== Available Wallets ===\n")

	addresses, err := sdk.ListWalletAddresses()
	if err != nil {
		fmt.Printf("❌ Failed to list wallets: %v\n", err)
		return err
	}

	if len(addresses) == 0 {
		fmt.Printf("No wallets found.\n")
		return nil
	}

	for i, address := range addresses {
		balance := 0.0
		if blockchain != nil {
			balance = blockchain.GetBalance(address)
		}
		fmt.Printf("%3d. %s  (balance: %.2f)\n", i+1, address, balance)
	}

	return nil
}

func checkWalletBalance(blockchain *sdk.Blockchain) error {
	fmt.Printf("\n=== Check Wallet Balance ===\n")

	fmt.Printf("Enter wallet address: ")
	var address string
	if _, err := fmt.Scanln(&address); err != nil {
		return err
	}

	balance := blockchain.GetBalance(address)
	fmt.Printf("Balance: %.2f\n", balance)

	return nil
}

func unlockWallet(blockchain *sdk.Blockchain) error {
	fmt.Printf("\n=== Unlock Wallet ===\n")

	fmt.Printf("Enter wallet address: ")
	var address string
	if _, err := fmt.Scanln(&address); err != nil {
		return err
	}

	passphrase, err := readPassphrase("Enter passphrase: ")
	if err != nil {
		return err
	}

	wallet, err := sdk.OpenWallet(address, passphrase)
	if err != nil {
		fmt.Printf("❌ Failed to unlock wallet: %v\n", err)
		return err
	}

	fmt.Printf("✅ Wallet %s unlocked (%s)\n", wallet.GetAddress(), wallet.GetWalletName())
	return nil
}

// readPassphrase reads a passphrase without echoing it to the terminal.
//
// The menu used fmt.Scanln, which echoes: the passphrase appeared on screen and
// in the user's scrollback.
func readPassphrase(prompt string) (string, error) {
	fmt.Print(prompt)

	fd := int(os.Stdin.Fd())
	if !term.IsTerminal(fd) {
		// Not a terminal (piped input, CI): fall back to a plain read.
		var value string
		_, err := fmt.Scanln(&value)
		return value, err
	}

	data, err := term.ReadPassword(fd)
	fmt.Println()
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func createTransaction(blockchain *sdk.Blockchain) error {
	fmt.Printf("\n=== Create Transaction ===\n")

	fmt.Printf("Enter sender address: ")
	var from string
	if _, err := fmt.Scanln(&from); err != nil {
		return err
	}

	fmt.Printf("Enter recipient address: ")
	var to string
	if _, err := fmt.Scanln(&to); err != nil {
		return err
	}

	fmt.Printf("Enter amount: ")
	var amountStr string
	if _, err := fmt.Scanln(&amountStr); err != nil {
		return err
	}

	amount, err := strconv.ParseFloat(amountStr, 64)
	if err != nil {
		fmt.Printf("❌ Invalid amount: %v\n", err)
		return err
	}

	// Create a simple bank transaction
	// This is a simplified version - in reality, you'd need proper wallet objects
	fmt.Printf("Transaction creation functionality needs to be implemented.\n")
	fmt.Printf("Would create transaction: %s -> %s (%.2f)\n", from, to, amount)

	return nil
}

func showPendingTransactions(blockchain *sdk.Blockchain) error {
	pendingTxs := blockchain.GetPendingTransactions()

	fmt.Printf("\n=== Pending Transactions (%d total) ===\n", len(pendingTxs))

	for i, tx := range pendingTxs {
		fmt.Printf("\nTransaction #%d:\n", i+1)
		fmt.Printf("  ID: %s\n", tx.GetID())
		fmt.Printf("  Protocol: %s\n", tx.GetProtocol())
		fmt.Printf("  Status: %s\n", tx.GetStatus())
	}

	return nil
}

func showTransactionHistory(blockchain *sdk.Blockchain) error {
	fmt.Printf("\n=== Transaction History ===\n")

	fmt.Printf("Enter wallet address: ")
	var address string
	if _, err := fmt.Scanln(&address); err != nil {
		return err
	}

	history := blockchain.GetTransactionHistory(address)

	fmt.Printf("Transaction history for %s (%d transactions):\n", address, len(history))

	for i, tx := range history {
		fmt.Printf("\nTransaction #%d:\n", i+1)
		fmt.Printf("  ID: %s\n", tx.GetID())
		fmt.Printf("  Protocol: %s\n", tx.GetProtocol())
		fmt.Printf("  Status: %s\n", tx.GetStatus())
	}

	return nil
}

func showMiningStatus(blockchain *sdk.Blockchain) error {
	info := blockchain.GetBlockchainInfo()

	fmt.Printf("\n=== Mining Status ===\n")
	fmt.Printf("Current Difficulty: %d\n", info.Difficulty)
	fmt.Printf("Block Time: %d seconds\n", info.BlockTime)
	fmt.Printf("Pending Transactions: %d\n", len(blockchain.GetPendingTransactions()))

	return nil
}

func adjustMiningDifficulty(blockchain *sdk.Blockchain) error {
	fmt.Printf("\n=== Adjust Mining Difficulty ===\n")

	fmt.Printf("Current difficulty: %d\n", blockchain.GetConfig().Difficulty)
	fmt.Printf("Enter new difficulty (1-10): ")

	var difficultyStr string
	if _, err := fmt.Scanln(&difficultyStr); err != nil {
		return err
	}

	difficulty, err := strconv.Atoi(difficultyStr)
	if err != nil || difficulty < 1 || difficulty > 10 {
		fmt.Printf("❌ Invalid difficulty. Must be between 1 and 10.\n")
		return err
	}

	// Update configuration
	config := blockchain.GetConfig()
	config.Difficulty = difficulty

	err = blockchain.UpdateConfig(config)
	if err != nil {
		fmt.Printf("❌ Failed to update difficulty: %v\n", err)
		return err
	}

	fmt.Printf("✅ Difficulty updated to %d\n", difficulty)
	return nil
}

func showPeers(blockchain *sdk.Blockchain) error {
	fmt.Printf("\n=== Network Peers ===\n")
	fmt.Printf("Peer management functionality needs to be implemented.\n")
	fmt.Printf("This would show connected peers and their status.\n")

	return nil
}

func showSyncStatus(blockchain *sdk.Blockchain) error {
	fmt.Printf("\n=== Synchronization Status ===\n")
	fmt.Printf("Synchronization functionality needs to be implemented.\n")
	fmt.Printf("This would show sync status with the network.\n")

	return nil
}

func showConfiguration(blockchain *sdk.Blockchain) error {
	config := blockchain.GetConfig()

	fmt.Printf("\n=== Current Configuration ===\n")
	fmt.Printf("Data Path: %s\n", config.DataPath)
	fmt.Printf("Difficulty: %d\n", config.Difficulty)
	fmt.Printf("Block Time: %d seconds\n", config.BlockTime)
	fmt.Printf("Token Count: %d\n", config.TokenCount)
	fmt.Printf("Fund Wallet Amount: %.2f\n", config.FundWalletAmount)
	fmt.Printf("Enable API: %t\n", config.EnableAPI)
	fmt.Printf("API Hostname: %s\n", config.APIHostName)
	fmt.Printf("P2P Hostname: %s\n", config.P2PHostName)

	return nil
}

func updateConfiguration(blockchain *sdk.Blockchain) error {
	fmt.Printf("\n=== Update Configuration ===\n")
	fmt.Printf("Configuration update functionality needs to be implemented.\n")
	fmt.Printf("This would allow updating various blockchain settings.\n")

	return nil
}
