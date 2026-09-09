package sdk

// Lower the scrypt cost for the whole test binary.
//
// This used to live in production code as a testing.Testing() branch inside
// deriveKey, which (a) pulled the "testing" package into the shipped binary and
// (b) was never recorded in the wallet file, so a wallet created under test could
// not be opened in production. The cost profile is now chosen here and persisted
// in the wallet's EncryptionParams.
func init() {
	defaultScryptParams = fastScryptParams
}
