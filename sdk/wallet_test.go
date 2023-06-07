// file: sdk/wallet_test.go
package sdk

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testAddr      = "7cd017593398aebb99da3e5e3bb62efad50d9fd925d8d633fbab0c2df12535f8"
	AddressLength = 32
)

func TestAddressLength(t *testing.T) {
	// Decode the test address
	addr, err := hex.DecodeString(testAddr)
	if err != nil {
		t.Fatalf("Failed to decode address: %v", err)
	}

	// Verify the address length
	if len(addr) != AddressLength {
		t.Errorf("Expected address length: %d, got: %d", AddressLength, len(addr))
	}

}

func TestWallet(t *testing.T) {
	// Create two wallets with different data
	wallet1, err := NewWallet("Wallet1", []string{"tag1", "tag2"})
	assert.NoError(t, err)
	assert.NotNil(t, wallet1)

	wallet2, err := NewWallet("Wallet2", []string{"tag3", "tag4"})
	assert.NoError(t, err)
	assert.NotNil(t, wallet2)

	// Test wallet data and properties
	assert.Equal(t, "Wallet1", wallet1.GetWalletName())
	assert.Equal(t, []string{"tag1", "tag2"}, wallet1.GetTags())
	assert.Equal(t, fundWalletAmount, wallet1.GetBalance())

	assert.Equal(t, "Wallet2", wallet2.GetWalletName())
	assert.Equal(t, []string{"tag3", "tag4"}, wallet2.GetTags())
	assert.Equal(t, fundWalletAmount, wallet2.GetBalance())

	// Test wallet address generation
	address1 := wallet1.GetAddress()
	assert.NotEmpty(t, address1)

	address2 := wallet2.GetAddress()
	assert.NotEmpty(t, address2)
	assert.NotEqual(t, address1, address2)

	// Test encryption and decryption

	// Password is to weak
	passphrase := "testpass"
	err = wallet1.Lock(passphrase)
	assert.Error(t, err, "password is too weak")
	assert.False(t, wallet1.Encrypted)

	passphrase = "te$tpaSS2023!"
	err = wallet1.Lock(passphrase)
	assert.NoError(t, err)
	assert.True(t, wallet1.Encrypted)

	err = wallet1.Unlock(passphrase)
	assert.NoError(t, err)
	assert.False(t, wallet1.Encrypted)

	// Test sending a transaction
	bc := NewBlockchain()
	tx, err := NewBankTransaction(wallet1, wallet2, 1.0)
	assert.NoError(t, err)
	err = wallet1.SignTransaction(tx)
	assert.NoError(t, err)

	sentTx, err := wallet1.SendTransaction(wallet2.GetAddress(), tx, bc)
	assert.NoError(t, err)
	assert.NotNil(t, sentTx)
}
