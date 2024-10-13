// Package sdk is a software development kit for building blockchain applications.
// File sdk/address.go - Address for all Address related Protocol based transactions
//
// This file defines the Address structure and associated methods for handling
// blockchain addresses within the SDK. It provides functionality for creating,
// manipulating, and hashing blockchain addresses.
//
// The Address structure represents a blockchain address with two components:
// 1. RawAddress: The original, unmodified address string.
// 2. PrependedAddress: The address with the blockchain symbol prepended.
//
// Key features and functionalities:
// - Creation of new Address instances with NewAddress function.
// - Retrieval of raw and prepended addresses.
// - Generation of address hashes using SHA-256.
//
// The file implements the following main components:
// 1. Address struct: Represents a blockchain address.
// 2. NewAddress function: Creates a new Address instance.
// 3. GetRawAddress method: Returns the raw address string.
// 4. GetPrependedAddress method: Returns the address with the blockchain symbol prepended.
// 5. prependSymbol function: Helper function to prepend the blockchain symbol to an address.
// 6. GetAddressHash method: Calculates and returns the SHA-256 hash of the prepended address.
//
// This implementation allows for flexible address handling within the blockchain SDK,
// supporting operations such as address creation, modification, and hashing. The use of
// prepended addresses with blockchain symbols enables easy identification and
// categorization of addresses across different blockchain networks.
//
// Usage of this file's components is essential for proper address management
// throughout the blockchain application, ensuring consistency and security in
// address handling and representation
package sdk

import (
	"crypto/sha256"
	"encoding/hex"
)

// Address represents a blockchain address.
// It contains the raw address string and a prepended version of the address.
type Address struct {
	RawAddress       string
	PrependedAddress string
}

// NewAddress creates a new Address with the given raw address and blockchain symbol.
// The raw address is prepended with the blockchain symbol to create the prepended address.
func NewAddress(rawAddress, blockchainSymbol string) *Address {
	prependedAddress := prependSymbol(rawAddress, blockchainSymbol)
	return &Address{
		RawAddress:       rawAddress,
		PrependedAddress: prependedAddress,
	}
}

// GetRawAddress returns the raw address string of the Address.
func (a *Address) GetRawAddress() string {
	return a.RawAddress
}

// GetPrependedAddress returns the address with the blockchain symbol prepended.
// This method returns the full address string that includes the blockchain symbol
// prepended to the raw address.
func (a *Address) GetPrependedAddress() string {
	return a.PrependedAddress
}

// prependSymbol prepends the given symbol to the address.
func prependSymbol(address, symbol string) string {
	return symbol + address
}

// GetAddressHash calculates and returns the SHA-256 hash of the prepended address.
func (a *Address) GetAddressHash() string {
	hash := sha256.Sum256([]byte(a.PrependedAddress))
	return hex.EncodeToString(hash[:])
}
