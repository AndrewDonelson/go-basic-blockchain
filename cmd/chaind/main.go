package main

import (
	"fmt"
	"log"
	"os"

	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
)

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
	nodeOpts := sdk.DefaultNodeOptions()

	// Apply command-line flags to node options
	nodeOpts.IsSeed = sdk.Args.GetBool("seed")
	nodeOpts.SeedAddress = sdk.Args.GetString("seed-address")

	// Create the node
	err = sdk.NewNode(nodeOpts)
	if err != nil {
		log.Fatalf("Failed to create node: %v", err)
		os.Exit(1)
	}

	// Get the node instance and run it
	node := sdk.GetNode()
	if node == nil {
		fmt.Println("Failed to get node instance")
		os.Exit(1)
	}

	node.Run()
}
