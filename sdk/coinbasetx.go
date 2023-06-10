// Package sdk is a software development kit for building blockchain applications.
// File sdk/coinbasetx.go - The Coinbase transaction
package sdk

import (
	"fmt"
)

// Bank is a transaction that represents a bank transfer.
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

// NewBankTransaction creates a new Bank transaction.
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

// Process processes the bank transaction.
func (c *Coinbase) Process() string {
	err := c.From.SetData("balance", c.TokenCount)
	if err != nil {
		return fmt.Sprintf("Error updating wallet %s balance: %s", c.From.GetAddress(), err.Error())
	}

	return fmt.Sprintf("Transferred %f from %s to %s", c.TransactionFee, c.From.Address, c.To.Address)
}
