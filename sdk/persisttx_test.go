// Package sdk is a software development kit for building blockchain applications.
// File sdk/persisttx.go - Persist Transaction for all Persistence related Protocol based transactions
package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPersistTransaction(t *testing.T) {
	walletEven, err := NewWallet(NewWalletOptions(ThisBlockchainOrganizationID, ThisBlockchainAppID, ThisBlockchainAdminUserID, ThisBlockchainDevAssetID, "walletEven", testPassPhrase, []string{"tag2", "tag4"}))
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}
	err = walletEven.Open(testPassPhrase)
	if err != nil {
		t.Fatalf("Failed to open wallet: %v", err)
	}

	walletOdd, err := NewWallet(NewWalletOptions(ThisBlockchainOrganizationID, ThisBlockchainAppID, ThisBlockchainAdminUserID, ThisBlockchainDevAssetID, "walletOdd", testPassPhrase, []string{"tag1", "tag3"}))
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}
	err = walletOdd.Open(testPassPhrase)
	if err != nil {
		t.Fatalf("Failed to open wallet: %v", err)
	}

	fee := transactionFee
	data := map[string]string{
		"key1": "value1",
		"key2": "value2",
	}

	persist, err := NewPersistTransaction(walletEven, walletOdd, fee, data)
	assert.NoError(t, err)
	assert.NotNil(t, persist)

	assert.Equal(t, PersistProtocolID, persist.Protocol)
	assert.Equal(t, walletEven, persist.From)
	assert.Equal(t, walletOdd, persist.To)
	assert.Equal(t, fee, persist.Fee)
	//assert.Equal(t, "pending", persist.Status)
	assert.Equal(t, data, persist.Data)

	// Process the transaction
	result := persist.Process()
	assert.Equal(t, "Persist transaction processed successfully", result)
	assert.Equal(t, TransactionStatus("processed"), persist.Status)

	// Create a new test chain with isolated data path
	config := NewConfig()
	config.DataPath = "./test_data_persist"
	bc := NewBlockchain(config)

	// sign the transaction
	persist.Signature, err = persist.Sign([]byte(walletEven.PrivatePEM()))
	assert.NoError(t, err)

	err = persist.Send(bc)
	assert.NoError(t, err)
}
