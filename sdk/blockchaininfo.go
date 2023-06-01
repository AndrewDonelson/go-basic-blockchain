package sdk

// BlockchainInfo is used by the API endpoint / to return information about the blockchain
type BlockchainInfo struct {
	Version    string  `json:"version"`
	Name       string  `json:"name"`
	Symbol     string  `json:"symbol"`
	BlockTime  int     `json:"block_time"`
	Difficulty int     `json:"difficulty"`
	Fee        float64 `json:"transaction_fee"`
}
