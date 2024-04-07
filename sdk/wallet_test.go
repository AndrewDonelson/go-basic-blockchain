package sdk

import (
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	testAddr       = "7cd017593398aebb99da3e5e3bb62efad50d9fd925d8d633fbab0c2df12535f8"
	testPassPhrase = "te$tpaSS2023!"
	AddressLength  = 32
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

func TestValidateAddress(t *testing.T) {
	// Test valid address
	err := ValidateAddress(testAddr)
	assert.NoError(t, err)

	// Test invalid address
	err = ValidateAddress("invalid")
	assert.Error(t, err)
}

func TestPasswordStrength(t *testing.T) {
	// Test valid password
	err := testPasswordStrength(testPassPhrase)
	assert.NoError(t, err)

	// Test invalid password
	err = testPasswordStrength("invalid")
	assert.Error(t, err)
}

func TestCreateWallet(t *testing.T) {
	// Create a new wallet
	wallet, err := NewWallet(NewWalletOptions(ThisBlockchainOrganizationID, ThisBlockchainAppID, ThisBlockchainAdminUserID, ThisBlockchainDevAssetID, "TestWallet", testPassPhrase, []string{"tag1", "tag2"}))
	assert.NoError(t, err)
	assert.NotNil(t, wallet)

	// Test wallet data and properties
	assert.Equal(t, "TestWallet", wallet.GetWalletName())
	assert.Equal(t, []string{"tag1", "tag2"}, wallet.GetTags())
	assert.Equal(t, fundWalletAmount, wallet.GetBalance())
}

// TestOpneCloseWallet test the open and close wallet functions including the locking and unlocking of the wallet
func TestOpenCloseWallet(t *testing.T) {
	// Create a new wallet
	wallet, err := NewWallet(NewWalletOptions(ThisBlockchainOrganizationID, ThisBlockchainAppID, ThisBlockchainAdminUserID, ThisBlockchainDevAssetID, "TestWallet", testPassPhrase, []string{"tag1", "tag2"}))
	assert.NoError(t, err)
	assert.NotNil(t, wallet)

	// Test wallet data and properties
	assert.Equal(t, "TestWallet", wallet.GetWalletName())
	assert.Equal(t, []string{"tag1", "tag2"}, wallet.GetTags())
	assert.Equal(t, fundWalletAmount, wallet.GetBalance())

	// Test wallet address generation
	address := wallet.GetAddress()
	assert.NotEmpty(t, address)

	// Test wallet closing
	err = wallet.Close(testPassPhrase)
	assert.NoError(t, err)

	assert.Equal(t, true, wallet.Encrypted)
	assert.NotEmpty(t, wallet.Ciphertext)

	// Test wallet opening
	err = wallet.Open(testPassPhrase)
	assert.NoError(t, err)
	assert.NotNil(t, wallet)
}

func TestWalletListCount(t *testing.T) {
	count, err := LocalWalletCount()
	assert.NoError(t, err)
	if count != 0 {
		LocalWalletList()
	}
}

func TestWallet(t *testing.T) {
	// Create two wallets with different data
	wallet1, err := NewWallet(NewWalletOptions(ThisBlockchainOrganizationID, ThisBlockchainAppID, ThisBlockchainAdminUserID, ThisBlockchainDevAssetID, "Wallet1", testPassPhrase, []string{"tag1", "tag2"}))

	assert.NoError(t, err)
	assert.NotNil(t, wallet1)

	wallet2, err := NewWallet(NewWalletOptions(ThisBlockchainOrganizationID, ThisBlockchainAppID, ThisBlockchainAdminUserID, ThisBlockchainDevAssetID, "Wallet2", testPassPhrase, []string{"tag3", "tag4"}))
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

	err = wallet1.Lock(testPassPhrase)
	assert.NoError(t, err)
	assert.True(t, wallet1.Encrypted)

	err = wallet1.Unlock(testPassPhrase)
	assert.NoError(t, err)
	assert.False(t, wallet1.Encrypted)

	// Test sending a transaction
	bc := NewBlockchain(NewConfig())
	tx, err := NewBankTransaction(wallet1, wallet2, 1.0)
	assert.NoError(t, err)

	tx.Signature, err = tx.Sign([]byte(wallet1.PrivatePEM()))
	assert.NoError(t, err)

	sentTx, err := wallet1.SendTransaction(wallet2.GetAddress(), tx, bc)
	assert.NoError(t, err)
	assert.NotNil(t, sentTx)
}
