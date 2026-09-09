package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/menu"
	"github.com/AndrewDonelson/go-basic-blockchain/internal/progress"
	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
	"golang.org/x/term"
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
		if err := os.Setenv("ENV_FILE", envFile); err != nil {
			log.Fatalf("Failed to set ENV_FILE: %v", err)
		}
		log.Printf("Using custom environment file: %s", envFile)
	}

	// Create node options using the parsed flags
	nodeOpts := buildNodeOptionsFromArgs(sdk.Args.GetBool("seed"), sdk.Args.GetString("seed-address"))

	// Create the node
	err = sdk.NewNode(nodeOpts)
	if err != nil {
		// log.Fatalf already exits; the os.Exit(1) that used to follow was dead.
		log.Fatalf("Failed to create node: %v", err)
	}

	// Set global verbose flag for logging
	sdk.ConfigSetVerbose(nodeOpts.Config.Verbose)

	// Route the standard logger through the progress indicator. This is the
	// application's call to make; it used to happen implicitly inside
	// progress.NewProgressIndicator().
	progress.InstallLogWriter()

	// Get the node instance
	node := sdk.GetNode()
	if node == nil {
		fmt.Println("Failed to get node instance")
		os.Exit(1)
	}

	// Run the node under a context we can cancel on a signal.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	nodeDone := make(chan struct{})
	go func() {
		defer close(nodeDone)
		node.RunContext(ctx)
	}()

	fmt.Println("\n🚀 Go Basic Blockchain started!")

	// Only offer the interactive menu when there is a terminal to drive it.
	//
	// handleMenuInput used to run unconditionally. Under systemd, in a container,
	// or with stdin redirected from /dev/null, every ReadString returns io.EOF
	// immediately and the old loop printed an error and `continue`d -- an
	// unbounded print-and-spin at 100% CPU.
	if term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Println("Press ENTER to open the interactive menu, or let the blockchain run automatically.")
		fmt.Println("Type 'menu' and press ENTER to open the menu at any time.")
		go handleMenuInput(ctx, cancel, node.Blockchain)
	} else {
		fmt.Println("No terminal attached; running non-interactively.")
	}

	// Set up signal handler for clean shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigChan:
		fmt.Println("\nShutting down blockchain...")
	case <-ctx.Done():
	}

	cancel()

	// Give the node a bounded window to flush state and drain connections.
	select {
	case <-nodeDone:
	case <-time.After(15 * time.Second):
		fmt.Println("Shutdown timed out; exiting anyway.")
	}
}

// handleMenuInput handles user input for menu activation.
func handleMenuInput(ctx context.Context, shutdown context.CancelFunc, blockchain *sdk.Blockchain) {
	reader := bufio.NewReader(os.Stdin)

	for {
		if ctx.Err() != nil {
			return
		}

		fmt.Print("\n> ")
		input, err := reader.ReadString('\n')
		if err != nil {
			// EOF means stdin is gone; there is nothing left to read and looping
			// would spin forever.
			if errors.Is(err, io.EOF) {
				return
			}
			fmt.Printf("Error reading input: %v\n", err)
			return
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
			// Signal a clean shutdown instead of os.Exit, which skipped every
			// flush and left the last block's state unsaved.
			shutdown()
			return
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
