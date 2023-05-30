package main

import (
	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
)

func main() {
	// Create a blockchain instance
	bc := sdk.NewBlockchain()

	go bc.Run(1)

	// This is to keep the main goroutine alive. Remove it if not necessary
	select {}
}
