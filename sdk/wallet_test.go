package sdk

import (
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"testing"
)

func TestWallet(t *testing.T) {
	name := "Alice"
	tags := []string{"personal", "crypto"}

	// Create a new wallet
	wallet, err := NewWallet(name, tags)
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}

	// Verify wallet properties
	if wallet.Name != name {
		t.Errorf("Expected wallet name: %s, got: %s", name, wallet.Name)
	}

	if len(wallet.Tags) != len(tags) {
		t.Errorf("Expected wallet tags length: %d, got: %d", len(tags), len(wallet.Tags))
	}

	for i, tag := range tags {
		if wallet.Tags[i] != tag {
			t.Errorf("Expected wallet tag: %s, got: %s", tag, wallet.Tags[i])
		}
	}

	// Generate a new public key from the wallet's private key
	block, _ := pem.Decode(wallet.PrivateKey)
	if block == nil {
		t.Fatal("Failed to parse PEM block containing the private key")
	}

	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		t.Fatalf("Failed to parse private key: %v", err)
	}

	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to marshal public key: %v", err)
	}

	// Verify wallet address generation
	expectedAddress := hex.EncodeToString(publicKeyBytes[:])

	if wallet.Address != expectedAddress {
		t.Errorf("Expected wallet address: %s, got: %s", expectedAddress, wallet.Address)
	}

	// Verify encryption and decryption of the private key
	passphrase := "secretpassphrase"

	// Encrypt the private key
	err = wallet.EncryptPrivateKey(passphrase)
	if err != nil {
		t.Fatalf("Failed to encrypt private key: %v", err)
	}

	if !wallet.Encrypted {
		t.Error("Expected wallet private key to be encrypted")
	}

	// Decrypt the private key
	err = wallet.DecryptPrivateKey(passphrase)
	if err != nil {
		t.Fatalf("Failed to decrypt private key: %v", err)
	}

	if wallet.Encrypted {
		t.Error("Expected wallet private key to be decrypted")
	}

	// Verify signature verification
	tx := &MockTransaction{
		ID:       "12345",
		Protocol: "mock",
	}

	// Sign the transaction
	err = tx.Sign([]byte(wallet.PrivateKey))
	if err != nil {
		t.Fatalf("Failed to sign transaction: %v", err)
	}

	// Verify the signature
	err = tx.Verify(tx.GetSignature())
	if err != nil {
		t.Errorf("Failed to verify signature: %v", err)
	}
}

// MockTransaction is a mock implementation of the Transaction interface.
type MockTransaction struct {
	ID        string
	Protocol  string
	Signature []byte
}

func (t *MockTransaction) GetProtocol() string {
	return t.Protocol
}

func (t *MockTransaction) Process() string {
	return "MockTransaction process"
}

func (t *MockTransaction) Verify(signature []byte) error {
	return nil
}

func (t *MockTransaction) Send() error {
	return nil
}

func (t *MockTransaction) Sign(signature []byte) error {
	t.Signature = signature
	return nil
}

func (t *MockTransaction) GetSignature() []byte {
	return t.Signature
}
