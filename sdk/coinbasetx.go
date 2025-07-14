// Package sdk is a software development kit for building blockchain applications.
// File sdk/coinbasetx.go - The Coinbase transaction
package sdk

import (
	"encoding/json"
	"fmt"
)

// Coinbase represents a coinbase transaction, which is a special type of transaction
// that is used to reward miners for mining a new block. It contains information
// about the block, the miner's reward, and any additional rewards or fees.
type Coinbase struct {
	Tx
	BlockchainName   string
	BlockchainSymbol string
	BlockTime        int
	Difficulty       int
	TransactionFee   float64
	MinerRewardPCT   float64
	MinerAddress     string
	DevRewardPCT     float64
	DevAddress       string
	FundWalletAmount float64
	TokenCount       int64
	TokenPrice       float64
	AllowNewTokens   bool
}

// MarshalJSON implements custom JSON marshaling for Coinbase transaction
func (c *Coinbase) MarshalJSON() ([]byte, error) {
	// First marshal the base Tx
	baseTx, err := json.Marshal(&c.Tx)
	if err != nil {
		return nil, err
	}

	// Create a map to hold the base transaction data
	var baseMap map[string]interface{}
	err = json.Unmarshal(baseTx, &baseMap)
	if err != nil {
		return nil, err
	}

	// Add the coinbase-specific data
	baseMap["blockchainName"] = c.BlockchainName
	baseMap["blockchainSymbol"] = c.BlockchainSymbol
	baseMap["blockTime"] = c.BlockTime
	baseMap["difficulty"] = c.Difficulty
	baseMap["transactionFee"] = c.TransactionFee
	baseMap["minerRewardPCT"] = c.MinerRewardPCT
	baseMap["minerAddress"] = c.MinerAddress
	baseMap["devRewardPCT"] = c.DevRewardPCT
	baseMap["devAddress"] = c.DevAddress
	baseMap["fundWalletAmount"] = c.FundWalletAmount
	baseMap["tokenCount"] = c.TokenCount
	baseMap["tokenPrice"] = c.TokenPrice
	baseMap["allowNewTokens"] = c.AllowNewTokens

	// Serialize the protocol data to the Data field
	protocolData := map[string]interface{}{
		"blockchainName":   c.BlockchainName,
		"blockchainSymbol": c.BlockchainSymbol,
		"blockTime":        c.BlockTime,
		"difficulty":       c.Difficulty,
		"transactionFee":   c.TransactionFee,
		"minerRewardPCT":   c.MinerRewardPCT,
		"minerAddress":     c.MinerAddress,
		"devRewardPCT":     c.DevRewardPCT,
		"devAddress":       c.DevAddress,
		"fundWalletAmount": c.FundWalletAmount,
		"tokenCount":       c.TokenCount,
		"tokenPrice":       c.TokenPrice,
		"allowNewTokens":   c.AllowNewTokens,
	}

	protocolDataBytes, err := json.Marshal(protocolData)
	if err != nil {
		return nil, err
	}
	baseMap["data"] = protocolDataBytes

	return json.Marshal(baseMap)
}

// NewCoinbaseTransaction creates a new coinbase transaction. It takes a from wallet, a to wallet, and a configuration object as input.
// The function returns a new Coinbase transaction and an error if any.
// The Coinbase transaction contains information about the block, the miner's reward, and any additional rewards or fees.
func NewCoinbaseTransaction(from *Wallet, to *Wallet, cfg *Config) (*Coinbase, error) {
	tx, err := NewTransaction(CoinbaseProtocolID, from, to)
	if err != nil {
		return nil, err
	}

	return &Coinbase{
		Tx:               *tx,
		BlockchainName:   cfg.BlockchainName,
		BlockchainSymbol: cfg.BlockchainSymbol,
		BlockTime:        cfg.BlockTime,
		Difficulty:       cfg.Difficulty,
		TransactionFee:   cfg.TransactionFee,
		MinerRewardPCT:   cfg.MinerRewardPCT,
		MinerAddress:     cfg.MinerAddress,
		DevRewardPCT:     cfg.DevRewardPCT,
		DevAddress:       cfg.DevAddress,
		FundWalletAmount: cfg.FundWalletAmount,
		TokenCount:       cfg.TokenCount,
		TokenPrice:       cfg.TokenPrice,
		AllowNewTokens:   cfg.AllowNewTokens,
	}, nil
}

// Process updates the wallet balance with the token count and returns a string
// describing the transfer of the transaction fee.
func (c *Coinbase) Process() string {
	err := c.From.SetData("balance", c.TokenCount)
	if err != nil {
		return fmt.Sprintf("Error updating wallet %s balance: %s", c.From.GetAddress(), err.Error())
	}

	return fmt.Sprintf("Transferred %f from %s to %s", c.TransactionFee, c.From.Address, c.To.Address)
}

// // String returns a string representation of the bank transaction.
// func (c *Coinbase) String() string {
// 	return fmt.Sprintf("%s%s%s%v%d%f%f%s%f%s%f%d%f%t",
// 		c.Tx.String(),
// 		c.BlockchainName,
// 		c.BlockchainSymbol,
// 		c.BlockTime,
// 		c.Difficulty,
// 		c.TransactionFee,
// 		c.MinerRewardPCT,
// 		c.MinerAddress,
// 		c.DevRewardPCT,
// 		c.DevAddress,
// 		c.FundWalletAmount,
// 		c.TokenCount,
// 		c.TokenPrice,
// 		c.AllowNewTokens,
// 	)
// }

// // calculateHash calculates the hash of the block.
// func (c *Coinbase) calculateHash() string {

// 	// Hash the string
// 	c.hash = sha256.Sum256([]byte(c.String()))

// 	// Return the hash as a string
// 	return hex.EncodeToString(hash[:])
// }
