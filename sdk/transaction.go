// Package sdk is a software development kit for building blockchain applications.
// File sdk/transaction.go - Base Transaction for all Dynamic Protocol based transactions
package sdk

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
	"time"
)

// TransactionVersion represents the current version of the transaction structure.
const TransactionVersion = 1

// TransactionStatus represents the possible states of a transaction.
type TransactionStatus string

const (
	StatusPending   TransactionStatus = "pending"
	StatusConfirmed TransactionStatus = "confirmed"
	StatusFailed    TransactionStatus = "failed"
)

// Transaction is an interface that defines the common methods for all Dynamic Protocol based transactions.
type Transaction interface {
	Process() string
	GetProtocol() string
	GetID() string
	GetHash() string
	GetSignature() string
	GetSenderWallet() *Wallet
	GetRecipientWallet() *Wallet
	GetFee() float64 // New method to get the transaction fee
	GetStatus() TransactionStatus
	SetStatus(status TransactionStatus)
	Sign(privPEM []byte) (string, error)
	Verify(pubKey []byte, sign string) (bool, error)
	Send(bc *Blockchain) error
	// SigningBytes returns the canonical payload a signature commits to. Every
	// protocol must include its own value-bearing fields here, otherwise those
	// fields are unsigned and can be tampered with in transit.
	SigningBytes() ([]byte, error)
	String() string
	Hex() string
	Hash() string
	Bytes() []byte
	JSON() string
	Validate() error
	Size() int
	EstimateFee(feePerByte float64) float64
	SetPriority(priority int)
	GetPriority() int
	// GetNonce returns the sender's sequence number for this transaction. It is
	// covered by the signature, so it cannot be edited to make an already-mined
	// transaction look new.
	GetNonce() uint64
	SetNonce(nonce uint64)
}

// Tx is a generic transaction that represents a transfer of value between two wallets.
type Tx struct {
	ID        *PUID             `json:"id"`
	Time      time.Time         `json:"time"`
	Version   int               `json:"version"`
	Protocol  string            `json:"protocol"`
	From      *Wallet           `json:"from"`
	To        *Wallet           `json:"to"`
	Fee       float64           `json:"fee"`
	Status    TransactionStatus `json:"status"`
	BlockNum  int               `json:"block_num"`
	Signature string            `json:"signature"`
	hash      string            `json:"-"`
	priority  int               `json:"-"`
	Nonce     uint64            `json:"nonce"`
	Data      []byte            `json:"data"`
}

// NewTransaction creates a new transaction with the specified protocol, sender wallet, and recipient wallet.
func NewTransaction(protocol string, from *Wallet, to *Wallet) (*Tx, error) {
	if err := isValidProtocol(protocol); err != nil {
		return nil, err
	}

	if from == nil || to == nil {
		return nil, fmt.Errorf("wallets can't be nil")
	}

	LogInfof("Creating %s transaction: %s → %s", protocol, from.GetAddress()[:8], to.GetAddress()[:8])

	if to.ID == nil {
		return nil, fmt.Errorf("to wallet PUID can't be empty")
	}
	assetID, err := NewRandomBigInt()
	if err != nil {
		return nil, err
	}

	// Build a *fresh* PUID. Taking to.ID directly would alias the recipient
	// wallet's own identity: mutating it here would rewrite the wallet's ID and
	// retroactively change the ID of every transaction previously sent to it.
	txID := NewPUID(
		NewBigInt(to.ID.GetOrganizationID().Val),
		NewBigInt(to.ID.GetAppID().Val),
		NewBigInt(to.ID.GetUserID().Val),
		assetID,
	)

	tx := &Tx{
		ID:       txID,
		Time:     time.Now(),
		Version:  TransactionVersion,
		Protocol: protocol,
		From:     from,
		To:       to,
		Fee:      transactionFee,
		Status:   StatusPending,
		// The nonce is the sender's sequence number, not a random value.
		//
		// A random nonce makes each transaction distinct but says nothing about
		// order, so it cannot stop an already-mined transaction being applied a
		// second time on another branch, and there is no stable key on which to
		// replace a stuck transaction with a better-paying one. Both need a
		// per-sender sequence.
		Nonce: from.ReserveNonce(),
	}

	return tx, nil
}

// MarshalJSON encodes the transaction in the canonical wire form.
//
// The previous implementation flattened From/To to addresses but its
// UnmarshalJSON counterpart ignored those fields entirely, so From and To came
// back nil and the next GetSenderWallet().GetAddress() nil-dereferenced.
func (t *Tx) MarshalJSON() ([]byte, error) {
	return json.Marshal(t.toWire())
}

