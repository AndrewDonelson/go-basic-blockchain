// file: sdk/address_test.gogo build
package sdk

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewAddress(t *testing.T) {
	rawAddress := "0x123"
	blockchainSymbol := "ETH-"
	expectedPrependedAddress := "ETH-0x123"

	address := NewAddress(rawAddress, blockchainSymbol)

	assert.Equal(t, rawAddress, address.RawAddress)
	assert.Equal(t, expectedPrependedAddress, address.PrependedAddress)
}

func TestGetRawAddress(t *testing.T) {
	address := &Address{RawAddress: "0x456"}

	rawAddress := address.GetRawAddress()

	assert.Equal(t, "0x456", rawAddress)
}

func TestGetPrependedAddress(t *testing.T) {
	address := &Address{PrependedAddress: "BTC-0x789"}

	prependedAddress := address.GetPrependedAddress()

	assert.Equal(t, "BTC-0x789", prependedAddress)
}

func TestPrependSymbol(t *testing.T) {
	address := "0xABC"
	symbol := "LTC-"
	expectedResult := "LTC-0xABC"

	result := prependSymbol(address, symbol)

	assert.Equal(t, expectedResult, result)
}

func TestGetAddressHash(t *testing.T) {
	address := &Address{PrependedAddress: "ETH-0xDEF"}
	expectedHash := sha256.Sum256([]byte("ETH-0xDEF"))
	expectedHashString := hex.EncodeToString(expectedHash[:])

	hash := address.GetAddressHash()

	assert.Equal(t, expectedHashString, hash)
}
