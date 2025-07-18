// Package sdk is a software development kit for building blockchain applications.
// File sdk/banktx.go - Bank Transaction for all Currency reelated Protocol based transactions
package sdk

import (
	"encoding/json"
	"fmt"
)

// Bank is a transaction that represents a bank transfer.
// It embeds the Tx struct and adds an Amount field to represent the transfer amount.
type Bank struct {
	Tx
	Amount float64
}

// MarshalJSON implements custom JSON marshaling for Bank transaction
func (b *Bank) MarshalJSON() ([]byte, error) {
	// First marshal the base Tx
	baseTx, err := json.Marshal(&b.Tx)
	if err != nil {
		return nil, err
	}

	// Create a map to hold the base transaction data
	var baseMap map[string]interface{}
	err = json.Unmarshal(baseTx, &baseMap)
	if err != nil {
		return nil, err
	}

	// Add the bank-specific data
	baseMap["amount"] = b.Amount

	// Serialize the protocol data to the Data field
	protocolData := map[string]interface{}{
		"amount": b.Amount,
	}

	protocolDataBytes, err := json.Marshal(protocolData)
	if err != nil {
		return nil, err
	}
	baseMap["data"] = protocolDataBytes

	return json.Marshal(baseMap)
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

	// Check if the from wallet has enough balance
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

	// Subtract the amount from the From wallet and add it to the To wallet
	newFromBalance := b.From.GetBalance() - (b.Amount + transactionFee)
	err := b.From.SetData("balance", newFromBalance)
	if err != nil {
		return fmt.Sprintf("Error updating wallet %s balance: %s", b.From.GetAddress(), err.Error())
	}

	//TODO: Disperse fee to the miner & dev wallet's (if applicable)

	return fmt.Sprintf("Transferred %f from %s to %s", b.Amount, b.From.Address, b.To.Address)
}
