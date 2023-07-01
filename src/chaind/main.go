package main

import (
	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
)

func main() {
	// Create a new node iunstance
	node := sdk.NewNode()

	// Run the node
	node.Run()
}
