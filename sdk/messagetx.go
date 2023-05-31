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
			Version:  TransactionProtocolVersion,
			Protocol: MessageProtocolID,
			From:     from,
			To:       to,
			Fee:      transactionFee,
		},
		Message: message,
	}

	return messageTx, nil
}

// GetProtocol returns the protocol ID of the transaction. message in this case.
func (m *Message) GetProtocol() string {
	return m.Protocol
}

// // Verify returns an error if the transaction is not valid.
// func (m *Message) Verify(signature []byte) error {
// 	return nil
// }

// // Send sends the filled and signed transaction to the network que to be added to the blockchain.
// func (m *Message) Send() error {
// 	return nil
// }

// // Sign signs the transaction with the private key of the sender.
// func (m *Message) Sign(signature []byte) error {
// 	return nil
// }

// Process returns a string representation of the message.
func (m *Message) Process() string {
	return fmt.Sprintf("Message from %s to %s: %s", m.From.Name, m.To.Name, m.Message)
}
