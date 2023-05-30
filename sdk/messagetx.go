package sdk

import (
	"fmt"
	"time"
)

// Message is a transaction that represents a message sent from one user to another.
type Message struct {
	Tx
	Message string
}

// NewMessageTransaction creates a new message transaction.
func NewMessageTransaction(from *Wallet, to *Wallet, message string) (*Message, error) {
	fmt.Printf("[%s] Creating TX (MESSAGE) - FROM: %s, TO: %s, Message: %s\n", time.Now().Format(logDateTimeFormat), from.GetAddress(), to.GetAddress(), message)

	// Validate if wallets exist
	if from == nil || to == nil {
		return nil, fmt.Errorf("wallets can't be nil")
	}

	// Validate if there's a message
	if message == "" {
		return nil, fmt.Errorf("message can't be empty")
	}

	// Create the new Message transaction
	messageTx := &Message{
		Tx: Tx{
			From: from,
			To:   to,
			Fee:  transactionFee,
		},
		Message: message,
	}

	return messageTx, nil
}

// Process returns a string representation of the message.
func (m *Message) Process() string {
	return fmt.Sprintf("Message from %s to %s: %s", m.From.Name, m.To.Name, m.Message)
}
