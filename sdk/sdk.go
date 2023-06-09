// Package sdk is a software development kit for building blockchain applications.
package sdk

// Version returns the version of the SDK
func Version() string {
	return BlockchainVersion
}

// Name returns the name of the SDK
func Name() string {
	return BlockchainName
}
