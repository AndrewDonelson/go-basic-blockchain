// Package sdk is a software development kit for building blockchain applications.
package sdk

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewAddress(t *testing.T) {
	t.Run("normal address creation", func(t *testing.T) {
		rawAddress := "0x123"
		blockchainSymbol := "ETH-"
		expectedPrependedAddress := "ETH-0x123"

		address := NewAddress(rawAddress, blockchainSymbol)

		assert.Equal(t, rawAddress, address.RawAddress, "Raw address should match input")
		assert.Equal(t, expectedPrependedAddress, address.PrependedAddress, "Prepended address should include symbol")
	})

	t.Run("with empty raw address", func(t *testing.T) {
		rawAddress := ""
		blockchainSymbol := "BTC-"
		expectedPrependedAddress := "BTC-"

		address := NewAddress(rawAddress, blockchainSymbol)

		assert.Equal(t, rawAddress, address.RawAddress, "Raw address should be empty")
		assert.Equal(t, expectedPrependedAddress, address.PrependedAddress, "Prepended address should only contain symbol")
	})

	t.Run("with empty blockchain symbol", func(t *testing.T) {
		rawAddress := "0x456"
		blockchainSymbol := ""
		expectedPrependedAddress := "0x456" // Empty prefix

		address := NewAddress(rawAddress, blockchainSymbol)

		assert.Equal(t, rawAddress, address.RawAddress, "Raw address should match input")
		assert.Equal(t, expectedPrependedAddress, address.PrependedAddress, "Prepended address should match raw address with empty prefix")
	})

	t.Run("with special characters", func(t *testing.T) {
		rawAddress := "0x789!@#$%^&*()"
		blockchainSymbol := "TEST:"
		expectedPrependedAddress := "TEST:0x789!@#$%^&*()"

		address := NewAddress(rawAddress, blockchainSymbol)

		assert.Equal(t, rawAddress, address.RawAddress, "Raw address should match input with special chars")
		assert.Equal(t, expectedPrependedAddress, address.PrependedAddress, "Prepended address should include symbol and special chars")
	})
}

func TestGetRawAddress(t *testing.T) {
	t.Run("normal raw address", func(t *testing.T) {
		address := &Address{RawAddress: "0x456"}
		rawAddress := address.GetRawAddress()
		assert.Equal(t, "0x456", rawAddress, "GetRawAddress should return the raw address")
	})

	t.Run("empty raw address", func(t *testing.T) {
		address := &Address{RawAddress: ""}
		rawAddress := address.GetRawAddress()
		assert.Equal(t, "", rawAddress, "GetRawAddress should return empty string for empty raw address")
	})

	t.Run("special characters in raw address", func(t *testing.T) {
		specialAddress := "0x123!@#$%^&*()"
		address := &Address{RawAddress: specialAddress}
		rawAddress := address.GetRawAddress()
		assert.Equal(t, specialAddress, rawAddress, "GetRawAddress should handle special characters")
	})
}

func TestGetPrependedAddress(t *testing.T) {
	t.Run("normal prepended address", func(t *testing.T) {
		address := &Address{PrependedAddress: "BTC-0x789"}
		prependedAddress := address.GetPrependedAddress()
		assert.Equal(t, "BTC-0x789", prependedAddress, "GetPrependedAddress should return the prepended address")
	})

	t.Run("empty prepended address", func(t *testing.T) {
		address := &Address{PrependedAddress: ""}
		prependedAddress := address.GetPrependedAddress()
		assert.Equal(t, "", prependedAddress, "GetPrependedAddress should return empty string for empty prepended address")
	})

	t.Run("special characters in prepended address", func(t *testing.T) {
		specialAddress := "TEST-0x123!@#$%^&*()"
		address := &Address{PrependedAddress: specialAddress}
		prependedAddress := address.GetPrependedAddress()
		assert.Equal(t, specialAddress, prependedAddress, "GetPrependedAddress should handle special characters")
	})
}

func TestPrependSymbol(t *testing.T) {
	testCases := []struct {
		name     string
		address  string
		symbol   string
		expected string
	}{
		{
			name:     "normal case",
			address:  "0xABC",
			symbol:   "LTC-",
			expected: "LTC-0xABC",
		},
		{
			name:     "empty address",
			address:  "",
			symbol:   "BTC-",
			expected: "BTC-",
		},
		{
			name:     "empty symbol",
			address:  "0xDEF",
			symbol:   "",
			expected: "0xDEF",
		},
		{
			name:     "both empty",
			address:  "",
			symbol:   "",
			expected: "",
		},
		{
			name:     "special characters",
			address:  "0x123!@#$%^&*()",
			symbol:   "TEST:",
			expected: "TEST:0x123!@#$%^&*()",
		},
		{
			name:     "unicode characters",
			address:  "0x寿司",
			symbol:   "币-",
			expected: "币-0x寿司",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := prependSymbol(tc.address, tc.symbol)
			assert.Equal(t, tc.expected, result)
		})
	}
}

