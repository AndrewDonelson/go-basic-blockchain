package main

import (
	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
)

func main() {
	// Create a blockchain instance
	bc := sdk.NewBlockchain()

	// Run the blockchain as a goroutine
	go bc.Run(1)

	// Start the API server if enabled
	if sdk.EnableAPI {
		// Start the API server
		sdk.NewAPI(bc).Start(":8080")
	} else {
		// This is to keep the main goroutine alive if API not enabled.
		select {}
	}
}
