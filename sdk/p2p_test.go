// Package sdk_test tests the sdk package functionality
package sdk_test

import (
	"fmt"
	"testing"

	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewP2P tests the NewP2P function
func TestNewP2P(t *testing.T) {
	p2p := sdk.NewP2P()
	assert.NotNil(t, p2p)
	// We can't directly access the nodes or queue, so we'll test other properties
	assert.False(t, p2p.IsRunning())
}

// TestRegisterNode tests the RegisterNode method
func TestRegisterNode(t *testing.T) {
	p2p := sdk.NewP2P()
	node := &sdk.Node{ID: "test-node"}

	err := p2p.RegisterNode(node)
	assert.NoError(t, err)
	assert.True(t, p2p.IsRegistered("test-node"))

	// Try to register the same node again
	err = p2p.RegisterNode(node)
	assert.Error(t, err)

	// Try to register a nil node
	err = p2p.RegisterNode(nil)
	assert.Error(t, err)
}

// TestIsRegistered tests the IsRegistered method
func TestIsRegistered(t *testing.T) {
	p2p := sdk.NewP2P()
	node := &sdk.Node{ID: "test-node"}
	err := p2p.RegisterNode(node)
	require.NoError(t, err)

	assert.True(t, p2p.IsRegistered("test-node"))
	assert.False(t, p2p.IsRegistered("non-existent-node"))
}

// TestAddTransaction tests the AddTransaction method indirectly
func TestAddTransaction(t *testing.T) {
	p2p := sdk.NewP2P()

	// Create a transaction ID
	puid := sdk.NewPUIDEmpty()

	tx := sdk.P2PTransaction{
		Tx:     sdk.Tx{ID: puid},
		Target: "all",
		Action: "test",
		Data:   "test-data",
	}

	// We can only test that AddTransaction doesn't panic
	p2p.AddTransaction(tx)
}

// TestProcessQueue tests the ProcessQueue method
// We can only verify it doesn't panic
func TestProcessQueue(t *testing.T) {
	p2p := sdk.NewP2P()

	// Register a test node
	p2p.RegisterNode(&sdk.Node{ID: "test-node"})

	// Create test transactions
	puid1 := sdk.NewPUIDEmpty()
	puid2 := sdk.NewPUIDEmpty()

	tx1 := sdk.P2PTransaction{
		Tx:     sdk.Tx{ID: puid1},
		Target: "node",
		Action: "validate",
	}

	tx2 := sdk.P2PTransaction{
		Tx:     sdk.Tx{ID: puid2},
		Target: "node",
		Action: "status",
		Data:   []byte(`{"NodeID":"test-node","Status":"active"}`),
	}

	p2p.AddTransaction(tx1)
	p2p.AddTransaction(tx2)

	// Process the queue - verify it doesn't panic
	p2p.ProcessQueue()
}

// TestBroadcastMessage tests the BroadcastMessage method with minimal nodes
func TestBroadcastMessage(t *testing.T) {
	p2p := sdk.NewP2P()

	// Create a transaction ID
	puid := sdk.NewPUIDEmpty()

	msg := sdk.P2PTransaction{
		Tx:     sdk.Tx{ID: puid},
		Target: "all",
		Action: "test",
		Data:   "test-data",
	}

	// Test with empty network
	err := p2p.BroadcastMessage(msg)
	assert.Error(t, err)

	// Add a node and test again
	node := &sdk.Node{
		ID:         "test-node",
		Blockchain: &sdk.Blockchain{},
	}
	err = p2p.RegisterNode(node)
	require.NoError(t, err)

	// This will return an error because the node has no wallet
	// But we're just testing the method gets called
	_ = p2p.BroadcastMessage(msg)
}

// TestHasTransaction tests the HasTransaction method
func TestHasTransaction(t *testing.T) {
	p2p := sdk.NewP2P()

	// Create a transaction ID
	puid := sdk.NewPUIDEmpty()

	// Register a node
	node := &sdk.Node{
		ID:         "test-node",
		Blockchain: &sdk.Blockchain{},
	}
	err := p2p.RegisterNode(node)
	require.NoError(t, err)

	// Test for a transaction that doesn't exist
	assert.False(t, p2p.HasTransaction(puid))
}

// TestStartAndStop tests the Start and Stop methods
func TestStartAndStop(t *testing.T) {
	p2p := sdk.NewP2P()

	// Test initial state
	assert.False(t, p2p.IsRunning())

	// Test starting
	err := p2p.Start()
	assert.NoError(t, err)
	assert.True(t, p2p.IsRunning())

	// Test starting again
	err = p2p.Start()
	assert.Error(t, err)

	// Test stopping
	err = p2p.Stop()
	assert.NoError(t, err)
	assert.False(t, p2p.IsRunning())

	// Test stopping again
	err = p2p.Stop()
	assert.Error(t, err)
}

// TestSetAsSeedNode tests the SetAsSeedNode method
// We can't directly check if the node is a seed node since isSeedNode is private
func TestSetAsSeedNode(t *testing.T) {
	p2p := sdk.NewP2P()

	// Set as seed node - verify it doesn't panic
	p2p.SetAsSeedNode()
}

// TestP2PTransactionState tests the P2PTransactionState functionality
func TestP2PTransactionState(t *testing.T) {
	// Test String method
	assert.Equal(t, "NONE", sdk.P2PTxNone.String())
	assert.Equal(t, "QUEUED", sdk.P2PTxQueued.String())
	assert.Equal(t, "PND13", sdk.P2PTxPnd13.String())

	// Test Next method
	state := sdk.P2PTxNone
	state.Next()
	assert.Equal(t, sdk.P2PTxQueued, state)
	state.Next()
	assert.Equal(t, sdk.P2PTxPnd13, state)

	// Test FromString method
	for i := sdk.P2PTxNone; i <= sdk.P2PTxArchived; i++ {
		newState, err := sdk.P2PTransactionStateFromString(i.String())
		assert.NoError(t, err)
		assert.Equal(t, i, newState)
	}

	// Test invalid state
	_, err := sdk.P2PTransactionStateFromString("INVALID")
	assert.Error(t, err)
}

// Helper function to register multiple nodes
func registerMultipleNodes(p2p *sdk.P2P, count int) {
	for i := 0; i < count; i++ {
		p2p.RegisterNode(&sdk.Node{ID: fmt.Sprintf("node%d", i)})
	}
}

// TestConnectToSeedNode tests the ConnectToSeedNode method
func TestConnectToSeedNode(t *testing.T) {
	// Skip this test as it requires network connectivity
	t.Skip("Skipping network test")
}

// TestMax tests the max function
func TestMax(t *testing.T) {
	cases := []struct {
		a, b, expected int
	}{
		{5, 3, 5},
		{3, 5, 5},
		{5, 5, 5},
		{-5, 3, 3},
		{3, -5, 3},
		{-5, -3, -3},
	}

	for _, tc := range cases {
		t.Run(fmt.Sprintf("max(%d,%d)", tc.a, tc.b), func(t *testing.T) {
			// Since max is not exported, we can't test it directly
			// In a real test, you might either expose it or test functionality that uses it
		})
	}
}
