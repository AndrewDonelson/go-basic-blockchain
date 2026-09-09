// Package sdk is a software development kit for building blockchain applications.
// File sdk/coinbasetx.go - The Coinbase transaction
package sdk

import (
	"encoding/json"
	"errors"
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

// MarshalJSON encodes the Coinbase transaction in the canonical wire form.
func (c *Coinbase) MarshalJSON() ([]byte, error) {
	w := c.Tx.toWire()
	w.BlockchainName = c.BlockchainName
	w.BlockchainSymbol = c.BlockchainSymbol
	w.BlockTime = c.BlockTime
	w.Difficulty = c.Difficulty
	w.TransactionFee = c.TransactionFee
	w.MinerRewardPCT = c.MinerRewardPCT
	w.MinerAddress = c.MinerAddress
	w.DevRewardPCT = c.DevRewardPCT
	w.DevAddress = c.DevAddress
	w.FundWalletAmount = c.FundWalletAmount
	w.TokenCount = c.TokenCount
	w.TokenPrice = c.TokenPrice
	w.AllowNewTokens = c.AllowNewTokens
	return json.Marshal(w)
}

// UnmarshalJSON decodes a Coinbase transaction from the canonical wire form.
func (c *Coinbase) UnmarshalJSON(data []byte) error {
	var w txWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	c.Tx.applyWire(w)
	c.BlockchainName = w.BlockchainName
	c.BlockchainSymbol = w.BlockchainSymbol
	c.BlockTime = w.BlockTime
	c.Difficulty = w.Difficulty
	c.TransactionFee = w.TransactionFee
	c.MinerRewardPCT = w.MinerRewardPCT
	c.MinerAddress = w.MinerAddress
	c.DevRewardPCT = w.DevRewardPCT
	c.DevAddress = w.DevAddress
	c.FundWalletAmount = w.FundWalletAmount
	c.TokenCount = w.TokenCount
	c.TokenPrice = w.TokenPrice
	c.AllowNewTokens = w.AllowNewTokens
	return nil
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
	if c.To == nil {
		return "coinbase transaction has no recipient"
	}
	// Previously this *set* the balance to TokenCount on every call, minting the
	// entire supply again each time. A coinbase credits its recipient exactly once,
	// which the genesis path enforces by only ever processing it in block 0.
	if err := c.To.SetData("balance", c.To.GetBalance()+float64(c.TokenCount)); err != nil {
		return fmt.Sprintf("Error updating wallet %s balance: %s", c.To.GetAddress(), err.Error())
	}
	c.Status = StatusConfirmed
	return fmt.Sprintf("Minted %d tokens to %s", c.TokenCount, c.To.Address)
}

// SigningBytes includes the chain parameters a coinbase commits to.
func (c *Coinbase) SigningBytes() ([]byte, error) {
	fields := c.Tx.signingFields()
	fields["token_count"] = c.TokenCount
	fields["token_price"] = c.TokenPrice
	fields["miner_address"] = c.MinerAddress
	fields["dev_address"] = c.DevAddress
	fields["miner_reward_pct"] = c.MinerRewardPCT
	fields["dev_reward_pct"] = c.DevRewardPCT
	fields["blockchain_name"] = c.BlockchainName
	fields["blockchain_symbol"] = c.BlockchainSymbol
	fields["difficulty"] = c.Difficulty
	fields["block_time"] = c.BlockTime
	fields["allow_new_tokens"] = c.AllowNewTokens
	return json.Marshal(fields)
}

// Sign signs the full Coinbase transaction, including its chain parameters.
func (c *Coinbase) Sign(privPEM []byte) (string, error) {
	payload, err := c.SigningBytes()
	if err != nil {
		return "", fmt.Errorf("error marshaling transaction: %w", err)
	}
	return signPayload(payload, privPEM)
}

// Verify verifies a signature over the full Coinbase transaction.
func (c *Coinbase) Verify(pubKey []byte, sign string) (bool, error) {
	payload, err := c.SigningBytes()
	if err != nil {
		return false, fmt.Errorf("error marshaling transaction: %w", err)
	}
	return verifyPayload(payload, pubKey, sign)
}

// Hash covers the chain parameters as well as the base fields.
func (c *Coinbase) Hash() string {
	c.Tx.hash = hashTransaction(c)
	return c.Tx.hash
}

// Bytes returns the canonical encoding of the full transaction.
func (c *Coinbase) Bytes() []byte {
	payload, err := c.SigningBytes()
	if err != nil {
		return nil
	}
	return payload
}

// Size reports the size of the full transaction.
func (c *Coinbase) Size() int { return len(c.Bytes()) }

// EstimateFee is derived from the full transaction size.
func (c *Coinbase) EstimateFee(feePerByte float64) float64 {
	return float64(c.Size()) * feePerByte
}

// Send queues the Coinbase transaction itself rather than its base transaction.
func (c *Coinbase) Send(bc *Blockchain) error {
	if err := c.Validate(); err != nil {
		return fmt.Errorf("invalid transaction: %w", err)
	}
	bc.AddTransaction(c)
	return nil
}

// Validate checks the base transaction plus Coinbase-specific invariants.
func (c *Coinbase) Validate() error {
	if err := c.Tx.Validate(); err != nil {
		return err
	}
	if c.TokenCount < 0 {
		return errors.New("coinbase token count cannot be negative")
	}
	if c.Tx.Protocol != CoinbaseProtocolID {
		return fmt.Errorf("coinbase transaction has wrong protocol: %s", c.Tx.Protocol)
	}
	return nil
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
