// Package sdk is a software development kit for building blockchain applications.
// File sdk/persisttx.go - Persistence Transaction for all On Chain Storage related Protocol based transactions
package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Persist is a transaction protocol for storing key/value pairs on the blockchain with indexing support.
type Persist struct {
	Tx
	Data map[string]string // Key/value pairs to be stored
}

// MarshalJSON encodes the Persist transaction in the canonical wire form.
//
// Persist had no MarshalJSON either, so its key/value payload never survived
// serialisation.
func (p *Persist) MarshalJSON() ([]byte, error) {
	w := p.Tx.toWire()
	w.PersistData = p.Data
	return json.Marshal(w)
}

// UnmarshalJSON decodes a Persist transaction from the canonical wire form.
func (p *Persist) UnmarshalJSON(data []byte) error {
	var w txWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	p.Tx.applyWire(w)
	p.Data = w.PersistData
	return nil
}

// NewPersistTransaction creates a new Persist transaction.
func NewPersistTransaction(from *Wallet, to *Wallet, fee float64, data map[string]string) (*Persist, error) {
	tx, err := NewTransaction(PersistProtocolID, from, to)
	if err != nil {
		return nil, err
	}

	return &Persist{
		Tx:   *tx,
		Data: data,
	}, nil
}

// Process processes the Persist transaction.
func (p *Persist) Process() string {
	// "processed" was not one of the three declared TransactionStatus values, so
	// Block.Validate (which requires StatusConfirmed) rejected every persisted
	// Persist transaction.
	p.Status = StatusConfirmed
	return "Persist transaction processed successfully"
}

// SigningBytes includes the stored key/value payload so it cannot be altered
// after signing.
func (p *Persist) SigningBytes() ([]byte, error) {
	fields := p.Tx.signingFields()
	fields["persist_data"] = p.Data
	return json.Marshal(fields)
}

// Sign signs the full Persist transaction, including its stored data.
func (p *Persist) Sign(privPEM []byte) (string, error) {
	payload, err := p.SigningBytes()
	if err != nil {
		return "", fmt.Errorf("error marshaling transaction: %w", err)
	}
	return signPayload(payload, privPEM)
}

// Verify verifies a signature over the full Persist transaction.
func (p *Persist) Verify(pubKey []byte, sign string) (bool, error) {
	payload, err := p.SigningBytes()
	if err != nil {
		return false, fmt.Errorf("error marshaling transaction: %w", err)
	}
	return verifyPayload(payload, pubKey, sign)
}

// Hash covers the stored data as well as the base fields.
func (p *Persist) Hash() string {
	p.Tx.hash = hashTransaction(p)
	return p.Tx.hash
}

// Bytes returns the canonical encoding, including the stored data.
func (p *Persist) Bytes() []byte {
	payload, err := p.SigningBytes()
	if err != nil {
		return nil
	}
	return payload
}

// Size reports the size of the full transaction.
func (p *Persist) Size() int { return len(p.Bytes()) }

// EstimateFee is derived from the full transaction size.
func (p *Persist) EstimateFee(feePerByte float64) float64 {
	return float64(p.Size()) * feePerByte
}

// Send queues the Persist transaction itself rather than its base transaction.
func (p *Persist) Send(bc *Blockchain) error {
	if err := p.Validate(); err != nil {
		return fmt.Errorf("invalid transaction: %w", err)
	}
	bc.AddTransaction(p)
	return nil
}

// Validate checks the base transaction plus Persist-specific invariants.
func (p *Persist) Validate() error {
	if err := p.Tx.Validate(); err != nil {
		return err
	}
	if len(p.Data) == 0 {
		return errors.New("persist transaction must carry at least one key/value pair")
	}
	if p.Tx.Protocol != PersistProtocolID {
		return fmt.Errorf("persist transaction has wrong protocol: %s", p.Tx.Protocol)
	}
	return nil
}
