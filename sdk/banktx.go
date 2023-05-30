package sdk

import (
	"fmt"
	"time"
)

// Bank is a transaction that represents a bank transfer.
type Bank struct {
	Tx
	Amount float64
}

// NewBankTransaction creates a new bank transaction.
func NewBankTransaction(from *Wallet, to *Wallet, amount float64) (*Bank, error) {
	fmt.Printf("[%s] Creating TX (BANK) - FROM: %s, TO: %s, Amount: %f\n", time.Now().Format(logDateTimeFormat), from.Address, to.Address, amount)

	// Validate if wallets exist
	if from == nil || to == nil {
		return nil, fmt.Errorf("wallets can't be nil")
	}

	// Check if the from wallet has enough balance
	if from.Balance < amount+transactionFee {
		return nil, fmt.Errorf("insufficient balance in the wallet")
	}

	return &Bank{
		Tx: Tx{
			From: from,
			To:   to,
			Fee:  transactionFee,
		},
		Amount: amount,
	}, nil
}

// Process processes the bank transaction.
func (b *Bank) Process() string {
	// Check if From wallet has enough balance for the transaction + fee
	if b.From.Balance < (b.Amount * transactionFee) {
		return fmt.Sprintf("Insufficient balance in wallet %s", b.From.GetAddress())
	}

	// Subtract the amount from the From wallet and add it to the To wallet
	b.From.Balance -= b.Amount
	b.To.Balance += b.Amount

	//TODO: Disperse fee to the miner & dev wallet's (if applicable)

	return fmt.Sprintf("Transferred %f from %s to %s", b.Amount, b.From.Address, b.To.Address)
}