// UnmarshalJSON decodes the canonical wire form, restoring From and To as
// verification-only wallets.
func (t *Tx) UnmarshalJSON(data []byte) error {
	var w txWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	t.applyWire(w)
	return nil
}

// isValidProtocol validates a provided protocol against the available protocols.
func isValidProtocol(protocol string) error {
	protocol = strings.ToUpper(protocol)
	for _, p := range AvailableProtocols {
		if protocol == p {
			return nil
		}
	}
	return fmt.Errorf("invalid protocol: %s", protocol)
}

// GetFee returns the fee for the transaction.
func (t *Tx) GetFee() float64 {
	return t.Fee
}

// GetStatus returns the current status of the transaction.
func (t *Tx) GetStatus() TransactionStatus {
	return t.Status
}

// SetStatus sets the status of the transaction.
func (t *Tx) SetStatus(status TransactionStatus) {
	t.Status = status
}

// GetProtocol returns the protocol ID of the transaction.
func (t *Tx) GetProtocol() string {
	return t.Protocol
}

// GetSenderWallet retrieves the sender's wallet from the blockchain based on the sender's address.
func (t *Tx) GetSenderWallet() *Wallet {
	return t.From
}

// GetRecipientWallet retrieves the recipient's wallet from the blockchain based on the recipient's address.
func (t *Tx) GetRecipientWallet() *Wallet {
	return t.To
}

// GetID returns the ID of the transaction.
func (t *Tx) GetID() string {
	// Belt and braces alongside the nil check in PUID.String: an ID-less
	// transaction is refused elsewhere by its empty ID, not by a panic here.
	if t == nil {
		return ""
	}
	return t.ID.String()
}

// GetHash returns the hash of the transaction.
func (t *Tx) GetHash() string {
	return t.hash
}

// String returns a string representation of the transaction.
func (t *Tx) String() string {
	return fmt.Sprintf("ID: %s, Time: %v, Version: %d, Protocol: %s, From: %s, To: %s, Fee: %f, Status: %s, BlockNum: %d, Nonce: %d",
		t.ID, t.Time, t.Version, t.Protocol, t.From.GetAddress(), t.To.GetAddress(), t.Fee, t.Status, t.BlockNum, t.Nonce)
}

// Log returns a string with the log of the transaction.
func (t *Tx) Log() string {
	return fmt.Sprintf("Transaction %s from %s to %s", t.ID, t.From.GetAddress(), t.To.GetAddress())
}

// Hex returns the hexadecimal representation of the transaction.
func (t *Tx) Hex() string {
	return hex.EncodeToString(t.Bytes())
}

// hashTransaction derives a transaction's hash from its canonical signing payload,
// so the hash covers exactly the same fields the signature does. Passing the
// concrete transaction (not the embedded Tx) is what makes protocol fields count.
func hashTransaction(tx Transaction) string {
	payload, err := tx.SigningBytes()
	if err != nil {
		LogInfof("Error building signing bytes for hash: %v", err)
		return ""
	}
	hash := sha256.Sum256(payload)
	return hex.EncodeToString(hash[:])
}

// Hash returns the hash of the transaction as a string.
func (t *Tx) Hash() string {
	t.hash = hashTransaction(t)
	return t.hash
}

// Bytes returns the serialized byte representation of the transaction.
// It uses the canonical signing payload so that, unlike gob, the encoding is
// stable across processes and covers the concrete protocol's own fields.
func (t *Tx) Bytes() []byte {
	payload, err := t.SigningBytes()
	if err != nil {
		LogInfof("Error encoding transaction: %v", err)
		return nil
	}
	return payload
}

// JSON returns the JSON representation of the transaction as a string.
func (t *Tx) JSON() string {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		LogInfof("Error marshaling transaction to JSON: %v", err)
		return ""
	}
	return string(data)
}

// IsCoinbase returns true if the transaction is a coinbase transaction.
func (t *Tx) IsCoinbase() bool {
	return t.Protocol == CoinbaseProtocolID
}

// Process returns a string with the process of the transaction.
func (t *Tx) Process() string {
	return fmt.Sprintf("Transaction from %s to %s", t.From.GetAddress(), t.To.GetAddress())
}

// Send sends the filled and signed transaction to the network queue to be added to the blockchain.
func (t *Tx) Send(bc *Blockchain) error {
	if err := t.Validate(); err != nil {
		return fmt.Errorf("invalid transaction: %v", err)
	}

	bc.AddTransaction(t)
	LogInfof("Transaction %s added to the transaction queue\n", t.ID)
	return nil
}

