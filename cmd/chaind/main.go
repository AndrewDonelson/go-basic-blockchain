package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/menu"
	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
)

const (
	menuActionOpenMenu = "open-menu"
	menuActionExit     = "exit"
	menuActionHelp     = "help"
	menuActionUnknown  = "unknown"
)

func buildNodeOptionsFromArgs(seed bool, seedAddress string) *sdk.NodeOptions {
	nodeOpts := sdk.DefaultNodeOptions()
	nodeOpts.IsSeed = seed
	nodeOpts.SeedAddress = seedAddress
	return nodeOpts
}

func menuCommandAction(input string) string {
	normalized := strings.ToLower(strings.TrimSpace(input))
	switch normalized {
	case "", "menu":
		return menuActionOpenMenu
	case "quit", "exit":
		return menuActionExit
	case "help":
		return menuActionHelp
	default:
		return menuActionUnknown
	}
}

func main() {
	// Parse command-line flags
	err := sdk.Args.Parse()
	if err == sdk.ErrNoArgs {
		fmt.Println("No arguments provided. Using default configuration.")
		fmt.Println("Use -h or --help for usage information.")
	} else if err != nil {
		fmt.Printf("Error parsing arguments: %v\n", err)
		os.Exit(1)
	}

	// Get the custom environment file path if provided
	envFile := sdk.Args.GetString("env")
	if envFile != "" {
		// Set the environment file path
		os.Setenv("ENV_FILE", envFile)
		log.Printf("Using custom environment file: %s", envFile)
	}

	// Create node options using the parsed flags
	nodeOpts := buildNodeOptionsFromArgs(sdk.Args.GetBool("seed"), sdk.Args.GetString("seed-address"))

	// Create the node
	err = sdk.NewNode(nodeOpts)
	if err != nil {
		log.Fatalf("Failed to create node: %v", err)
		os.Exit(1)
	}

	// Set global verbose flag for logging
	sdk.ConfigSetVerbose(nodeOpts.Config.Verbose)

	// Get the node instance
	node := sdk.GetNode()
	if node == nil {
		fmt.Println("Failed to get node instance")
		os.Exit(1)
	}

	// Start the node in a goroutine
	go node.Run()

	// Start the interactive menu system
	fmt.Println("\n🚀 Go Basic Blockchain started!")
	fmt.Println("Press ENTER to open the interactive menu, or let the blockchain run automatically.")
	fmt.Println("Type 'menu' and press ENTER to open the menu at any time.")

	// Start a goroutine to handle menu input
	go handleMenuInput(node.Blockchain)

	// Set up signal handler for clean shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// Keep the main thread alive
	<-sigChan
	fmt.Println("\nShutting down blockchain...")
	if node.Blockchain != nil {
		node.Blockchain.Cleanup()
	}
	os.Exit(0)
}

// handleMenuInput handles user input for menu activation
func handleMenuInput(blockchain *sdk.Blockchain) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("\n> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		switch menuCommandAction(input) {
		case menuActionOpenMenu:
			// Create and start the menu system
			menuSystem := menu.CreateBlockchainMenu(blockchain)
			fmt.Println("\nOpening interactive menu...")

			if err := menuSystem.Navigate(); err != nil {
				fmt.Printf("Menu error: %v\n", err)
			}

			fmt.Println("\nMenu closed. Blockchain continues running.")
			fmt.Println("Press ENTER or type 'menu' to open the menu again.")
		case menuActionExit:
			fmt.Println("Shutting down blockchain...")
			os.Exit(0)
		case menuActionHelp:
			fmt.Println("Available commands:")
			fmt.Println("  ENTER or 'menu' - Open interactive menu")
			fmt.Println("  'quit' or 'exit' - Shutdown blockchain")
			fmt.Println("  'help' - Show this help")
		case menuActionUnknown:
			input = strings.TrimSpace(input)
			fmt.Printf("Unknown command: %s. Type 'help' for available commands.\n", input)
		}
	}
}
