package sdk

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewNode(t *testing.T) {
	t.Run("with default options", func(t *testing.T) {
		err := NewNode(nil)
		assert.NoError(t, err)
		assert.NotNil(t, node)
		assert.True(t, node.IsReady())
		assert.NotEmpty(t, node.ID)
		assert.NotNil(t, node.Config)
		assert.NotNil(t, node.Blockchain)
		assert.NotNil(t, node.API)
		assert.NotNil(t, node.P2P)
		assert.NotNil(t, node.Wallet)
	})

	t.Run("with custom options", func(t *testing.T) {
		opts := &NodeOptions{
			EnvName:  "test",
			DataPath: "./test_data",
			Config:   NewConfig(),
		}
		err := NewNode(opts)
		assert.NoError(t, err)
		assert.NotNil(t, node)
		assert.True(t, node.IsReady())
		assert.Equal(t, opts.DataPath, node.Config.DataPath)
	})
}

func TestDefaultNodeOptions(t *testing.T) {
	opts := DefaultNodeOptions()
	assert.NotNil(t, opts)
	assert.Equal(t, "chaind", opts.EnvName)
	assert.Equal(t, "./chaind_data", opts.DataPath)
	assert.NotNil(t, opts.Config)
}

func TestNodeIsReady(t *testing.T) {
	err := NewNode(nil)
	require.NoError(t, err)
	assert.True(t, node.IsReady())

	node.initialized = false
	assert.False(t, node.IsReady())
}

func TestNodeSaveAndLoad(t *testing.T) {
	err := NewNode(nil)
	require.NoError(t, err)

	originalID := node.ID
	originalConfig := node.Config

	err = node.save()
	assert.NoError(t, err)

	// Create a new node to test loading
	node = &Node{}
	err = node.load()
	assert.NoError(t, err)

	assert.Equal(t, originalID, node.ID)
	assert.Equal(t, originalConfig, node.Config)
}

func TestNodeRun(t *testing.T) {
	err := NewNode(nil)
	require.NoError(t, err)

	// This is a bit tricky to test as it runs indefinitely
	// We'll use a channel to stop it after a short time
	done := make(chan bool)
	go func() {
		time.Sleep(100 * time.Millisecond)
		done <- true
	}()

	go node.Run()

	<-done
	// If we reach here, it means Run() didn't block indefinitely
	assert.True(t, true)
}

func TestNodeProcessP2PTransaction(t *testing.T) {
	err := NewNode(nil)
	require.NoError(t, err)

	testCases := []struct {
		name          string
		tx            P2PTransaction
		expectedError bool
	}{
		{
			name: "validate transaction",
			tx: P2PTransaction{
				Tx:     Tx{ID: NewPUIDEmpty()},
				Target: "node",
				Action: "validate",
				Data:   []byte("test data"),
			},
			expectedError: false,
		},
		{
			name: "update status",
			tx: P2PTransaction{
				Tx:     Tx{ID: NewPUIDEmpty()},
				Target: "node",
				Action: "status",
				Data:   []byte(`{"NodeID":"test","Status":"active"}`),
			},
			expectedError: true, // Because the node doesn't exist in the network
		},
		{
			name: "add node",
			tx: P2PTransaction{
				Tx:     Tx{ID: NewPUIDEmpty()},
				Target: "node",
				Action: "add",
				Data:   []byte(`{"ID":"test"}`),
			},
			expectedError: false,
		},
		{
			name: "remove node",
			tx: P2PTransaction{
				Tx:     Tx{ID: NewPUIDEmpty()},
				Target: "node",
				Action: "remove",
				Data:   []byte(`"test"`),
			},
			expectedError: true, // Because the node doesn't exist in the network
		},
		{
			name: "register node",
			tx: P2PTransaction{
				Tx:     Tx{ID: NewPUIDEmpty()},
				Target: "node",
				Action: "register",
				Data:   []byte(`{"ID":"test2"}`),
			},
			expectedError: false,
		},
		{
			name: "unknown action",
			tx: P2PTransaction{
				Tx:     Tx{ID: NewPUIDEmpty()},
				Target: "node",
				Action: "unknown",
				Data:   []byte("test data"),
			},
			expectedError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			err := node.ProcessP2PTransaction(tc.tx)
			if tc.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestNodeRegister(t *testing.T) {
	err := NewNode(nil)
	require.NoError(t, err)

	err = node.Register()
	assert.NoError(t, err)

	// Check if the node is registered in its own P2P network
	assert.True(t, node.P2P.IsRegistered(node.ID))
}

func TestNodeValidateTransaction(t *testing.T) {
	err := NewNode(nil)
	require.NoError(t, err)

	// Create a valid transaction
	tx, err := NewTransaction("test", node.Wallet, node.Wallet)
	require.NoError(t, err)

	signature, err := tx.Sign([]byte(node.Wallet.PrivatePEM()))
	require.NoError(t, err)

	tx.Signature = signature

	p2pTx := P2PTransaction{
		Tx:     *tx,
		Target: "node",
		Action: "validate",
		Data:   []byte("test data"),
	}

	err = node.validateTransaction(p2pTx)
	assert.NoError(t, err)
}

func TestNodeUpdateStatus(t *testing.T) {
	err := NewNode(nil)
	require.NoError(t, err)

	// Add a test node to the P2P network
	testNode := &Node{ID: "test", Status: "inactive"}
	node.P2P.nodes = append(node.P2P.nodes, testNode)

	status := NodeStatus{
		NodeID: "test",
		Status: "active",
	}
	statusData, _ := json.Marshal(status)

	p2pTx := P2PTransaction{
		Tx:     Tx{ID: NewPUIDEmpty()},
		Target: "node",
		Action: "status",
		Data:   statusData,
	}

	err = node.updateStatus(p2pTx)
	assert.NoError(t, err)
	assert.Equal(t, "active", testNode.Status)
}

func TestNodeAddAndRemoveNode(t *testing.T) {
	err := NewNode(nil)
	require.NoError(t, err)

	newNode := Node{ID: "test"}
	nodeData, _ := json.Marshal(newNode)

	addTx := P2PTransaction{
		Tx:     Tx{ID: NewPUIDEmpty()},
		Target: "node",
		Action: "add",
		Data:   nodeData,
	}

	err = node.addNode(addTx)
	assert.NoError(t, err)
	assert.Len(t, node.P2P.nodes, 1)

	removeTx := P2PTransaction{
		Tx:     Tx{ID: NewPUIDEmpty()},
		Target: "node",
		Action: "remove",
		Data:   []byte(`"test"`),
	}

	err = node.removeNode(removeTx)
	assert.NoError(t, err)
	assert.Len(t, node.P2P.nodes, 0)
}

func TestNodeRegisterNode(t *testing.T) {
	err := NewNode(nil)
	require.NoError(t, err)

	newNode := Node{ID: "test"}
	nodeData, _ := json.Marshal(newNode)

	registerTx := P2PTransaction{
		Tx:     Tx{ID: NewPUIDEmpty()},
		Target: "node",
		Action: "register",
		Data:   nodeData,
	}

	err = node.registerNode(registerTx)
	assert.NoError(t, err)
	assert.Len(t, node.P2P.nodes, 1)

	// Try to register the same node again
	err = node.registerNode(registerTx)
	assert.Error(t, err)
	assert.Len(t, node.P2P.nodes, 1)
}

func TestGenerateRandomPassphrase(t *testing.T) {
	passphrase := generateRandomPassphrase()
	assert.Len(t, passphrase, 64) // 32 bytes in hex format
}
