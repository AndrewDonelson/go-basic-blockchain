// Package sdk is a software development kit for building blockchain applications.
// File sdk/blockchain_transactions.go - Submitting transactions to the mempool.
package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
)

var (
	// ErrTransactionAlreadyQueued is returned when a transaction with the same
	// id is already waiting in the mempool.
	ErrTransactionAlreadyQueued = errors.New("transaction is already queued")

	// ErrCoinbaseSubmitted is returned when a coinbase is offered for
	// submission. A coinbase mints supply; only the genesis block carries one.
	ErrCoinbaseSubmitted = errors.New("a coinbase transaction cannot be submitted")
)

// AddTransaction adds a transaction to the blockchain's transaction queue.
//
// The mempool is the single source of truth. Previously a BANK or MESSAGE
// transaction was handed to the sidechain router and *not* queued, and the router
// output was later reconstituted by convertToBankTransaction /
// convertToMessageTransaction -- placeholder builders that hardcoded
// `Amount: 0.0` and `Message: "Rollup message"` around wallet stubs with no keys.
// Every routed transfer therefore reached its block with its value zeroed and its
// signature unverifiable. The router now observes transactions for its rollup
// accounting while the mempool keeps the real thing.
func (bc *Blockchain) AddTransaction(transaction Transaction) {
	if err := bc.SubmitTransaction(transaction); err != nil {
		LogVerbosef("transaction %s not queued: %v", transaction.GetID(), err)
	}
}

// SubmitTransaction is AddTransaction, reporting why a transaction was not
// queued.
//
// AddTransaction answers nothing, which is fine for a caller inside the node
// and useless for one on the other side of an API: "your transaction was
// dropped" with no reason is not something an application can act on. The two
// share one path, so they cannot disagree about what is admitted.
func (bc *Blockchain) SubmitTransaction(transaction Transaction) error {
	if err := bc.admitTransactionLocal(transaction); err != nil {
		return err
	}

	// Locally originated, so relay it to peers. Transactions that arrive *from* a
	// peer use AddTransactionLocal directly: re-announcing one would bounce it
	// back and forth between nodes indefinitely.
	bc.announceTransaction(transaction)
	return nil
}

// AddTransactionLocal queues a transaction without relaying it to peers.
//
// It reports whether the transaction was newly queued, so a relayed duplicate is
// not counted or re-announced.
func (bc *Blockchain) AddTransactionLocal(transaction Transaction) bool {
	return bc.admitTransactionLocal(transaction) == nil
}

// admitTransactionLocal queues a transaction without relaying it, returning
// the reason when it is not queued.
func (bc *Blockchain) admitTransactionLocal(transaction Transaction) error {
	if transaction == nil {
		return errors.New("transaction is nil")
	}

	// A coinbase mints supply, so it can never be submitted -- by a user or a
	// peer. Only the genesis block carries one.
	if transaction.GetProtocol() == CoinbaseProtocolID {
		bc.Metrics().Inc("tx_rejected")
		LogVerbosef("Rejecting transaction %s: a coinbase cannot be submitted", transaction.GetID())
		return ErrCoinbaseSubmitted
	}

	// Only what its sender authorised may wait to be mined: a block carrying anything else is
	// refused by every peer (VerifyTransactionAuthority is a consensus rule), so a node that
	// queued it would mine blocks nobody accepts.
	if err := VerifyTransactionAuthority(transaction); err != nil {
		bc.Metrics().Inc("tx_rejected")
		LogVerbosef("Rejecting transaction %s: %v", transaction.GetID(), err)
		return err
	}

	// Reject a transaction the sender cannot afford. Nothing used to check a
	// transaction against unspent outputs, so the same funds could be committed
	// any number of times.
	if err := bc.ValidateTransactionFunds(transaction); err != nil {
		bc.Metrics().Inc("tx_rejected")
		LogVerbosef("Rejecting transaction %s: %v", transaction.GetID(), err)
		return err
	}

	// Fail fast on a nonce the chain has already confirmed. The UTXO set refuses
	// it again at block-application time; catching it here keeps a replay out of
	// the mempool rather than letting it sit there and poison every block that
	// tries to include it.
	if sender, _ := transactionParties(transaction); sender != "" {
		if last, seen := bc.UTXOSet().LastNonce(sender); seen && transaction.GetNonce() <= last {
			bc.Metrics().Inc("tx_rejected")
			LogVerbosef("Rejecting transaction %s: %v (sender %s is at nonce %d)",
				transaction.GetID(), ErrNonceNotIncreasing, sender, last)
			return fmt.Errorf("%w: sender %s is at nonce %d, this transaction carries %d",
				ErrNonceNotIncreasing, sender, last, transaction.GetNonce())
		}
	}

	id := transaction.GetID()

	bc.mux.Lock()
	for _, queued := range bc.TransactionQueue {
		if queued.GetID() == id {
			bc.mux.Unlock()
			return fmt.Errorf("%w: %s", ErrTransactionAlreadyQueued, id)
		}
	}
	// The mempool is bounded and admission is fee-competitive. It used to be an
	// unbounded slice, so anyone could grow it until the node ran out of memory.
	if err := bc.admitToMempoolLocked(transaction); err != nil {
		bc.mux.Unlock()
		bc.Metrics().Inc("tx_rejected")
		LogVerbosef("Rejecting transaction %s: %v", id, err)
		return err
	}
	bc.mux.Unlock()

	bc.Metrics().Inc("tx_submitted")

	if bc.progressIndicator != nil {
		// "pending" is accurate here. Reporting "confirmed" immediately after
		// queueing, as this used to, told the caller a transaction was final while
		// it was still sitting in the mempool.
		bc.progressIndicator.ShowTransactionProgress(id, "pending")
	}

	bc.routeToSidechain(transaction)
	return nil
}

// routeToSidechain mirrors a transaction into the protocol router for rollup
// accounting. A routing failure never loses the transaction: it is already in the
// mempool.
func (bc *Blockchain) routeToSidechain(transaction Transaction) {
	protocol := transaction.GetProtocol()
	if protocol != BankProtocolID && protocol != MessageProtocolID {
		return
	}
	if bc.sidechainRouter == nil {
		return
	}

	txData, err := json.Marshal(transaction)
	if err != nil {
		LogVerbosef("Failed to marshal transaction for sidechain: %v", err)
		return
	}

	sender, recipient := "", ""
	if w := transaction.GetSenderWallet(); w != nil {
		sender = w.GetAddress()
	}
	if w := transaction.GetRecipientWallet(); w != nil {
		recipient = w.GetAddress()
	}

	if _, err := bc.sidechainRouter.RouteTransaction(protocol, txData, sender, recipient); err != nil {
		LogVerbosef("Failed to route transaction through sidechain: %v", err)
		return
	}

	LogVerbosef("Transaction mirrored to sidechain: %s (protocol: %s)", transaction.GetID(), protocol)
}

// RemoveTransaction removes a transaction from the pending queue.
func (bc *Blockchain) RemoveTransaction(id string) bool {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	for i, tx := range bc.TransactionQueue {
		if tx.GetID() == id {
			bc.TransactionQueue = append(bc.TransactionQueue[:i], bc.TransactionQueue[i+1:]...)
			return true
		}
	}

	return false
}
