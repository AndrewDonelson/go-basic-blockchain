// Package sdk is a software development kit for building blockchain applications.
// File sdk/blockchaininfo.go - Blockchain Info for all Blockchain related Protocol based transactions
package sdk

// BlockchainInfo represents information about a blockchain, including its version, name, symbol,
// block time, difficulty, and transaction fee.
type BlockchainInfo struct {
	Version    string  `json:"version,omitempty"`
	Name       string  `json:"name,omitempty"`
	Symbol     string  `json:"symbol,omitempty"`
	BlockTime  int     `json:"block_time,omitempty"`
	Difficulty int     `json:"difficulty,omitempty"`
	Fee        float64 `json:"transaction_fee,omitempty"`
}
