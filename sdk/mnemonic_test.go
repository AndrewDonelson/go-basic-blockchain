package sdk

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestGenerateMnemonic tests the GenerateMnemonic function by generating a mnemonic
// and verifying that it is not empty and contains 12 words.
func TestGenerateMnemonic(t *testing.T) {
	mnemonic, err := GenerateMnemonic()
	assert.NoError(t, err)
	assert.NotEmpty(t, mnemonic)

	// split string into slice by space
	mnemonicSlice := strings.Split(mnemonic, " ")
	words := len(mnemonicSlice)
	assert.Equal(t, 12, words)
}

// TestDeriveKeyPair tests the DeriveKeyPair function by generating a mnemonic,
// deriving a public and private key from it, and verifying that the keys
// are not empty.
func TestDeriveKeyPair(t *testing.T) {
	mnemonic, _ := GenerateMnemonic()
	publicKey, privateKey, err := DeriveKeyPair(mnemonic, testPassPhrase)
	assert.NoError(t, err)
	assert.NotEmpty(t, publicKey)
	assert.NotEmpty(t, privateKey)

	// Display mnemonic and keys
	fmt.Println("Mnemonic: ", mnemonic)
	fmt.Println("Password: ", testPassPhrase)
	fmt.Println("Master private key: ", privateKey)
	fmt.Println("Master public key: ", publicKey)
}

// TestGenerateWalletAddress tests the GenerateWalletAddress function by generating a mnemonic,
// deriving a public key from it, and verifying that the generated wallet address is not empty.
func TestGenerateWalletAddress(t *testing.T) {
	mnemonic, _ := GenerateMnemonic()
	publicKey, _, _ := DeriveKeyPair(mnemonic, testPassPhrase)
	address, err := GenerateWalletAddress(publicKey)
	assert.NoError(t, err)
	assert.NotEmpty(t, address)
}
