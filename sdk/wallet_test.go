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

// func TestWalletEncryption(t *testing.T) {
// 	passphrase := "mysecretpassphrase"

// 	// Create a new wallet.
// 	wallet, err := NewWallet("John Doe", []string{"tag1", "tag2"})
// 	if err != nil {
// 		t.Fatalf("failed to create wallet: %v", err)
// 	}

// 	// Encrypt the private key.
// 	err = wallet.EncryptPrivateKey(passphrase)
// 	if err != nil {
// 		t.Fatalf("failed to encrypt private key: %v", err)
// 	}

// 	// Check if the private key is encrypted.
// 	if !wallet.Encrypted {
// 		t.Error("expected private key to be encrypted, but it is not")
// 	}

// 	// Decrypt the private key.
// 	err = wallet.DecryptPrivateKey(passphrase)
// 	if err != nil {
// 		t.Fatalf("failed to decrypt private key: %v", err)
// 	}

// 	// Check if the private key is decrypted.
// 	if wallet.Encrypted {
// 		t.Error("expected private key to be decrypted, but it is encrypted")
// 	}
// }

// func TestWalletEncryptionWrongPassphrase(t *testing.T) {
// 	passphrase := "mysecretpassphrase"
// 	wrongPassphrase := "wrongpassphrase"

// 	// Create a new wallet.
// 	wallet, err := NewWallet("John Doe", []string{"tag1", "tag2"})
// 	if err != nil {
// 		t.Fatalf("failed to create wallet: %v", err)
// 	}

// 	// Encrypt the private key.
// 	err = wallet.EncryptPrivateKey(passphrase)
// 	if err != nil {
// 		t.Fatalf("failed to encrypt private key: %v", err)
// 	}

// 	// Check if the private key is encrypted.
// 	if !wallet.Encrypted {
// 		t.Error("expected private key to be encrypted, but it is not")
// 	}

// 	// Attempt to decrypt the private key with the wrong passphrase.
// 	err = wallet.DecryptPrivateKey(wrongPassphrase)
// 	if err == nil {
// 		t.Error("expected decryption to fail with wrong passphrase, but it succeeded")
// 	}

// 	// Check if the private key is still encrypted.
// 	if !wallet.Encrypted {
// 		t.Error("expected private key to remain encrypted, but it is decrypted")
// 	}
// }

// func TestWalletInteraction(t *testing.T) {
// 	// Create two wallets for testing.
// 	wallet1, err := NewWallet("Wallet1", nil)
// 	if err != nil {
// 		t.Fatalf("failed to create Wallet1: %v", err)
// 	}

// 	wallet2, err := NewWallet("Wallet2", nil)
// 	if err != nil {
// 		t.Fatalf("failed to create Wallet2: %v", err)
// 	}

// 	// Print the initial wallet balances.
// 	fmt.Printf("Initial Balances:\nWallet1 Balance: %.2f\nWallet2 Balance: %.2f\n", wallet1.Balance, wallet2.Balance)

// 	// Wallet1 sends a transaction to Wallet2.
// 	amount := 10.0
// 	transaction, err := NewBankTransaction(wallet1, wallet2, amount)
// 	sentTX, err := wallet1.SendTransaction(wallet2.Address, transaction, nil)
// 	if err != nil {
// 		t.Fatalf("failed to send transaction: %v", err)
// 	}
// 	fmt.Println("Sent Transaction: ", PrettyPrint(sentTX))

// 	// // Verify the transaction signature.
// 	// if !VerifySignature(sendTX) {
// 	// 	t.Fatalf("failed to verify transaction signature: %v", err)
// 	// }

// 	// Print the updated wallet balances.
// 	fmt.Printf("Updated Balances:\nWallet1 Balance: %.2f\nWallet2 Balance: %.2f\n", wallet1.Balance, wallet2.Balance)
// }

// func TestWalletEncryptionUnencryptedPrivateKey(t *testing.T) {
// 	passphrase := "mysecretpassphrase"

// 	// Create a new wallet.
// 	wallet, err := NewWallet("John Doe", []string{"tag1", "tag2"})
// 	if err != nil {
// 		t.Fatalf("failed to create wallet: %v", err)
// 	}

// 	// Check if the private key is unencrypted.
// 	if wallet.Encrypted {
// 		t.Error("expected private key to be unencrypted, but it is encrypted")
// 	}

// 	// Attempt to encrypt the already unencrypted private key.
// 	err = wallet.EncryptPrivateKey(passphrase)
// 	if err != nil {
// 		t.Fatalf("failed to encrypt private key: %v", err)
// 	}

// 	// Check if the private key is still unencrypted.
// 	if wallet.Encrypted {
// 		t.Error("expected private key to remain unencrypted, but it is encrypted")
// 	}
// }

// func TestWalletEncryptionInvalidPassphrase(t *testing.T) {
// 	passphrase := "mysecretpassphrase"

// 	// Create a new wallet.
// 	wallet, err := NewWallet("John Doe", []string{"tag1", "tag2"})
// 	if err != nil {
// 		t.Fatalf("failed to create wallet: %v", err)
// 	}

// 	// Encrypt the private key.
// 	err = wallet.EncryptPrivateKey(passphrase)
// 	if err != nil {
// 		t.Fatalf("failed to encrypt private key: %v", err)
// 	}

// 	// Check if the private key is encrypted.
// 	if !wallet.Encrypted {
// 		t.Error("expected private key to be encrypted, but it is not")
// 	}

// 	// Attempt to decrypt the private key with an empty passphrase.
// 	err = wallet.DecryptPrivateKey("")
// 	if err == nil {
// 		t.Error("expected decryption to fail with empty passphrase, but it succeeded")
// 	}

// 	// Check if the private key is still encrypted.
// 	if !wallet.Encrypted {
// 		t.Error("expected private key to remain encrypted, but it is decrypted")
// 	}
// }

// // MockTransaction is a mock implementation of the Transaction interface.
// type MockTransaction struct {
// 	ID        string
// 	Protocol  string
// 	Signature []byte
// }

// func (t *MockTransaction) GetProtocol() string {
// 	return t.Protocol
// }

// func (t *MockTransaction) Process() string {
// 	return "MockTransaction process"
// }

// func (t *MockTransaction) Verify(signature []byte) error {
// 	return nil
// }

// func (t *MockTransaction) Send() error {
// 	return nil
// }

// func (t *MockTransaction) Sign(signature []byte) error {
// 	t.Signature = signature
// 	return nil
// }

// func (t *MockTransaction) GetSignature() []byte {
// 	return t.Signature
// }
