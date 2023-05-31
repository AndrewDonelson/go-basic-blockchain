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

	// Validate if wallets exist
	if from == nil || to == nil {
		return nil, fmt.Errorf("wallets can't be nil")
	}

	// Check if the from wallet has enough balance
	if from.Balance < amount+transactionFee {
		return nil, fmt.Errorf("insufficient balance in the wallet")
	}

	fmt.Printf("[%s] Creating TX (BANK)\n- FROM: %s\n- TO: %s\n- Amount: %f\n", time.Now().Format(logDateTimeFormat), from.GetAddress(), to.GetAddress(), amount)

	// Create the new Bank transaction
	return &Bank{
		Tx: Tx{
			Version:  TransactionProtocolVersion,
			Protocol: BankProtocolID,
			From:     from,
			To:       to,
			Fee:      transactionFee,
		},
		Amount: amount,
	}, nil
}

// GetProtocol returns the protocol ID of the transaction. bank in this case.
func (b *Bank) GetProtocol() string {
	return b.Protocol
}

// // Verify returns an error if the transaction is not valid.
// func (b *Bank) Verify(signature []byte) error {
// 	return nil
// }

// // Send sends the filled and signed transaction to the network que to be added to the blockchain.
// func (b *Bank) Send() error {
// 	return nil
// }

// // Sign signs the transaction with the private key of the sender.
// func (b *Bank) Sign(signature []byte) error {
// 	b.Tx.Sign(signature)
// 	return nil
// }

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
