package sdk

import (
	"fmt"
	"testing"
)

// TestIsRegistered tests the IsRegistered() method.
func TestIsRegistered(t *testing.T) {
	// Create a new P2P network
	p := NewP2P()

	// Create two nodes
	node1 := NewNode(
		NewNodeOptions(".testNode1", "./data_test",
			&Config{
				BlockchainName:   "TB1testBlockchain_1",
				BlockchainSymbol: "TB1",
				BlockTime:        blockTimeInSec,
				Difficulty:       proofOfWorkDifficulty,
				TransactionFee:   transactionFee,
				MinerRewardPCT:   minerRewardPCT,
				MinerAddress:     "",
				DevRewardPCT:     devRewardPCT,
				DevAddress:       "",
				APIHostName:      ":8080",
				P2PHostName:      ":5000",
				EnableAPI:        true,
				FundWalletAmount: fundWalletAmount,
				TokenCount:       tokenCount,
				TokenPrice:       tokenPrice,
				AllowNewTokens:   allowNewTokens,
				testing:          true,
			}))

	node2 := &Node{
		ID: "node2",
	}

	// Register two nodes with the P2P network
	p.RegisterNode(node1)
	p.RegisterNode(node2)

	// Test isRegistered()
	if !p.IsRegistered(node1.ID) {
		t.Errorf("expected %s to be registered", node1.ID)
	}
	if !p.IsRegistered(node2.ID) {
		t.Errorf("expected %s to be registered", node2.ID)
	}
	if p.IsRegistered("invalid-node") {
		t.Error("expected invalid-node to not be registered")
	}
}

func TestProcessQueue(t *testing.T) {
	node := NewNode(nil)
	p2p := NewP2P()
	p2p.RegisterNode(node)
	p2p.ProcessQueue() // should do nothing

	// Add some mock transactions
	tx1 := P2PTransaction{
		Tx:     Tx{ID: "tx1"},
		Target: "node",
		Action: "status",
	}
	tx2 := P2PTransaction{
		Tx:     Tx{ID: "tx2"},
		Target: "node",
		Action: "add",
	}
	tx3 := P2PTransaction{
		Tx:     Tx{ID: "tx3"},
		Target: "node",
		Action: "remove",
	}
	tx4 := P2PTransaction{
		Tx:     Tx{ID: "tx4"},
		Target: "node",
		Action: "register",
	}
	tx5 := P2PTransaction{
		Tx:     Tx{ID: "tx5"},
		Target: "node",
		Action: "validate",
	}

	p2p.AddTransaction(tx1)
	p2p.AddTransaction(tx2)
	p2p.AddTransaction(tx3)
	p2p.AddTransaction(tx4)
	p2p.AddTransaction(tx5)

	// Start processing the transaction queue
	p2p.ProcessQueue()
	p2p.ProcessQueue() // should do nothing

	// Check if the transaction is in the network
	if !p2p.HasTransaction("tx1") {
		t.Errorf("Transaction not found in the network")
	}

	fmt.Printf("Test ProcessQueue passed\n")
}
