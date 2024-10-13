// Package sdk is a software development kit for building blockchain applications.
// File sdk/transaction.go - Base Transaction for all Dynamic Protocol based transactions
package sdk

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/gob"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log"
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
	GetFee() float64 // New method to get the transaction fee
	GetStatus() TransactionStatus
	SetStatus(status TransactionStatus)
	Sign(privPEM []byte) (string, error)
	Verify(pubKey []byte, sign string) (bool, error)
	Send(bc *Blockchain) error
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

	log.Printf("[%s] Creating %s-TX - FROM: %s, TO: %s\n", time.Now().Format(time.RFC3339), protocol, from.GetAddress(), to.GetAddress())

	toWalletPUID := to.ID
	if toWalletPUID == nil {
		return nil, fmt.Errorf("to wallet PUID can't be empty")
	}
	assetID, err := NewRandomBigInt()
	if err != nil {
		return nil, err
	}

	toWalletPUID.SetAssetID(assetID)

	tx := &Tx{
		ID:       toWalletPUID,
		Time:     time.Now(),
		Version:  TransactionVersion,
		Protocol: protocol,
		From:     from,
		To:       to,
		Fee:      transactionFee,
		Status:   StatusPending,
		Nonce:    from.GetNextNonce(),
	}

	return tx, nil
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

// GetID returns the ID of the transaction.
func (t *Tx) GetID() string {
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

// Hash returns the hash of the transaction as a string.
func (t *Tx) Hash() string {
	txCopy := *t
	txCopy.hash = ""
	txCopy.Signature = ""
	hash := sha256.Sum256(txCopy.Bytes())
	t.hash = hex.EncodeToString(hash[:])
	return t.hash
}

// Bytes returns the serialized byte representation of the transaction.
func (t *Tx) Bytes() []byte {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(t)
	if err != nil {
		log.Printf("Error encoding transaction: %v", err)
		return nil
	}
	return buf.Bytes()
}

// JSON returns the JSON representation of the transaction as a string.
func (t *Tx) JSON() string {
	data, err := json.MarshalIndent(t, "", "  ")
	if err != nil {
		log.Printf("Error marshaling transaction to JSON: %v", err)
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
	log.Printf("Transaction %s added to the transaction queue\n", t.ID)
	return nil
}

// Sign signs the transaction with the provided private key.
func (t *Tx) Sign(privPEM []byte) (string, error) {
	txBytes, err := json.Marshal(t)
	if err != nil {
		return "", fmt.Errorf("error marshaling transaction: %v", err)
	}
	reader := bytes.NewReader(txBytes)

	block, _ := pem.Decode(privPEM)
	if block == nil {
		return "", errors.New("failed to decode PEM block containing private key")
	}

	pk, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("error parsing private key: %v", err)
	}

	h := sha256.New()
	if _, err := io.Copy(h, reader); err != nil {
		return "", fmt.Errorf("error hashing transaction: %v", err)
	}
	hash := h.Sum(nil)

	sign, err := ecdsa.SignASN1(rand.Reader, pk, hash)
	if err != nil {
		return "", fmt.Errorf("error signing transaction: %v", err)
	}
	return base64.StdEncoding.EncodeToString(sign), nil
}

// Verify verifies the signature of the transaction with the provided public key.
func (t *Tx) Verify(pubKey []byte, sign string) (bool, error) {
	txBytes, err := json.Marshal(t)
	if err != nil {
		return false, fmt.Errorf("error marshaling transaction: %v", err)
	}
	reader := bytes.NewReader(txBytes)

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

	h := sha256.New()
	if _, err := io.Copy(h, reader); err != nil {
		return false, fmt.Errorf("error hashing transaction: %v", err)
	}
	hash := h.Sum(nil)

	bSign, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return false, fmt.Errorf("error decoding signature: %v", err)
	}
	return ecdsa.VerifyASN1(pk, hash, bSign), nil
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
