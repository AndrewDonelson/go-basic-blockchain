package sdk

import (
	"errors"
	"testing"
)

// rbfTx builds a fake transaction from a given sender at a given nonce.
func rbfTx(t *testing.T, from, to *Wallet, id string, fee float64, nonce uint64) *fakeTx {
	t.Helper()
	tx := newFakeTx(id, fee, 250).withParties(from, to)
	tx.Tx.Nonce = nonce
	return tx
}

func rbfWallets(t *testing.T) (*Wallet, *Wallet) {
	t.Helper()
	return nonceTestWallet(t, "rbf-sender"), nonceTestWallet(t, "rbf-recipient")
}

// TestReplaceByFeeRaisesAStuckTransaction is the point of the feature: a
// transaction that priced its fee too low is otherwise stuck until it is
// evicted, and the sender cannot raise the fee because doing so needs the same
// nonce.
func TestReplaceByFeeRaisesAStuckTransaction(t *testing.T) {
	bc := mempoolTestChain(t, 50)
	alice, bob := rbfWallets(t)

	stuck := rbfTx(t, alice, bob, "stuck", 0.01, 7)
	bc.mux.Lock()
	if err := bc.admitToMempoolLocked(stuck); err != nil {
		t.Fatalf("admit: %v", err)
	}
	bc.mux.Unlock()

	better := rbfTx(t, alice, bob, "better", 1.00, 7)
	bc.mux.Lock()
	err := bc.admitToMempoolLocked(better)
	queued := len(bc.TransactionQueue)
	bc.mux.Unlock()

	if err != nil {
		t.Fatalf("a higher-paying replacement was refused: %v", err)
	}
	if queued != 1 {
		t.Fatalf("the mempool holds %d transactions; a replacement revises one "+
			"intended payment, it does not add a second", queued)
	}
	if !contains(queuedIDs(bc), "better") {
		t.Fatalf("the replacement is not in the mempool: %v", queuedIDs(bc))
	}
	if contains(queuedIDs(bc), "stuck") {
		t.Fatal("the replaced transaction is still queued")
	}
}

// TestReplacementMustPayAMargin: if any equal-paying transaction could replace
// another, two peers could bounce replacements off each other indefinitely and
// every node would relay each one.
func TestReplacementMustPayAMargin(t *testing.T) {
	bc := mempoolTestChain(t, 50)
	alice, bob := rbfWallets(t)

	original := rbfTx(t, alice, bob, "original", 1.00, 3)
	bc.mux.Lock()
	if err := bc.admitToMempoolLocked(original); err != nil {
		t.Fatalf("admit: %v", err)
	}
	bc.mux.Unlock()

	for _, tc := range []struct {
		name string
		fee  float64
	}{
		{"an equal fee", 1.00},
		{"a lower fee", 0.50},
		{"a fee below the bump", 1.05},
	} {
		t.Run(tc.name, func(t *testing.T) {
			candidate := rbfTx(t, alice, bob, "candidate", tc.fee, 3)

			bc.mux.Lock()
			err := bc.admitToMempoolLocked(candidate)
			bc.mux.Unlock()

			if !errors.Is(err, ErrDuplicateNonce) {
				t.Fatalf("expected ErrDuplicateNonce, got %v", err)
			}
			if !contains(queuedIDs(bc), "original") {
				t.Fatal("the original was displaced by a replacement that did not " +
					"pay the required margin")
			}
		})
	}
}

// TestReplacementNeedsTheSameSender: two different people using the same nonce
// are unrelated payments, not revisions of one.
func TestReplacementNeedsTheSameSender(t *testing.T) {
	bc := mempoolTestChain(t, 50)
	alice, bob := rbfWallets(t)
	carol := nonceTestWallet(t, "rbf-other-sender")

	mine := rbfTx(t, alice, bob, "alice-5", 1.00, 5)
	theirs := rbfTx(t, carol, bob, "carol-5", 100.00, 5)

	bc.mux.Lock()
	if err := bc.admitToMempoolLocked(mine); err != nil {
		t.Fatalf("admit: %v", err)
	}
	if err := bc.admitToMempoolLocked(theirs); err != nil {
		t.Fatalf("a different sender's transaction was treated as a replacement: %v", err)
	}
	queued := len(bc.TransactionQueue)
	bc.mux.Unlock()

	if queued != 2 {
		t.Fatalf("the mempool holds %d transactions, want 2 -- two senders using "+
			"the same nonce are unrelated payments", queued)
	}
}

