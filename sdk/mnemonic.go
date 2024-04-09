package sdk

import (
	"crypto/sha256"
	"encoding/hex"

	"github.com/tyler-smith/go-bip32"
	"github.com/tyler-smith/go-bip39"
)

// GenerateMnemonic generates a new 12-word mnemonic passphrase.
func GenerateMnemonic() (string, error) {

	// Generate 256-bit entropy
	entropy, err := bip39.NewEntropy(128)
	if err != nil {
		return "", err
	}

	// Generate a mnemonic from the entropy
	mnemonic, err := bip39.NewMnemonic(entropy)
	if err != nil {
		return "", err
	}

	// Return the mnemonic
	return mnemonic, nil
}

// DeriveKeyPair derives a public/private key pair from a given mnemonic passphrase.
func DeriveKeyPair(mnemonic string, password string) (publicKey, privateKey []byte, err error) {
	// Initialize empty slices
	emptyKey := make([]byte, 32)

	// Create a seed from the mnemonic
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, password)
	if err != nil {
		// return empty slices and the error
		return emptyKey, emptyKey, err
	}

	masterKey, err := bip32.NewMasterKey(seed)
	if err != nil {
		return emptyKey, emptyKey, err
	}

	publicKey, _ = masterKey.PublicKey().Serialize()
	privateKey, _ = masterKey.Serialize()

	return publicKey, privateKey, nil
}

// GenerateWalletAddress generates a wallet address from a given public key.
func GenerateWalletAddress(publicKey []byte) (string, error) {
	hash := sha256.Sum256(publicKey)
	address := hex.EncodeToString(hash[:])

	return address, nil
}
