// Package sdk is a software development kit for building blockchain applications.
// File sdk/txcodec.go - canonical wire encoding for the Transaction interface
package sdk

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// txWire is the single on-disk / on-the-wire shape for every transaction type.
//
// Transaction is an interface, so encoding/json cannot decode into it on its own:
// unmarshalling a Block previously failed with "cannot unmarshal object into Go
// struct field Block.transactions of type sdk.Transaction", and LoadExistingBlocks
// swallowed that error -- so the node silently discarded its entire history on
// every restart. Protocol acts as the discriminator that makes decoding possible.
type txWire struct {
	ID        *PUID             `json:"id"`
	Time      time.Time         `json:"time"`
	Version   int               `json:"version"`
	Protocol  string            `json:"protocol"`
	From      string            `json:"from"`
	To        string            `json:"to"`
	FromPub   string            `json:"from_public_key,omitempty"`
	ToPub     string            `json:"to_public_key,omitempty"`
	Fee       float64           `json:"fee"`
	Status    TransactionStatus `json:"status"`
	BlockNum  int               `json:"block_num"`
	Signature string            `json:"signature"`
	Nonce     uint64            `json:"nonce"`
	Data      []byte            `json:"data,omitempty"`

	// Protocol-specific fields.
	Amount      float64           `json:"amount,omitempty"`
	Message     string            `json:"message,omitempty"`
	PersistData map[string]string `json:"persist_data,omitempty"`

	// Coinbase-specific fields.
	BlockchainName   string  `json:"blockchain_name,omitempty"`
	BlockchainSymbol string  `json:"blockchain_symbol,omitempty"`
	BlockTime        int     `json:"block_time,omitempty"`
	Difficulty       int     `json:"difficulty,omitempty"`
	TransactionFee   float64 `json:"transaction_fee,omitempty"`
	MinerRewardPCT   float64 `json:"miner_reward_pct,omitempty"`
	MinerAddress     string  `json:"miner_address,omitempty"`
	DevRewardPCT     float64 `json:"dev_reward_pct,omitempty"`
	DevAddress       string  `json:"dev_address,omitempty"`
	FundWalletAmount float64 `json:"fund_wallet_amount,omitempty"`
	TokenCount       int64   `json:"token_count,omitempty"`
	TokenPrice       float64 `json:"token_price,omitempty"`
	AllowNewTokens   bool    `json:"allow_new_tokens,omitempty"`
	SubsidyUnits     int64   `json:"subsidy_units,omitempty"`
	BlockHeight      int64   `json:"block_height,omitempty"`

	// Anchor-specific fields.
	PublisherID     uint64 `json:"publisher_id,omitempty"`
	GameID          uint64 `json:"game_id,omitempty"`
	FromHeight      uint64 `json:"from_height,omitempty"`
	ToHeight        uint64 `json:"to_height,omitempty"`
	TipHash         string `json:"tip_hash,omitempty"`
	AnchorPayloads  uint64 `json:"anchor_payload_count,omitempty"`
	AnchorSignature string `json:"anchor_signature,omitempty"`
}

// toWire projects the base transaction onto the wire shape.
//
// The sender's public key travels with the transaction. Without it a transaction
// read back from a block could never be verified: From was serialised as a bare
// address, so the key needed to check the signature was simply gone.
func (t *Tx) toWire() txWire {
	w := txWire{
		ID:        t.ID,
		Time:      t.Time,
		Version:   t.Version,
		Protocol:  t.Protocol,
		Fee:       t.Fee,
		Status:    t.Status,
		BlockNum:  t.BlockNum,
		Signature: t.Signature,
		Nonce:     t.Nonce,
		Data:      t.Data,
	}
	if t.From != nil {
		w.From = t.From.GetAddress()
		w.FromPub = t.From.PublicPEM()
	}
	if t.To != nil {
		w.To = t.To.GetAddress()
		w.ToPub = t.To.PublicPEM()
	}
	return w
}

// applyWire restores the base transaction from the wire shape.
func (t *Tx) applyWire(w txWire) {
	t.ID = w.ID
	t.Time = w.Time
	t.Version = w.Version
	t.Protocol = w.Protocol
	t.Fee = w.Fee
	t.Status = w.Status
	t.BlockNum = w.BlockNum
	t.Signature = w.Signature
	t.Nonce = w.Nonce
	t.Data = w.Data
	t.From = walletFromWire(w.From, w.FromPub)
	t.To = walletFromWire(w.To, w.ToPub)
}

// walletFromWire rebuilds a verification-only wallet: an address plus, when it
// was carried, the public key needed to check signatures. It never holds a
// private key.
func walletFromWire(address, publicPEM string) *Wallet {
	if address == "" && publicPEM == "" {
		return nil
	}
	w := &Wallet{Address: address}
	if publicPEM != "" {
		w.vault = &Vault{Pem: &PEM{PublicKey: publicPEM}}
	}
	return w
}

// DecodeTransaction decodes a transaction from its canonical JSON encoding,
// dispatching on the "protocol" discriminator.
func DecodeTransaction(data []byte) (Transaction, error) {
	var w txWire
	if err := json.Unmarshal(data, &w); err != nil {
		return nil, fmt.Errorf("invalid transaction encoding: %w", err)
	}

	protocol := strings.ToUpper(strings.TrimSpace(w.Protocol))
	if protocol == "" {
		return nil, fmt.Errorf("transaction has no protocol")
	}

	base := Tx{}
	base.applyWire(w)
	base.Protocol = protocol

	switch protocol {
	case BankProtocolID:
		return &Bank{Tx: base, Amount: w.Amount}, nil

	case MessageProtocolID:
		return &Message{Tx: base, Message: w.Message}, nil

	case PersistProtocolID:
		return &Persist{Tx: base, Data: w.PersistData}, nil

	case CoinbaseProtocolID:
		return &Coinbase{
			Tx:               base,
			BlockchainName:   w.BlockchainName,
			BlockchainSymbol: w.BlockchainSymbol,
			BlockTime:        w.BlockTime,
			Difficulty:       w.Difficulty,
			TransactionFee:   w.TransactionFee,
			MinerRewardPCT:   w.MinerRewardPCT,
			MinerAddress:     w.MinerAddress,
			DevRewardPCT:     w.DevRewardPCT,
			DevAddress:       w.DevAddress,
			FundWalletAmount: w.FundWalletAmount,
			TokenCount:       w.TokenCount,
			TokenPrice:       w.TokenPrice,
			AllowNewTokens:   w.AllowNewTokens,
			// Without these a reloaded subsidy pays nothing, so a block that was
			// valid when mined is refused when read back from disk. This branch
			// is a second decode path alongside Coinbase.UnmarshalJSON, and the
			// two can drift -- which is how the fields went missing here in the
			// first place. TestCoinbaseWireFormatCarriesEveryField compares them.
			SubsidyUnits: w.SubsidyUnits,
			BlockHeight:  w.BlockHeight,
		}, nil

	case AnchorProtocolID:
		anchor, signature, err := anchorFromWire(w)
		if err != nil {
			return nil, err
		}
		return &AnchorTx{Tx: base, Anchor: anchor, AnchorSignature: signature}, nil

	case ChainProtocolID, P2PProtocolID:
		return &base, nil

	default:
		return nil, fmt.Errorf("unknown transaction protocol: %s", protocol)
	}
}

// EncodeTransaction encodes any transaction to its canonical JSON encoding.
func EncodeTransaction(tx Transaction) ([]byte, error) {
	if tx == nil {
		return nil, fmt.Errorf("cannot encode a nil transaction")
	}
	return json.Marshal(tx)
}