// TestReplacementNeedsTheSameNonce: a sender's next payment is not a revision of
// the previous one.
func TestReplacementNeedsTheSameNonce(t *testing.T) {
	bc := mempoolTestChain(t, 50)
	alice, bob := rbfWallets(t)

	bc.mux.Lock()
	if err := bc.admitToMempoolLocked(rbfTx(t, alice, bob, "nonce-1", 1.00, 1)); err != nil {
		t.Fatalf("admit: %v", err)
	}
	if err := bc.admitToMempoolLocked(rbfTx(t, alice, bob, "nonce-2", 1.00, 2)); err != nil {
		t.Fatalf("admit: %v", err)
	}
	queued := len(bc.TransactionQueue)
	bc.mux.Unlock()

	if queued != 2 {
		t.Fatalf("the mempool holds %d transactions, want 2", queued)
	}
}

// TestReplacementIsRankedByFeeRate, not raw fee: block space is the scarce
// resource, so a much larger transaction paying a bit more is worth less.
func TestReplacementIsRankedByFeeRate(t *testing.T) {
	bc := mempoolTestChain(t, 50)
	alice, bob := rbfWallets(t)

	small := newFakeTx("small", 1.0, 250).withParties(alice, bob)
	small.Tx.Nonce = 4
	bc.mux.Lock()
	if err := bc.admitToMempoolLocked(small); err != nil {
		t.Fatalf("admit: %v", err)
	}
	bc.mux.Unlock()

	// Twice the fee but ten times the size: a worse deal per byte.
	bulky := newFakeTx("bulky", 2.0, 2500).withParties(alice, bob)
	bulky.Tx.Nonce = 4

	bc.mux.Lock()
	err := bc.admitToMempoolLocked(bulky)
	bc.mux.Unlock()

	if !errors.Is(err, ErrDuplicateNonce) {
		t.Fatalf("a replacement paying more in total but less per byte was accepted: %v", err)
	}
}

// TestReplacementDoesNotBypassTheCapacityBound.
func TestReplacementDoesNotBypassTheCapacityBound(t *testing.T) {
	bc := mempoolTestChain(t, 4)
	alice, bob := rbfWallets(t)

	bc.mux.Lock()
	for i := uint64(0); i < 4; i++ {
		if err := bc.admitToMempoolLocked(rbfTx(t, alice, bob, "base", 1.0, i)); err != nil {
			t.Fatalf("admit %d: %v", i, err)
		}
	}
	// A replacement of an existing nonce, while full.
	if err := bc.admitToMempoolLocked(rbfTx(t, alice, bob, "replacement", 100.0, 2)); err != nil {
		t.Fatalf("replacement while full was refused: %v", err)
	}
	queued := len(bc.TransactionQueue)
	bc.mux.Unlock()

	if queued != 4 {
		t.Fatalf("the mempool holds %d transactions against a cap of 4", queued)
	}
	if !contains(queuedIDs(bc), "replacement") {
		t.Fatal("the replacement is not queued")
	}
}

// TestReplacementIsCounted keeps the metric honest.
func TestReplacementIsCounted(t *testing.T) {
	bc := mempoolTestChain(t, 50)
	alice, bob := rbfWallets(t)

	bc.mux.Lock()
	_ = bc.admitToMempoolLocked(rbfTx(t, alice, bob, "first", 1.0, 9))
	_ = bc.admitToMempoolLocked(rbfTx(t, alice, bob, "second", 10.0, 9))
	bc.mux.Unlock()

	if got := bc.Metrics().Snapshot()["tx_replaced"]; got != 1 {
		t.Fatalf("tx_replaced = %v, want 1", got)
	}
}

// TestReplacedTransactionDoesNotReachABlock: only one of the two can ever be
// mined, and it must be the one that was kept.
func TestReplacedTransactionDoesNotReachABlock(t *testing.T) {
	bc := mempoolTestChain(t, 50)
	alice, bob := rbfWallets(t)

	bc.mux.Lock()
	_ = bc.admitToMempoolLocked(rbfTx(t, alice, bob, "underpriced", 0.01, 2))
	_ = bc.admitToMempoolLocked(rbfTx(t, alice, bob, "repriced", 5.00, 2))
	selected := bc.selectBlockTransactionsLocked()
	bc.mux.Unlock()

	if len(selected) != 1 {
		t.Fatalf("the block took %d transactions, want 1", len(selected))
	}
	if selected[0].GetID() != "repriced" {
		t.Fatalf("the block took %s, want the repriced transaction", selected[0].GetID())
	}
}
