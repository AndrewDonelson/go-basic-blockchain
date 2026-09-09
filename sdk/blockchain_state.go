// Package sdk is a software development kit for building blockchain applications.
// File sdk/blockchain_state.go - Derived state: balances, the UTXO set, funds and nonces.
package sdk

import (
	"errors"
	"fmt"
)

// SyncWalletNonce resets a wallet's nonce counter from chain state.
//
// A wallet holds its counter in memory, so one loaded from disk starts at zero
// and would produce transactions the chain refuses as replays. Callers that load
// a wallet and then spend from it must call this first.
//
// Transactions already queued count too: two transactions built back to back
// must not both claim the same nonce.
func (bc *Blockchain) SyncWalletNonce(w *Wallet) {
	if w == nil {
		return
	}
	w.SetNextNonce(bc.NextNonceFor(w.GetAddress()))
}

// NextNonceFor returns the nonce an address should use for its next
// transaction, accounting for what is already in the mempool.
func (bc *Blockchain) NextNonceFor(address string) uint64 {
	if address == "" {
		return 0
	}

	next := bc.UTXOSet().NextNonce(address)

	bc.mux.Lock()
	defer bc.mux.Unlock()
	for _, tx := range bc.TransactionQueue {
		if tx == nil {
			continue
		}
		sender, _ := transactionParties(tx)
		if sender != address {
			continue
		}
		if queued := tx.GetNonce(); queued >= next {
			next = queued + 1
		}
	}
	return next
}

// GetBalance returns the balance of a given wallet address.
func (bc *Blockchain) GetBalance(address string) float64 {
	bc.mux.Lock()
	set := bc.ensureUTXOSetLocked()
	bc.mux.Unlock()

	return set.Balance(address)
}

// GetBalanceUnits returns a balance in indivisible base units.
//
// Prefer this over GetBalance wherever the value is compared or accumulated:
// float64 cannot represent 0.1 exactly, so float balances drift.
func (bc *Blockchain) GetBalanceUnits(address string) int64 {
	bc.mux.Lock()
	set := bc.ensureUTXOSetLocked()
	bc.mux.Unlock()

	return set.BalanceUnits(address)
}

// UTXOs returns the unspent outputs held by an address.
func (bc *Blockchain) UTXOs(address string) []*UTXO {
	bc.mux.Lock()
	set := bc.ensureUTXOSetLocked()
	bc.mux.Unlock()

	return set.OutputsFor(address)
}

// UTXOSet returns the live set. Callers must not mutate it.
func (bc *Blockchain) UTXOSet() *UTXOSet {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	return bc.ensureUTXOSetLocked()
}

// ensureUTXOSetLocked lazily creates the set, replaying any blocks already held.
//
// Blockchain is constructed as a struct literal in several places, so the set
// cannot rely on NewBlockchain having run.
func (bc *Blockchain) ensureUTXOSetLocked() *UTXOSet {
	if bc.utxos != nil {
		return bc.utxos
	}

	set := NewUTXOSet()
	split := bc.feeSplitFor()
	for _, block := range bc.Blocks {
		if _, err := set.ApplyBlock(block, split); err != nil {
			LogVerbosef("Could not replay block %s into the UTXO set: %v",
				block.Index.String(), err)
		}
	}
	bc.utxos = set
	return set
}

// feeSplitFor returns where fees are paid, from the chain config.
func (bc *Blockchain) feeSplitFor() feeSplit {
	if bc.cfg == nil {
		return feeSplit{}
	}
	return feeSplit{
		MinerAddress: bc.cfg.MinerAddress,
		MinerPercent: bc.cfg.MinerRewardPCT,
		DevAddress:   bc.cfg.DevAddress,
		DevPercent:   bc.cfg.DevRewardPCT,
	}
}

// RebuildUTXOSet replays the chain to reconstruct the set from scratch.
//
// The set is derived state and is deliberately not persisted: replaying the
// blocks is what guarantees it matches them. Loading a snapshot would let the
// two drift, which is the class of bug this whole file exists to remove.
func (bc *Blockchain) RebuildUTXOSet() error {
	bc.mux.Lock()
	blocks := make([]*Block, len(bc.Blocks))
	copy(blocks, bc.Blocks)
	split := bc.feeSplitFor()
	bc.mux.Unlock()

	rebuilt := NewUTXOSet()
	for _, block := range blocks {
		if _, err := rebuilt.ApplyBlock(block, split); err != nil {
			return fmt.Errorf("replaying block %s: %w", block.Index.String(), err)
		}
	}

	bc.mux.Lock()
	bc.utxos = rebuilt
	bc.mux.Unlock()

	LogVerbosef("UTXO set rebuilt: %d unspent outputs, %.8f tokens in circulation",
		rebuilt.Size(), UnitsToAmount(rebuilt.TotalUnits()))
	return nil
}

// CanSpend reports whether an address can cover an amount plus a fee.
func (bc *Blockchain) CanSpend(address string, amount, fee float64) bool {
	need := AmountToUnits(amount) + AmountToUnits(fee)
	return bc.GetBalanceUnits(address) >= need
}

// ValidateTransactionFunds checks a transaction against the current set.
//
// This is what actually prevents a double spend: previously nothing compared a
// transaction against what remained unspent, so the same funds could be
// committed any number of times.
func (bc *Blockchain) ValidateTransactionFunds(tx Transaction) error {
	if tx == nil {
		return errors.New("transaction is nil")
	}

	sender, _ := transactionParties(tx)
	if sender == "" {
		return errors.New("transaction has no sender")
	}

	need := AmountToUnits(tx.GetFee())
	if bank, ok := tx.(*Bank); ok {
		need += AmountToUnits(bank.Amount)
	}
	if need <= 0 {
		return nil
	}

	// Funds already committed by queued transactions must count against the
	// balance, or a sender could queue the same coins repeatedly.
	pending := bc.pendingSpendUnits(sender, tx.GetID())

	if available := bc.GetBalanceUnits(sender); available < need+pending {
		return fmt.Errorf("%w: %s has %s unspent (%s already committed in the mempool), needs %s",
			ErrInsufficientFunds, sender, formatUnits(available),
			formatUnits(pending), formatUnits(need))
	}
	return nil
}

// pendingSpendUnits sums what a sender has already committed in the mempool,
// ignoring the transaction being validated.
func (bc *Blockchain) pendingSpendUnits(address, excludeID string) int64 {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	var total int64
	for _, tx := range bc.TransactionQueue {
		if tx.GetID() == excludeID {
			continue
		}
		sender, _ := transactionParties(tx)
		if sender != address {
			continue
		}
		total += AmountToUnits(tx.GetFee())
		if bank, ok := tx.(*Bank); ok {
			total += AmountToUnits(bank.Amount)
		}
	}
	return total
}

// CalculateTotalSupply calculates the total supply of tokens in the blockchain.
// CalculateTotalSupply returns the tokens currently in circulation.
//
// This is the sum of every unspent output, not the sum of every coinbase ever
// minted. The old version counted minted supply only, so it never reflected what
// was actually held.
func (bc *Blockchain) CalculateTotalSupply() float64 {
	bc.mux.Lock()
	set := bc.ensureUTXOSetLocked()
	bc.mux.Unlock()

	return UnitsToAmount(set.TotalUnits())
}
