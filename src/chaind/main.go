package main

import (
	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
)

func main() {
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

	// Create a new node iunstance
	node := sdk.NewNode(nodeOpts)

	// Run the node
	node.Run()
}
