// Package sdk is a software development kit for building blockchain applications.
// File  sdk/node_test.go - Node Test for all Node related Protocol based transactions
package sdk

import (
	"testing"
)

const (
	DummyTXID = "1234:5678:90ab:cdef"
)

// TestNewNode tests the NewNode func
func TestNewNode(t *testing.T) {
	// Create a 1st Node instance
	node1 := NewNode(nil)
	if !node1.IsReady() {
		t.Errorf("Failed to create 1st Node instance")
	}

	// Create a 2nd Node instance
	node2 := NewNode(nil)
	if !node1.IsReady() {
		t.Errorf("Failed to create 2nd Node instance")
	}

	// Check if the two node instances are different
	if node1.ID == node2.ID {
		t.Errorf("1st Node instance ID and 2nd Node instance ID are same")
	}

}

// TestRegister tests the Register func
func TestRegister(t *testing.T) {

	// Create 3 node instances
	node1 := NewNode(nil)
	node2 := NewNode(nil)
	node3 := NewNode(nil)

	// Register the nodes with the P2P network
	node1.Register()
	node2.Register()
	node3.Register()

	// Check if the nodes have been registered successfully
	if !node1.P2P.IsRegistered(node1.ID) || !node2.P2P.IsRegistered(node2.ID) || !node3.P2P.IsRegistered(node3.ID) {
		t.Errorf("Failed to register nodes with P2P network")
	}

}

// TestAddTransaction tests the AddTransaction func
func TestAddTransaction(t *testing.T) {

	// Create a dummy P2P transaction
	tx := P2PTransaction{
		Tx:     Tx{ID: "0x0123456789"},
		Target: "node",
		Action: "register",
		Data:   "example node data",
	}

	// Create 3 node instances
	node1 := NewNode(nil)
	node2 := NewNode(nil)
	node3 := NewNode(nil)

	// Add the transaction to each node's P2P network
	node1.P2P.AddTransaction(tx)
	node2.P2P.AddTransaction(tx)
	node3.P2P.AddTransaction(tx)

	// Check if the transaction has been added successfully
	if !node1.P2P.HasTransaction("0x0123456789") || !node2.P2P.HasTransaction("0x0123456789") || !node3.P2P.HasTransaction("0x0123456789") {
		t.Errorf("Failed to add transaction to transactions queue")
	}

}

// TestBroadcast tests the Broadcast func
func TestBroadcast(t *testing.T) {

	// Create a dummy P2P transaction
	tx := P2PTransaction{
		Tx:     Tx{ID: NewPUIDFromString(DummyTXID)}, // "0x1234567890" -> 1234:5678:90ab:cdef
		Target: "node",
		Action: "add",
		Data:   "example node data",
	}

	// Create 3 node instances
	node1 := NewNode(nil)
	node2 := NewNode(nil)
	node3 := NewNode(nil)

	// Broadcast the transaction to each node's P2P network
	node1.P2P.Broadcast(tx)
	node2.P2P.Broadcast(tx)
	node3.P2P.Broadcast(tx)

	// Check if the transaction has been broadcast successfully
	if !node1.P2P.HasTransaction("0x1234567890") || !node2.P2P.HasTransaction("0x1234567890") || !node3.P2P.HasTransaction("0x1234567890") {
		t.Errorf("Failed to broadcast transaction")
	}

}
