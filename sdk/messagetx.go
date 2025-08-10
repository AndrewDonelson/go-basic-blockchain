// Package sdk is a software development kit for building blockchain applications.
// File sdk/messagetx.go - Message Transaction for all Instant Messaging related Protocol based transactions
package sdk

import (
	"fmt"
)

// Message is a transaction that represents a message sent from one user to another.
type Message struct {
	Tx
	Message string
}

// NewMessageTransaction creates a new message transaction.
func NewMessageTransaction(from *Wallet, to *Wallet, message string) (*Message, error) {
	tx, err := NewTransaction(MessageProtocolID, from, to)
	if err != nil {
		return nil, err
	}

	// Validate if there's a message
	if message == "" {
		return nil, fmt.Errorf("message can't be empty")
	}

	return &Message{
		Tx:      *tx,
		Message: message,
	}, nil
}

// Process returns a string representation of the message.
func (m *Message) Process() string {
	return fmt.Sprintf("Message from %s to %s: %s", m.From.GetWalletName(), m.To.GetWalletName(), m.Message)
}

// Transaction interface method implementations
// These methods delegate to the embedded Tx struct

func (m *Message) GetProtocol() string {
	return m.Tx.GetProtocol()
}

func (m *Message) GetID() string {
	return m.Tx.GetID()
}

func (m *Message) GetHash() string {
	return m.Tx.GetHash()
}

func (m *Message) GetSignature() string {
	return m.Tx.GetSignature()
}

func (m *Message) GetSenderWallet() *Wallet {
	return m.Tx.GetSenderWallet()
}

func (m *Message) GetRecipientWallet() *Wallet {
	return m.Tx.GetRecipientWallet()
}

func (m *Message) GetFee() float64 {
	return m.Tx.GetFee()
}

func (m *Message) GetStatus() TransactionStatus {
	return m.Tx.GetStatus()
}

func (m *Message) SetStatus(status TransactionStatus) {
	m.Tx.SetStatus(status)
}

func (m *Message) Sign(privPEM []byte) (string, error) {
	return m.Tx.Sign(privPEM)
}

func (m *Message) Verify(pubKey []byte, sign string) (bool, error) {
	return m.Tx.Verify(pubKey, sign)
}

func (m *Message) Send(bc *Blockchain) error {
	return m.Tx.Send(bc)
}

func (m *Message) String() string {
	return m.Tx.String()
}

func (m *Message) Hex() string {
	return m.Tx.Hex()
}

func (m *Message) Hash() string {
	return m.Tx.Hash()
}

func (m *Message) Bytes() []byte {
	return m.Tx.Bytes()
}

func (m *Message) JSON() string {
	return m.Tx.JSON()
}

func (m *Message) Validate() error {
	return m.Tx.Validate()
}

func (m *Message) Size() int {
	return m.Tx.Size()
}

func (m *Message) EstimateFee(feePerByte float64) float64 {
	return m.Tx.EstimateFee(feePerByte)
}

func (m *Message) SetPriority(priority int) {
	m.Tx.SetPriority(priority)
}

func (m *Message) GetPriority() int {
	return m.Tx.GetPriority()
}
