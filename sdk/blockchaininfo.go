// Package sdk is a software development kit for building blockchain applications.
// File sdk/blockchaininfo.go - Blockchain Info for all Blockchain related Protocol based transactions
package sdk

// BlockchainInfo is used by the API endpoint / to return information about the blockchain
type BlockchainInfo struct {
	Version    string  `json:"version,omitempty"`
	Name       string  `json:"name,omitempty"`
	Symbol     string  `json:"symbol,omitempty"`
	BlockTime  int     `json:"block_time,omitempty"`
	Difficulty int     `json:"difficulty,omitempty"`
	Fee        float64 `json:"transaction_fee,omitempty"`
}
