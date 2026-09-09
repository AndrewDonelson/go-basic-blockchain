// Package sdk is a software development kit for building blockchain applications.
// File sdk/messagetx.go - Message Transaction for all Instant Messaging related Protocol based transactions
package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Message is a transaction that represents a message sent from one user to another.
type Message struct {
	Tx
	Message string
}

// MarshalJSON encodes the Message transaction in the canonical wire form.
//
// Message had no MarshalJSON, so Go promoted Tx.MarshalJSON and the message body
// was silently dropped on every serialisation.
func (m *Message) MarshalJSON() ([]byte, error) {
	w := m.Tx.toWire()
	w.Message = m.Message
	return json.Marshal(w)
}

// UnmarshalJSON decodes a Message transaction from the canonical wire form.
func (m *Message) UnmarshalJSON(data []byte) error {
	var w txWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	m.Tx.applyWire(w)
	m.Message = w.Message
	return nil
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
	if len(message) > MaxMessageLength {
		return nil, fmt.Errorf("message exceeds maximum length of %d bytes", MaxMessageLength)
	}

	return &Message{
		Tx:      *tx,
		Message: message,
	}, nil
}

// Process returns a string representation of the message.
func (m *Message) Process() string {
	from, to := "unknown", "unknown"
	if m.From != nil {
		from = m.From.GetWalletName()
	}
	if m.To != nil {
		to = m.To.GetWalletName()
	}
	m.Status = StatusConfirmed
	return fmt.Sprintf("Message from %s to %s: %s", from, to, m.Message)
}

// Transaction interface methods that MUST be overridden.
//
// Everything else is promoted from the embedded Tx; hand-written pass-throughs
// only hid the fact that Message was never covered by the signature.

// SigningBytes includes the message body so it cannot be rewritten after signing.
func (m *Message) SigningBytes() ([]byte, error) {
	fields := m.Tx.signingFields()
	fields["message"] = m.Message
	return json.Marshal(fields)
}

// Sign signs the full Message transaction, including the message body.
func (m *Message) Sign(privPEM []byte) (string, error) {
	payload, err := m.SigningBytes()
	if err != nil {
		return "", fmt.Errorf("error marshaling transaction: %w", err)
	}
	return signPayload(payload, privPEM)
}

// Verify verifies a signature over the full Message transaction.
func (m *Message) Verify(pubKey []byte, sign string) (bool, error) {
	payload, err := m.SigningBytes()
	if err != nil {
		return false, fmt.Errorf("error marshaling transaction: %w", err)
	}
	return verifyPayload(payload, pubKey, sign)
}

// Hash covers the message body as well as the base fields.
func (m *Message) Hash() string {
	m.Tx.hash = hashTransaction(m)
	return m.Tx.hash
}

// Bytes returns the canonical encoding, including the message body.
func (m *Message) Bytes() []byte {
	payload, err := m.SigningBytes()
	if err != nil {
		return nil
	}
	return payload
}

// Size reports the size of the full transaction.
func (m *Message) Size() int { return len(m.Bytes()) }

// EstimateFee is derived from the full transaction size.
func (m *Message) EstimateFee(feePerByte float64) float64 {
	return float64(m.Size()) * feePerByte
}

// Send queues the Message transaction itself rather than its base transaction.
func (m *Message) Send(bc *Blockchain) error {
	if err := m.Validate(); err != nil {
		return fmt.Errorf("invalid transaction: %w", err)
	}
	bc.AddTransaction(m)
	return nil
}

// Validate checks the base transaction plus Message-specific invariants.
func (m *Message) Validate() error {
	if err := m.Tx.Validate(); err != nil {
		return err
	}
	if m.Message == "" {
		return errors.New("message transaction body can't be empty")
	}
	if len(m.Message) > MaxMessageLength {
		return fmt.Errorf("message exceeds maximum length of %d bytes", MaxMessageLength)
	}
	if m.Tx.Protocol != MessageProtocolID {
		return fmt.Errorf("message transaction has wrong protocol: %s", m.Tx.Protocol)
	}
	return nil
}

// GetID is nil-safe because rollup reconstruction can produce partially built values.
func (m *Message) GetID() string {
	if m == nil || m.Tx.ID == nil {
		return ""
	}
	return m.Tx.GetID()
}
