// Package sdk is a software development kit for building blockchain applications.
// File sdk/persisttx.go - Persist Transaction for all Persistence related Protocol based transactions
package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPersistTransaction(t *testing.T) {
	from, err := NewWallet("walletEven", testPassPhrase, []string{"tag2", "tag4"})
	assert.NoError(t, err)

	to, err := NewWallet("walletOdd", testPassPhrase, []string{"tag1", "tag3"})
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

	// Create a new test chain
	bc := NewBlockchain(NewConfig())

	// sign the transaction
	err = from.SignTransaction(persist)
	assert.NoError(t, err)

	err = persist.Send(bc)
	assert.NoError(t, err)
}
