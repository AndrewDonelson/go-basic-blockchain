// Package sdk is a software development kit for building blockchain applications.
// File sdk/address.go - Address for all Address related Protocol based transactions
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
func (a *Address) GetPrependedAddress() string {
	return a.PrependedAddress
}

// prependSymbol prepends the given symbol to the address.
func prependSymbol(address, symbol string) string {
	return symbol + address
}

// GetAddressHash calculates and returns the SHA-256 hash of the address.
func (a *Address) GetAddressHash() string {
	hash := sha256.Sum256([]byte(a.PrependedAddress))
	return hex.EncodeToString(hash[:])
}
