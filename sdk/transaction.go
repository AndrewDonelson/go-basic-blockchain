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

	"github.com/pborman/uuid"
)

// Transaction is an interface that defines the Processes for the different types of protocol transactions.
type Transaction interface {
	Process() string
	GetProtocol() string
	GetSignature() string
	GetSenderWallet() *Wallet
	Sign(privPEM []byte) (string, error)
	Verify(pubKey []byte, sign string) (bool, error)
	Send(bc *Blockchain) error
	String() string
	Hex() string
	Hash() string
	Bytes() []byte
	Json() string
}

// Tx is a transaction that represents a generic transaction.
type Tx struct {
	ID        string    // Unit ID of the transaction
	Time      time.Time // Time the transaction was created
	Version   string    // Version of the transaction
	Protocol  string    // Protocol ID (coinbase, bank, message, etc)
	From      *Wallet   // Wallet sending the transaction
	To        *Wallet   // Wallet receiving the transaction
	Fee       float64   // Fee for the transaction
	Status    string    // Status of the transaction (pending, confirmed, inserted, failed)
	Signature string    // Signature of the transaction
	hash      []byte    // Hash of the transaction
}

// NewTransaction creates a new Base transaction with no protocol. This is used for coinbase transactions.
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

	// Create the new Message transaction
	tx := &Tx{
		ID:       uuid.New(),
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
	return t.ID
}

// String returns a string representation of the transaction.
func (t *Tx) String() string {
	return fmt.Sprintf("Transaction %s from %s to %s", t.ID, t.From.GetAddress(), t.To.GetAddress())
}

// Hex returns the hexadecimal representation of the transaction.
func (t *Tx) Hex() string {
	return hex.EncodeToString(t.Bytes())
}

// Hash returns the hash of the transaction as a string.
func (t *Tx) Hash() string {
	hash := sha256.Sum256(t.Bytes())
	return hex.EncodeToString(hash[:])
}

// Bytes returns the serialized byte representation of the transaction.
func (t *Tx) Bytes() []byte {
	data, _ := json.Marshal(t)
	return data
}

// Json returns the JSON representation of the transaction as a string.
func (t *Tx) Json() string {
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
