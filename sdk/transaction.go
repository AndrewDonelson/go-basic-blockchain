// file: sdk/transaction.go - Base Transaction for all Dynamic Protocol based transactions
// package: sdk
// description: This file contains the Transaction struct and all the methods associated with it.
package sdk

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/sha256"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/pborman/uuid"
)

// Transaction is an interface that defines the Processes for the different types of protocol transactions.
type Transaction interface {
	Process() string
	GetProtocol() string
	GetSignature() []byte
	GetHash() []byte
	GetSenderWallet() *Wallet
	Send(bc *Blockchain) error
	Sign(signature []byte) error
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
	Signature []byte    // Signature of the transaction
	Hash      []byte    // Hash of the transaction
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

	fmt.Printf("[%s] Creating TX (%s) - FROM: %s, TO: %s\n", time.Now().Format(logDateTimeFormat), protocol, from.GetAddress(), to.GetAddress())

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

// Gethash returns the hash of the transaction.
func (t *Tx) GetHash() []byte {
	return t.Hash
}

// IsCoinbase returns true if the transaction is a coinbase transaction.
func (t *Tx) IsCoinbase() bool {
	return t.Protocol == CoinbaseProtocolID
}

// Process returns a string with the process of the transaction.
func (t *Tx) Process() string {
	return fmt.Sprintf("Transaction from %s to %s", t.From.Name, t.To.Name)
}

// Send sends the filled and signed transaction to the network queue to be added to the blockchain.
func (t *Tx) Send(bc *Blockchain) error {
	// Add the transaction to the transaction queue in the blockchain
	bc.AddTransaction(t)

	// Placeholder implementation
	fmt.Printf("Transaction %s added to the transaction queue\n", t.ID)

	return nil
}

// Sign signs the transaction with the private key of the sender.
func (t *Tx) Sign(signature []byte) error {
	t.Signature = signature
	return nil
}

func (t *Tx) Verify(signature []byte) error {
	//func (t *Tx) Verify(fromWallet *Wallet) error {
	// Verify the signature logic here
	// This ensures that the transaction has not been tampered with

	// Prepare the hashed message
	hash := sha256.Sum256([]byte(t.ID))

	// Extract the r and s components from the signature
	r := big.Int{}
	s := big.Int{}
	sigLen := len(signature)
	r.SetBytes(signature[:(sigLen / 2)])
	s.SetBytes(signature[(sigLen / 2):])

	// Create the public key from the sender's wallet
	curve := elliptic.P256()

	pubBytes, err := t.From.PublicBytes()
	if err != nil {
		return err
	}

	x, y := elliptic.Unmarshal(curve, pubBytes)
	publicKey := ecdsa.PublicKey{Curve: curve, X: x, Y: y}

	// Verify the signature using the public key
	if !VerifySignature(hash[:], signature, &publicKey) {
		return errors.New("invalid signature")
	}

	return nil
}

// GetSignature returns the signature of the Persist transaction.
func (t *Tx) GetSignature() []byte {
	return t.Signature
}
