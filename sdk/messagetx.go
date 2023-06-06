// file: sdk/messagetx.go - Message Transaction for all Instant Messaging related Protocol based transactions
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
