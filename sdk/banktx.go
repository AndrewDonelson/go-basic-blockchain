// Package sdk is a software development kit for building blockchain applications.
// File sdk/banktx.go - Bank Transaction for all Currency reelated Protocol based transactions
package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
)

// Bank is a transaction that represents a bank transfer.
// It embeds the Tx struct and adds an Amount field to represent the transfer amount.
type Bank struct {
	Tx
	Amount float64
}

// MarshalJSON encodes the Bank transaction in the canonical wire form.
func (b *Bank) MarshalJSON() ([]byte, error) {
	w := b.Tx.toWire()
	w.Amount = b.Amount
	return json.Marshal(w)
}

// UnmarshalJSON decodes a Bank transaction from the canonical wire form.
func (b *Bank) UnmarshalJSON(data []byte) error {
	var w txWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	b.Tx.applyWire(w)
	b.Amount = w.Amount
	return nil
}

// NewBankTransaction creates a new Bank transaction. It takes a from wallet, a to wallet, and an amount to transfer.
// It first creates a new Transaction using the BankProtocolID, the from wallet, and the to wallet.
// It then checks if the from wallet has enough balance to cover the transfer amount plus the transaction fee.
// If the balance is sufficient, it returns a new Bank transaction with the created Transaction and the transfer amount.
// If the balance is insufficient, it returns an error.
func NewBankTransaction(from *Wallet, to *Wallet, amount float64) (*Bank, error) {
	tx, err := NewTransaction(BankProtocolID, from, to)
	if err != nil {
		return nil, err
	}

	// Advisory affordability check against the wallet's own cached balance.
	//
	// This is a convenience for callers building a transaction locally; it is NOT
	// authoritative. The UTXO set decides what can actually be spent, and
	// Blockchain.ValidateTransactionFunds enforces it when the transaction is
	// submitted. A wallet's cached number was the only check that existed before
	// the UTXO set, and nothing kept it in step with the chain.
	total := amount + transactionFee
	if from.GetBalance() < total {
		return nil, fmt.Errorf("insufficient balance in the wallet")
	}

	return &Bank{
		Tx:     *tx,
		Amount: amount,
	}, nil
}

// Process processes the bank transaction. It first checks if the "From" wallet has enough balance to cover the transaction amount plus the transaction fee. If the balance is sufficient, it subtracts the amount and fee from the "From" wallet and adds the amount to the "To" wallet. It returns a formatted string indicating the success or failure of the transaction.
func (b *Bank) Process() string {
	// Check if From wallet has enough balance for the transaction + fee
	if b.From.GetBalance() < (b.Amount + transactionFee) {
		return fmt.Sprintf("Insufficient balance in wallet %s", b.From.GetAddress())
	}

	// Debit the sender.
	newFromBalance := b.From.GetBalance() - (b.Amount + b.Fee)
	if err := b.From.SetData("balance", newFromBalance); err != nil {
		return fmt.Sprintf("Error updating wallet %s balance: %s", b.From.GetAddress(), err.Error())
	}

	// Credit the recipient. Without this the transferred value is simply destroyed
	// while the transaction reports success.
	if err := b.To.SetData("balance", b.To.GetBalance()+b.Amount); err != nil {
		return fmt.Sprintf("Error updating wallet %s balance: %s", b.To.GetAddress(), err.Error())
	}

	return fmt.Sprintf("Transferred %f from %s to %s", b.Amount, b.From.Address, b.To.Address)
}

// Transaction interface methods that MUST be overridden.
//
// Everything else (GetID, GetFee, GetStatus, ...) is promoted from the embedded Tx
// and needs no wrapper. Only the methods whose behaviour genuinely differs for a
// Bank transaction are defined here -- previously these were all hand-written
// pass-throughs, which is how Amount ended up outside the signature.

// SigningBytes includes Amount, so a signed Bank transfer cannot have its value
// altered in transit without invalidating the signature.
func (b *Bank) SigningBytes() ([]byte, error) {
	fields := b.Tx.signingFields()
	fields["amount"] = b.Amount
	return json.Marshal(fields)
}

// Sign signs the full Bank transaction, including Amount.
func (b *Bank) Sign(privPEM []byte) (string, error) {
	payload, err := b.SigningBytes()
	if err != nil {
		return "", fmt.Errorf("error marshaling transaction: %w", err)
	}
	return signPayload(payload, privPEM)
}

// Verify verifies a signature over the full Bank transaction, including Amount.
func (b *Bank) Verify(pubKey []byte, sign string) (bool, error) {
	payload, err := b.SigningBytes()
	if err != nil {
		return false, fmt.Errorf("error marshaling transaction: %w", err)
	}
	return verifyPayload(payload, pubKey, sign)
}

// Hash covers Amount as well as the base fields.
func (b *Bank) Hash() string {
	if b == nil || b.Tx.ID == nil {
		return ""
	}
	b.Tx.hash = hashTransaction(b)
	return b.Tx.hash
}

// Bytes returns the canonical encoding, including Amount.
func (b *Bank) Bytes() []byte {
	payload, err := b.SigningBytes()
	if err != nil {
		return nil
	}
	return payload
}

// Size reports the size of the full transaction, not just its base fields.
func (b *Bank) Size() int {
	return len(b.Bytes())
}

// EstimateFee is derived from the full transaction size.
func (b *Bank) EstimateFee(feePerByte float64) float64 {
	return float64(b.Size()) * feePerByte
}

// Send queues the Bank transaction itself. Delegating to Tx.Send would enqueue the
// embedded base transaction and silently drop Amount.
func (b *Bank) Send(bc *Blockchain) error {
	if err := b.Validate(); err != nil {
		return fmt.Errorf("invalid transaction: %w", err)
	}
	bc.AddTransaction(b)
	LogVerbosef("Bank transaction %s added to the transaction queue", b.GetID())
	return nil
}

// Validate checks the base transaction plus Bank-specific invariants.
func (b *Bank) Validate() error {
	if err := b.Tx.Validate(); err != nil {
		return err
	}
	if b.Amount <= 0 {
		return errors.New("bank transaction amount must be greater than zero")
	}
	if b.Tx.Protocol != BankProtocolID {
		return fmt.Errorf("bank transaction has wrong protocol: %s", b.Tx.Protocol)
	}
	return nil
}

// GetID is nil-safe because rollup reconstruction can produce partially built
// Bank values before their identity has been assigned.
func (b *Bank) GetID() string {
	if b == nil || b.Tx.ID == nil {
		return ""
	}
	return b.Tx.GetID()
}
