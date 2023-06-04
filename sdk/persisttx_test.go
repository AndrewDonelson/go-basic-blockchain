package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPersistTransaction(t *testing.T) {
	from, err := NewWallet("walletEven", []string{"tag2", "tag4"})
	assert.NoError(t, err)

	to, err := NewWallet("walletOdd", []string{"tag1", "tag3"})
	assert.NoError(t, err)

	fee := transactionFee
	data := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	persist, err := NewPersistTransaction(from, to, fee, data)
	assert.NoError(t, err)
	assert.NotNil(t, persist)

	assert.Equal(t, PersistProtocolID, persist.Protocol)
	assert.Equal(t, from, persist.From)
	assert.Equal(t, to, persist.To)
	assert.Equal(t, fee, persist.Fee)
	//assert.Equal(t, "pending", persist.Status)
	assert.Equal(t, data, persist.Data)

	// Process the transaction
	result := persist.Process()
	assert.Equal(t, "Persist transaction processed successfully", result)
	assert.Equal(t, "processed", persist.Status)

	// Verify the signature
	err = persist.Verify([]byte("signature"))
	assert.NoError(t, err)

	// Create a new test chain
	bc := NewBlockchain()

	err = persist.Send(bc)
	assert.NoError(t, err)

	// Sign the transaction
	err = persist.Sign([]byte("signature"))
	assert.NoError(t, err)
}
