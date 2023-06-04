// file: sdk/banktx.go - Bank Transaction for all Currency reelated Protocol based transactions
package sdk

import (
	"fmt"
)

// Bank is a transaction that represents a bank transfer.
type Bank struct {
	Tx
	Amount float64
}

func NewBankTransaction(from *Wallet, to *Wallet, amount float64) (*Bank, error) {
	tx, err := NewTransaction(BankProtocolID, from, to)
	if err != nil {
		return nil, err
	}

	// Check if the from wallet has enough balance
	if from.Balance < amount+transactionFee {
		return nil, fmt.Errorf("insufficient balance in the wallet")
	}

	return &Bank{
		Tx:     *tx,
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
