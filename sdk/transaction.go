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

// Transaction is an interface that defines the common methods for all Dynamic Protocol based transactions.
// Process() executes the transaction logic.
// GetProtocol() returns the protocol name associated with the transaction.
// GetID() returns the unique identifier for the transaction.
// GetHash() returns the hash of the transaction.
// GetSignature() returns the signature of the transaction.
// GetSenderWallet() returns the wallet of the transaction sender.
// Sign(privPEM []byte) signs the transaction with the provided private key.
// Verify(pubKey []byte, sign string) verifies the transaction signature with the provided public key.
// Send(bc *Blockchain) sends the transaction to the provided blockchain.
// String() returns a string representation of the transaction.
// Hex() returns the hexadecimal representation of the transaction.
// Hash() returns the hash of the transaction.
// Bytes() returns the byte representation of the transaction.
// JSON() returns the JSON representation of the transaction.
type Transaction interface {
	Process() string
	GetProtocol() string
	GetID() string
	GetHash() string
	GetSignature() string
	GetSenderWallet() *Wallet
	Sign(privPEM []byte) (string, error)
	Verify(pubKey []byte, sign string) (bool, error)
	Send(bc *Blockchain) error
	String() string
	Hex() string
	Hash() string
	Bytes() []byte
	JSON() string
}

// Tx is a generic transaction that represents a transfer of value between two wallets.
// The ID field is a unique identifier for the transaction, which is typically the hash of the transaction.
// The Time field represents the time the transaction was created.
// The Version field specifies the version of the transaction.
// The Protocol field identifies the protocol associated with the transaction (e.g. coinbase, bank, message).
// The From and To fields represent the sending and receiving wallets, respectively.
// The Fee field specifies the transaction fee.
// The Status field tracks the current status of the transaction (e.g. pending, confirmed, inserted, failed).
// The BlockNum field indicates the block number the transaction was inserted into.
// The Signature field contains the signature of the transaction.
// The hash field stores the hash of the transaction.
type Tx struct {
	ID        *PUID     // Unit ID of the transaction (TODO: actually this should be the hash of the transaction)
	Time      time.Time // Time the transaction was created
	Version   string    // Version of the transaction
	Protocol  string    // Protocol ID (coinbase, bank, message, etc)
	From      *Wallet   // Wallet sending the transaction
	To        *Wallet   // Wallet receiving the transaction
	Fee       float64   // Fee for the transaction
	Status    string    // Status of the transaction (pending, confirmed, inserted, failed)
	BlockNum  int       // Block number the transaction was inserted into
	Signature string    // Signature of the transaction
	hash      string    // Hash of the transaction
}

// NewTransaction creates a new transaction with the specified protocol, sender wallet, and recipient wallet.
// If the protocol is invalid, an error is returned.
// If either the sender or recipient wallet is nil, an error is returned.
// The new transaction is initialized with the current time, the specified protocol, the sender and recipient wallets,
// and a default transaction fee. The transaction ID is set to the recipient wallet's PUID, and a random asset ID is
// assigned to the recipient wallet's PUID.
func NewTransaction(protocol string, from *Wallet, to *Wallet) (*Tx, error) {
	err := isValidProtocol(protocol)
	if err != nil {
		return nil, err
	}

	// Validate if wallets exist
	if from == nil || to == nil {
		return nil, fmt.Errorf("wallets can't be nil")
	}

	fmt.Printf("[%s] Creating %s-TX - FROM: %s, TO: %s\n", time.Now().Format(logDateTimeFormat), protocol, from.GetAddress(), to.GetAddress())

	toWalletPUID := to.ID
	if toWalletPUID == nil {
		return nil, fmt.Errorf("to wallet PUID can't be empty")
	}
	assetID, err := NewRandomBigInt()
	if err != nil {
		return nil, err
	}

	toWalletPUID.SetAssetID(assetID)

	// Create the new Message transaction
	tx := &Tx{
		ID:       toWalletPUID,
		Time:     time.Now(),
		Version:  TransactionProtocolVersion,
		Protocol: protocol,
		From:     from,
		To:       to,
		Fee:      transactionFee,
	}

	return tx, nil
}

// isValidProtocol validates a provided protocol against the available protocols.
// It returns an error if the protocol is not valid.
func isValidProtocol(protocol string) error {
	// Convert the provided protocol to lowercase
	protocol = strings.ToUpper(protocol)

	// Check if the protocol exists in the available protocols slice
	for _, p := range AvailableProtocols {
		if protocol == p {
			return nil
		}
	}

	return fmt.Errorf("invalid protocol: %s", protocol)
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
	return fmt.Sprintf("%s%v%s%s%s%s%f%s%d%s",
		t.ID,
		t.Time,
		t.Version,
		t.Protocol,
		t.From.GetAddress(),
		t.To.GetAddress(),
		t.Fee,
		t.Status,
		t.BlockNum,
		t.Signature,
	)
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
	// make a copy and clear the hash property
	txCopy := *t
	txCopy.hash = ""

	hash := sha256.Sum256(txCopy.Bytes())
	t.hash = hex.EncodeToString(hash[:])
	return t.hash
}

// Bytes returns the serialized byte representation of the transaction.
func (t *Tx) Bytes() []byte {
	data, _ := json.Marshal(t)
	return data
}

// JSON returns the JSON representation of the transaction as a string.
func (t *Tx) JSON() string {
	data, _ := json.MarshalIndent(t, "", "  ")
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
	// Add the transaction to the transaction queue in the blockchain
	bc.AddTransaction(t)

	// Placeholder implementation
	fmt.Printf("Transaction %s added to the transaction queue\n", t.ID)

	return nil
}

// Sign signs the transaction with the provided private key.
func (t *Tx) Sign(privPEM []byte) (string, error) {

	txBytes, err := json.Marshal(t)
	if err != nil {
		log.Fatal(err)
	}
	reader := bytes.NewReader(txBytes)

	// Load the private key file in x509 format
	block, _ := pem.Decode(privPEM)
	if block == nil {
		return "", errors.New("privKey no pem data found")
	}

	pk, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return "", err
	}

	// Hash the input to get a summary of the information
	h := sha256.New()
	_, err = io.Copy(h, reader)
	if err != nil {
		return "", err
	}
	hash := h.Sum(nil)
	// ECDSA Signing
	sign, err := ecdsa.SignASN1(rand.Reader, pk, hash)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(sign), nil
}

// Verify verifies the signature of the transaction with the provided public key.
func (t *Tx) Verify(pubKey []byte, sign string) (bool, error) {

	txBytes, err := json.Marshal(t)
	if err != nil {
		log.Fatal(err)
	}
	reader := bytes.NewReader(txBytes)

	// Load the public key in x509 format
	block, _ := pem.Decode(pubKey)
	if block == nil {
		return false, errors.New("pubKey no pem data found")
	}
	genericPublicKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return false, err
	}
	pk := genericPublicKey.(*ecdsa.PublicKey)
	// Hash the input to get a summary of the information
	h := sha256.New()
	_, err = io.Copy(h, reader)
	if err != nil {
		return false, err
	}
	hash := h.Sum(nil)
	// ECDSA Validation
	bSign, err := base64.StdEncoding.DecodeString(sign)
	if err != nil {
		return false, err
	}
	return ecdsa.VerifyASN1(pk, hash, bSign), nil
}

// GetSignature returns the signature of the Persist transaction.
func (t *Tx) GetSignature() string {
	return t.Signature
}
