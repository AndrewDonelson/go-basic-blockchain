package sdk

import (
	"crypto"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"time"
)

// Transaction is an interface that defines the Processes for the different types of protocol transactions.
type Transaction interface {
	GetProtocol() string
	Process() string
	Verify(signature []byte) error
	Send() error
	Sign(signature []byte) error
}

// Tx is a transaction that represents a generic transaction.
type Tx struct {
	Version   string  // Version of the transaction
	Protocol  string  // Protocol ID (coinbase, bank, message, etc)
	From      *Wallet // Wallet sending the transaction
	To        *Wallet // Wallet receiving the transaction
	Fee       float64 // Fee for the transaction
	Signature []byte  // Signature of the transaction
}

// NewTransaction creates a new Base transaction with no protocol. This is used for coinbase transactions.
func NewTransaction(from *Wallet, to *Wallet) (*Tx, error) {
	fmt.Printf("[%s] Creating TX (COINBASE) - FROM: %s, TO: %s\n", time.Now().Format(logDateTimeFormat), from.GetAddress(), to.GetAddress())

	// Validate if wallets exist
	if from == nil || to == nil {
		return nil, fmt.Errorf("wallets can't be nil")
	}

	// Create the new Message transaction
	tx := &Tx{
		Version:  TransactionProtocolVersion,
		Protocol: CoinbaseProtocolID,
		From:     from,
		To:       to,
		Fee:      transactionFee,
	}

	return tx, nil
}

// GetProtocol returns the protocol ID of the transaction.
func (t *Tx) GetProtocol() string {
	return t.Protocol
}

// IsCoinbase returns true if the transaction is a coinbase transaction.
func (t *Tx) IsCoinbase() bool {
	return t.Protocol == CoinbaseProtocolID
}

// IsBank returns true if the transaction is a bank transaction.
func (t *Tx) IsBank() bool {
	return t.Protocol == BankProtocolID
}

// IsMessage returns true if the transaction is a message transaction.
func (t *Tx) IsMessage() bool {
	return t.Protocol == MessageProtocolID
}

func (t *Tx) Process() string {
	return fmt.Sprintf("Transaction from %s to %s", t.From.Name, t.To.Name)
}

// Verify returns an error if the transaction is not valid.
func (t *Tx) Verify(signature []byte) error {
	return nil
}

// Send sends the filled and signed transaction to the network que to be added to the blockchain.
func (t *Tx) Send() error {
	return nil
}

// Sign signs the transaction with the private key of the sender.
func (t *Tx) Sign(signature []byte) error {
	return nil
}

func (t *Tx) VerifyTransactionSignature(signature []byte, pubKeyBytes []byte) error {
	// Convert the public key to rsa.PublicKey
	block, _ := pem.Decode(pubKeyBytes)
	if block == nil {
		return fmt.Errorf("failed to parse PEM block containing the public key")
	}

	pubKey, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse public key: %v", err)
	}

	rsaPubKey, ok := pubKey.(*rsa.PublicKey)
	if !ok {
		return fmt.Errorf("public key is not of type *rsa.PublicKey")
	}

	// Get the SHA-256 hash of the transaction
	txHash := sha256.Sum256([]byte(fmt.Sprintf("%v", t)))

	// Verify the transaction signature
	err = rsa.VerifyPKCS1v15(rsaPubKey, crypto.SHA256, txHash[:], signature)
	if err != nil {
		return fmt.Errorf("failed to verify transaction signature: %v", err)
	}

	return nil
}
