package main

import (
	"fmt"
	"log"
	"math/big"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
)

func main() {

	ls := sdk.NewLocalStorage("./test_data")

	fromWallet, err := sdk.NewWallet("fromWallet", "pa$$W0RD123!", []string{"test", "wallet"})
	if err != nil {
		log.Fatal(err)
	}

	toWallet, err := sdk.NewWallet("toWallet", "pa$$W0RD123!", []string{"test", "wallet"})
	if err != nil {
		log.Fatal(err)
	}

	// Create a new NodeData instance
	nodeData := &sdk.NodeData{
		ID:   "node1",
		Name: "Node 1",
		// Additional fields as needed
	}

	// Create a new BlockchainData instance
	blockchainData := &sdk.BlockchainData{
		ID:      "blockchain1",
		Version: "1.0",
		// Additional fields as needed
	}

	// Create a new Transaction instance
	transaction, err := sdk.NewMessageTransaction(fromWallet, toWallet, "Hello World!")
	if err != nil {
		log.Fatal(err)
	}

	// Create a new Block instance
	block := &sdk.Block{
		Index:        *big.NewInt(1),
		Timestamp:    time.Now(),
		Transactions: []sdk.Transaction{},
		Nonce:        "1234567890",
		Hash:         "",
		PreviousHash: "",
	}
	block.Transactions = append(block.Transactions, transaction)

	// Set the NodeData
	err = ls.Set("node", nodeData)
	if err != nil {
		log.Println(err)
	}

	// Set the BlockchainData
	err = ls.Set("blockchain", blockchainData)
	if err != nil {
		log.Println(err)
	}

	// Set the Block
	err = ls.Set("blocks", block)
	if err != nil {
		log.Println(err)
	}

	// Set the Transaction
	err = ls.Set("transactions", transaction)
	if err != nil {
		log.Println(err)
	}

	// Query the Block
	criteria := &sdk.BlockQueryCriteria{
		Number: 1,
		// Additional criteria fields as needed
	}
	results, err := ls.Find(criteria)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Query results: %+v\n", results)

	// Query the Transaction
	txCriteria := &sdk.TransactionQueryCriteria{
		Amount: 100.0,
		// Additional criteria fields as needed
	}
	txResults, err := ls.Find(txCriteria)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Query results: %+v\n", txResults)
}