// signingFields returns the base fields every transaction commits to when signed.
// Signature and the cached hash are deliberately absent: including them would make
// a signature depend on itself, and verification could never reproduce the digest.
func (t *Tx) signingFields() map[string]interface{} {
	fields := map[string]interface{}{
		"version":  t.Version,
		"protocol": t.Protocol,
		"fee":      t.Fee,
		"nonce":    t.Nonce,
		"time":     t.Time.UTC().UnixNano(),
		"data":     t.Data,
	}
	if t.ID != nil {
		fields["id"] = t.ID.String()
	}
	if t.From != nil {
		fields["from"] = t.From.GetAddress()
	}
	if t.To != nil {
		fields["to"] = t.To.GetAddress()
	}
	return fields
}

// SigningBytes returns the canonical signing payload for a base transaction.
// json.Marshal sorts map keys, so the encoding is deterministic across runs.
func (t *Tx) SigningBytes() ([]byte, error) {
	return json.Marshal(t.signingFields())
}

// signPayload signs an already-canonical payload with the given PEM private key.
func signPayload(payload []byte, privPEM []byte) (string, error) {
	block, _ := pem.Decode(privPEM)
	if block == nil {
		return "", errors.New("failed to decode PEM block containing private key")
	}

	pk, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("error parsing private key: %v", err)
	}

	hash := sha256.Sum256(payload)

	sign, err := ecdsa.SignASN1(rand.Reader, pk, hash[:])
	if err != nil {
		return "", fmt.Errorf("error signing transaction: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sign), nil
}

// verifyPayload checks a signature over an already-canonical payload.
func verifyPayload(payload []byte, pubKey []byte, sign string) (bool, error) {
	block, _ := pem.Decode(pubKey)
	if block == nil {
		return false, errors.New("failed to decode PEM block containing public key")
	}
	genericPublicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return false, fmt.Errorf("error parsing public key: %v", err)
	}
	pk, ok := genericPublicKey.(*ecdsa.PublicKey)
	if !ok {
		return false, errors.New("not an ECDSA public key")
	}

	hash := sha256.Sum256(payload)

	bSign, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return false, fmt.Errorf("error decoding signature: %v", err)
	}
	return ecdsa.VerifyASN1(pk, hash[:], bSign), nil
}

// Sign signs the transaction with the provided private key.
//
// Concrete protocols (Bank, Message, ...) MUST override this so that their own
// value-bearing fields are covered; see Bank.Sign. Go has no virtual dispatch on
// embedded structs, so delegating to Tx.Sign would sign only the base fields.
func (t *Tx) Sign(privPEM []byte) (string, error) {
	payload, err := t.SigningBytes()
	if err != nil {
		return "", fmt.Errorf("error marshaling transaction: %v", err)
	}
	return signPayload(payload, privPEM)
}

// Verify verifies the signature of the transaction with the provided public key.
func (t *Tx) Verify(pubKey []byte, sign string) (bool, error) {
	payload, err := t.SigningBytes()
	if err != nil {
		return false, fmt.Errorf("error marshaling transaction: %v", err)
	}
	return verifyPayload(payload, pubKey, sign)
}

// GetSignature returns the signature of the transaction.
func (t *Tx) GetSignature() string {
	return t.Signature
}

// Validate checks if the transaction is valid.
func (t *Tx) Validate() error {
	if t.From == nil || t.To == nil {
		return errors.New("invalid sender or recipient")
	}
	if t.Fee < 0 {
		return errors.New("invalid fee")
	}
	if t.Version != TransactionVersion {
		return fmt.Errorf("unsupported transaction version: %d", t.Version)
	}
	if err := isValidProtocol(t.Protocol); err != nil {
		return err
	}
	return nil
}

// Size returns the size of the transaction in bytes.
func (t *Tx) Size() int {
	return len(t.Bytes())
}

// EstimateFee estimates the fee for the transaction based on its size and the given fee per byte.
func (t *Tx) EstimateFee(feePerByte float64) float64 {
	return float64(t.Size()) * feePerByte
}

// SetPriority sets the priority of the transaction.
func (t *Tx) SetPriority(priority int) {
	t.priority = priority
}

// GetPriority returns the priority of the transaction.
func (t *Tx) GetPriority() int {
	return t.priority
}

// GetNonce returns the sender's sequence number for this transaction.
func (t *Tx) GetNonce() uint64 {
	if t == nil {
		return 0
	}
	return t.Nonce
}

// SetNonce sets the sender's sequence number.
//
// It must be called before signing: the nonce is part of the signing payload,
// so changing it afterwards invalidates the signature.
func (t *Tx) SetNonce(nonce uint64) {
	if t == nil {
		return
	}
	t.Nonce = nonce
}