func TestGetAddressHash(t *testing.T) {
	t.Run("normal prepended address hash", func(t *testing.T) {
		address := &Address{PrependedAddress: "ETH-0xDEF"}
		expectedHash := sha256.Sum256([]byte("ETH-0xDEF"))
		expectedHashString := hex.EncodeToString(expectedHash[:])

		hash := address.GetAddressHash()

		assert.Equal(t, expectedHashString, hash, "Hash should match expected SHA-256 value")
	})

	t.Run("empty prepended address hash", func(t *testing.T) {
		address := &Address{PrependedAddress: ""}
		expectedHash := sha256.Sum256([]byte(""))
		expectedHashString := hex.EncodeToString(expectedHash[:])

		hash := address.GetAddressHash()

		assert.Equal(t, expectedHashString, hash, "Hash should handle empty prepended address")
	})

	t.Run("special characters in hash", func(t *testing.T) {
		specialAddress := "TEST-0x123!@#$%^&*()"
		address := &Address{PrependedAddress: specialAddress}
		expectedHash := sha256.Sum256([]byte(specialAddress))
		expectedHashString := hex.EncodeToString(expectedHash[:])

		hash := address.GetAddressHash()

		assert.Equal(t, expectedHashString, hash, "Hash should handle special characters")
	})

	t.Run("unicode characters in hash", func(t *testing.T) {
		unicodeAddress := "币-0x寿司"
		address := &Address{PrependedAddress: unicodeAddress}
		expectedHash := sha256.Sum256([]byte(unicodeAddress))
		expectedHashString := hex.EncodeToString(expectedHash[:])

		hash := address.GetAddressHash()

		assert.Equal(t, expectedHashString, hash, "Hash should handle unicode characters")
	})

	t.Run("hash determinism", func(t *testing.T) {
		// Create two addresses with the same prepended value
		address1 := &Address{PrependedAddress: "BTC-0x789"}
		address2 := &Address{PrependedAddress: "BTC-0x789"}

		hash1 := address1.GetAddressHash()
		hash2 := address2.GetAddressHash()

		assert.Equal(t, hash1, hash2, "Same address should produce the same hash")
	})

	t.Run("hash format", func(t *testing.T) {
		address := &Address{PrependedAddress: "ETH-0xABC"}
		hash := address.GetAddressHash()

		// Check that the hash is a valid hex string
		_, err := hex.DecodeString(hash)
		require.NoError(t, err, "Hash should be a valid hex string")

		// Check that the hash is the correct length for SHA-256 (64 hex chars)
		assert.Equal(t, 64, len(hash), "SHA-256 hash should be 64 hex characters long")

		// Check that the hash contains only hex characters
		validHexChars := true
		for _, r := range hash {
			if !strings.ContainsRune("0123456789abcdef", r) {
				validHexChars = false
				break
			}
		}
		assert.True(t, validHexChars, "Hash should only contain hex characters")
	})
}

func TestAddressIntegration(t *testing.T) {
	// Test the full flow: create address -> get methods -> hash
	rawAddress := "0x987654321"
	blockchainSymbol := "CHAIN-"

	// Create a new address
	address := NewAddress(rawAddress, blockchainSymbol)

	// Verify the original values are stored
	assert.Equal(t, rawAddress, address.GetRawAddress(), "GetRawAddress should return original raw address")
	assert.Equal(t, blockchainSymbol+rawAddress, address.GetPrependedAddress(), "GetPrependedAddress should return symbol+address")

	// Generate and verify the hash
	hash := address.GetAddressHash()
	expectedHash := sha256.Sum256([]byte(blockchainSymbol + rawAddress))
	expectedHashString := hex.EncodeToString(expectedHash[:])
	assert.Equal(t, expectedHashString, hash, "GetAddressHash should return correct hash of prepended address")

	// Ensure hash is 32 bytes (64 hex chars)
	assert.Equal(t, 64, len(hash), "Hash should be 64 characters long (32 bytes)")
}

// Benchmark performance of address operations
func BenchmarkNewAddress(b *testing.B) {
	rawAddress := "0x123456789abcdef"
	symbol := "ETH-"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		NewAddress(rawAddress, symbol)
	}
}

func BenchmarkGetAddressHash(b *testing.B) {
	address := NewAddress("0x123456789abcdef", "ETH-")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		address.GetAddressHash()
	}
}
