package sdk

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewP2P(t *testing.T) {
	p2p := NewP2P()
	assert.NotNil(t, p2p)
	assert.Empty(t, p2p.nodes)
	assert.Empty(t, p2p.queue)
	assert.False(t, p2p.running)
}

func TestRegisterNode(t *testing.T) {
	p2p := NewP2P()
	node := &Node{ID: "test-node"}

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

func TestIsRegistered(t *testing.T) {
	p2p := NewP2P()
	node := &Node{ID: "test-node"}
	p2p.RegisterNode(node)

	assert.True(t, p2p.IsRegistered("test-node"))
	assert.False(t, p2p.IsRegistered("non-existent-node"))
}

func TestBroadcastMessage(t *testing.T) {
	p2p := NewP2P()
	node1 := &Node{ID: "node1", Blockchain: &Blockchain{}}
	node2 := &Node{ID: "node2", Blockchain: &Blockchain{}}
	p2p.RegisterNode(node1)
	p2p.RegisterNode(node2)

	msg := P2PTransaction{
		Tx:     Tx{ID: NewPUIDThis()},
		Target: "all",
		Action: "test",
		Data:   "test-data",
	}

	err := p2p.BroadcastMessage(msg)
	assert.NoError(t, err)

	// Test broadcasting to an empty network
	p2p = NewP2P()
	err = p2p.BroadcastMessage(msg)
	assert.Error(t, err)
}

func TestOneThirdAndTwoThirds(t *testing.T) {
	p2p := NewP2P()
	for i := 0; i < 10; i++ {
		p2p.RegisterNode(&Node{ID: fmt.Sprintf("node%d", i)})
	}

	assert.Equal(t, 3, p2p.OneThird())
	assert.Equal(t, 6, p2p.TwoThirds())

	// Test with less than 3 nodes
	p2p = NewP2P()
	p2p.RegisterNode(&Node{ID: "node1"})
	assert.Equal(t, 1, p2p.OneThird())
	assert.Equal(t, 2, p2p.TwoThirds())
}

func TestGetRandomOneThirdAndTwoThirds(t *testing.T) {
	p2p := NewP2P()
	for i := 0; i < 10; i++ {
		p2p.RegisterNode(&Node{ID: fmt.Sprintf("node%d", i)})
	}

	oneThird := p2p.GetRandomOneThird()
	twoThirds := p2p.GetRandomTwoThirds()

	assert.Equal(t, 3, len(oneThird))
	assert.Equal(t, 6, len(twoThirds))

	// Check that the selections are random
	oneThirdAgain := p2p.GetRandomOneThird()
	assert.NotEqual(t, oneThird, oneThirdAgain)
}

func TestAddTransaction(t *testing.T) {
	p2p := NewP2P()
	tx := P2PTransaction{
		Tx:     Tx{ID: NewPUIDThis()},
		Target: "all",
		Action: "test",
		Data:   "test-data",
	}

	p2p.AddTransaction(tx)
	assert.Equal(t, 1, len(p2p.queue))
}

func TestHasTransaction(t *testing.T) {
	p2p := NewP2P()
	node := &Node{
		ID: "test-node",
		Blockchain: &Blockchain{
			Blocks: []*Block{
				{
					Transactions: []Transaction{
						&Tx{ID: NewPUIDThis()},
					},
				},
			},
		},
	}
	p2p.RegisterNode(node)

	assert.True(t, p2p.HasTransaction(node.Blockchain.Blocks[0].Transactions[0].(*Tx).ID))
	assert.False(t, p2p.HasTransaction(NewPUIDThis()))
}

func TestProcessQueue(t *testing.T) {
	p2p := NewP2P()
	node := &Node{ID: "test-node", Blockchain: &Blockchain{}}
	p2p.RegisterNode(node)

	tx1 := P2PTransaction{
		Tx:     Tx{ID: NewPUIDThis()},
		Target: "node",
		Action: "validate",
	}
	tx2 := P2PTransaction{
		Tx:     Tx{ID: NewPUIDThis()},
		Target: "node",
		Action: "status",
	}
	p2p.AddTransaction(tx1)
	p2p.AddTransaction(tx2)

	p2p.ProcessQueue()
	assert.Empty(t, p2p.queue)
}

func TestBroadcast(t *testing.T) {
	p2p := NewP2P()
	node1 := &Node{ID: "node1", Blockchain: &Blockchain{}}
	node2 := &Node{ID: "node2", Blockchain: &Blockchain{}}
	p2p.RegisterNode(node1)
	p2p.RegisterNode(node2)

	tx := P2PTransaction{
		Tx:     Tx{ID: NewPUIDThis()},
		Target: "all",
		Action: "test",
		Data:   "test-data",
	}

	err := p2p.Broadcast(tx)
	assert.NoError(t, err)

	// Test broadcasting to an empty network
	p2p = NewP2P()
	err = p2p.Broadcast(tx)
	assert.Error(t, err)
}

func TestStartAndIsRunning(t *testing.T) {
	p2p := NewP2P()
	assert.False(t, p2p.IsRunning())

	err := p2p.Start()
	assert.NoError(t, err)
	assert.True(t, p2p.IsRunning())

	// Try to start again
	err = p2p.Start()
	assert.Error(t, err)
}

func TestRunProcessQueueAndNodeDiscovery(t *testing.T) {
	p2p := NewP2P()
	p2p.Start()

	// Add a transaction to the queue
	tx := P2PTransaction{
		Tx:     Tx{ID: NewPUIDThis()},
		Target: "all",
		Action: "test",
		Data:   "test-data",
	}
	p2p.AddTransaction(tx)

	// Wait for a short time to allow processing
	time.Sleep(1 * time.Second)

	assert.Empty(t, p2p.queue)
}

func TestBroadcastStatus(t *testing.T) {
	p2p := NewP2P()
	node := &Node{
		ID:         "test-node",
		Wallet:     &Wallet{},
		Blockchain: &Blockchain{},
	}
	p2p.RegisterNode(node)

	err := p2p.BroadcastStatus(node, "active")
	assert.NoError(t, err)
}

func TestP2PTransactionState(t *testing.T) {
	state := P2PTxNone
	assert.Equal(t, "NONE", state.String())

	state.Next()
	assert.Equal(t, P2PTxQueued, state)

	newState, err := P2PTransactionStateFromString("VALID")
	assert.NoError(t, err)
	assert.Equal(t, P2PTxValid, newState)

	_, err = P2PTransactionStateFromString("INVALID")
	assert.Error(t, err)
}

func TestMax(t *testing.T) {
	assert.Equal(t, 5, max(3, 5))
	assert.Equal(t, 5, max(5, 3))
	assert.Equal(t, 5, max(5, 5))
}
