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
	nodeOpts := sdk.NewNodeOptions("chaind", "./chaind_data",
		&sdk.Config{
			BlockchainName:   "Go Basic Blockchain",
			BlockchainSymbol: "GBB",
			BlockTime:        3,
			Difficulty:       4,
			TransactionFee:   0.05,
			MinerRewardPCT:   0.5,
			MinerAddress:     "",
			DevRewardPCT:     0.5,
			DevAddress:       "",
			APIHostName:      ":8080",
			P2PHostName:      ":5000",
			EnableAPI:        true,
			FundWalletAmount: 100,
			TokenCount:       1000000,
			TokenPrice:       1,
			AllowNewTokens:   false,
			DataPath:         "./data",
		})

	// NewNode creates a new node instance with the provided options.
	// The node is responsible for managing the blockchain, including creating
	// and validating blocks, handling transactions, and providing an API for
	// interacting with the blockchain.
	node := sdk.NewNode(nodeOpts)

	// Run starts the node and blocks until the node is stopped.
	// The node will handle incoming connections, process transactions,
	// mine new blocks, and provide an API for interacting with the
	// blockchain.
	node.Run()
}
