// file: cmd/chaind/main.go
// main is the entry point for the chaind application, which creates a new node
// instance and runs it. The node is configured with various options, such as the
// blockchain name, symbol, block time, difficulty, transaction fee, miner and
// developer reward percentages, API and P2P hostnames, whether the API is
// enabled, the initial wallet amount, token count and price, and whether new
// tokens are allowed.
package main

import (
	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
)

func main() {
	// NewNodeOptions creates a new node options struct with the specified configuration.
	// The node options include the blockchain name, symbol, block time, difficulty,
	// transaction fee, miner and developer reward percentages, API and P2P hostnames,
	// whether the API is enabled, the initial wallet amount, token count and price,
	// and whether new tokens are allowed.
	//nodeOpts := sdk.NewNodeOptions("chaind", "./chaind_data", sdk.NewConfig())

	// NewNode creates a new node instance with the provided options.
	// The node is responsible for managing the blockchain, including creating
	// and validating blocks, handling transactions, and providing an API for
	// interacting with the blockchain.
	sdk.NewNode(nil)

	// Run starts the node and blocks until the node is stopped.
	// The node will handle incoming connections, process transactions,
	// mine new blocks, and provide an API for interacting with the
	// blockchain.
	sdk.GetNode().Run()
}
