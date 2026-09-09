package sdk

import (
	"errors"
	"math/big"
	"testing"
	"time"
)

func nonceTestWallet(t *testing.T, name string) *Wallet {
	t.Helper()
	w, err := NewWallet(NewWalletOptions(
		ThisBlockchainOrganizationID, ThisBlockchainAppID,
		ThisBlockchainAdminUserID, ThisBlockchainDevAssetID,
		name, testPassPhrase, []string{"nonce"}))
	if err != nil {
		t.Fatalf("wallet %s: %v", name, err)
	}
	if err := w.Open(testPassPhrase); err != nil {
		t.Fatalf("open %s: %v", name, err)
	}
	return w
}

// -----------------------------------------------------------------------------
// Assignment
// -----------------------------------------------------------------------------

// TestNoncesAreSequentialNotRandom is the change this rests on. A random nonce
// makes each transaction distinct but says nothing about order, so it cannot
// stop an already-mined transaction being applied again, and gives replace-by-fee
// no stable key.
func TestNoncesAreSequentialNotRandom(t *testing.T) {
	bc := forkTestChain(t, 0, uint32(genesisDifficulty))
	alice := nonceTestWallet(t, "nonce-sequential")
	bob := nonceTestWallet(t, "nonce-sequential-to")
	_ = bc

	for want := uint64(0); want < 5; want++ {
		tx, err := NewTransaction(MessageProtocolID, alice, bob)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if tx.GetNonce() != want {
			t.Fatalf("transaction %d has nonce %d, want %d", want, tx.GetNonce(), want)
		}
	}
}

func TestWalletNonceAccessors(t *testing.T) {
	w := nonceTestWallet(t, "nonce-accessors")

	if got := w.NextNonce(); got != 0 {
		t.Fatalf("a fresh wallet starts at nonce %d, want 0", got)
	}
	if got := w.ReserveNonce(); got != 0 {
		t.Fatalf("first reserved nonce is %d, want 0", got)
	}
	if got := w.NextNonce(); got != 1 {
		t.Fatalf("after reserving, next is %d, want 1", got)
	}

	w.SetNextNonce(42)
	if got := w.ReserveNonce(); got != 42 {
		t.Fatalf("after SetNextNonce(42), reserved %d", got)
	}

	// A nil wallet must not panic: transactions are built from wallets that may
	// not be set up yet.
	var missing *Wallet
	if got := missing.ReserveNonce(); got != 0 {
		t.Fatalf("nil wallet reserved %d", got)
	}
	if got := missing.NextNonce(); got != 0 {
		t.Fatalf("nil wallet reported %d", got)
	}
	missing.SetNextNonce(9)
}

// TestNonceIsCoveredBySignature: if the nonce were unsigned, an old transaction
// could be relabelled with a fresh nonce and replayed.
func TestNonceIsCoveredBySignature(t *testing.T) {
	alice := nonceTestWallet(t, "nonce-signed")
	bob := nonceTestWallet(t, "nonce-signed-to")

	tx, err := NewTransaction(MessageProtocolID, alice, bob)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	before, err := tx.SigningBytes()
	if err != nil {
		t.Fatalf("signing bytes: %v", err)
	}

	tx.SetNonce(tx.GetNonce() + 1)

	after, err := tx.SigningBytes()
	if err != nil {
		t.Fatalf("signing bytes: %v", err)
	}

	if string(before) == string(after) {
		t.Fatal("changing the nonce did not change the signing payload, so a " +
			"mined transaction could be renumbered and replayed")
	}
}

// -----------------------------------------------------------------------------
// Chain enforcement
// -----------------------------------------------------------------------------

// TestUTXOSetRefusesARepeatedNonce is the replay guard itself.
func TestUTXOSetRefusesARepeatedNonce(t *testing.T) {
	alice, bob := utxoWallets(t)
	set := NewUTXOSet()
	split := utxoTestSplit()

	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	genesis := utxoBlock(t, 0, "", mintTo(t, alice, alice, 100))
	if _, err := set.ApplyBlock(genesis, split); err != nil {
		t.Fatalf("genesis: %v", err)
	}

	first, err := NewBankTransaction(alice, bob, 10)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	block1 := utxoBlock(t, 1, genesis.Hash, first)
	if _, err := set.ApplyBlock(block1, split); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// The same transaction offered again on a later block.
	block2 := utxoBlock(t, 2, block1.Hash, first)
	_, err = set.ApplyBlock(block2, split)
	if err == nil {
		t.Fatal("a transaction was applied to the chain twice")
	}
	if !errors.Is(err, ErrNonceNotIncreasing) {
		t.Fatalf("expected ErrNonceNotIncreasing, got %v", err)
	}
}

func TestNonceMustStrictlyIncrease(t *testing.T) {
	alice, bob := utxoWallets(t)
	set := NewUTXOSet()
	split := utxoTestSplit()

	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}
	genesis := utxoBlock(t, 0, "", mintTo(t, alice, alice, 100))
	if _, err := set.ApplyBlock(genesis, split); err != nil {
		t.Fatalf("genesis: %v", err)
	}

	high, err := NewBankTransaction(alice, bob, 5)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	high.SetNonce(10)

	block1 := utxoBlock(t, 1, genesis.Hash, high)
	if _, err := set.ApplyBlock(block1, split); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Going backwards is refused, even though the nonce was never used.
	low, err := NewBankTransaction(alice, bob, 5)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	low.SetNonce(4)

	block2 := utxoBlock(t, 2, block1.Hash, low)
	if _, err := set.ApplyBlock(block2, split); !errors.Is(err, ErrNonceNotIncreasing) {
		t.Fatalf("a lower nonce was accepted: %v", err)
	}

	if got := set.NextNonce(alice.GetAddress()); got != 11 {
		t.Fatalf("next nonce is %d, want 11", got)
	}
}

// TestNonceStateSurvivesAReorg: a rolled-back transaction must become
// resubmittable, or the sender is locked out of a nonce they never spent.
func TestNonceStateSurvivesAReorg(t *testing.T) {
	alice, bob := utxoWallets(t)
	set := NewUTXOSet()
	split := utxoTestSplit()

	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}
	genesis := utxoBlock(t, 0, "", mintTo(t, alice, alice, 100))
	if _, err := set.ApplyBlock(genesis, split); err != nil {
		t.Fatalf("genesis: %v", err)
	}

	if _, seen := set.LastNonce(alice.GetAddress()); seen {
		t.Fatal("a coinbase recipient should have no send nonce")
	}

	spend, err := NewBankTransaction(alice, bob, 10)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	block1 := utxoBlock(t, 1, genesis.Hash, spend)
	if _, err := set.ApplyBlock(block1, split); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if _, seen := set.LastNonce(alice.GetAddress()); !seen {
		t.Fatal("the spend did not record a nonce")
	}

	// Roll it back, as a reorganisation would.
	if err := set.RevertBlock(block1.Hash); err != nil {
		t.Fatalf("revert: %v", err)
	}

	if _, seen := set.LastNonce(alice.GetAddress()); seen {
		t.Fatal("the nonce survived the rollback, so the sender can never " +
			"resubmit the transaction that was rolled back")
	}

	// And the very same transaction applies cleanly on the new branch.
	replacement := utxoBlock(t, 1, genesis.Hash, spend)
	replacement.Header.Nonce = 99
	replacement.Hash = replacement.CalculateHash()
	if _, err := set.ApplyBlock(replacement, split); err != nil {
		t.Fatalf("the rolled-back transaction could not be re-applied: %v", err)
	}
}

// TestNonceUndoIsPerBlockNotPerTransaction: two transactions from one sender in
// one block must restore the value from before the block.
func TestNonceUndoIsPerBlockNotPerTransaction(t *testing.T) {
	alice, bob := utxoWallets(t)
	set := NewUTXOSet()
	split := utxoTestSplit()

	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}
	genesis := utxoBlock(t, 0, "", mintTo(t, alice, alice, 500))
	if _, err := set.ApplyBlock(genesis, split); err != nil {
		t.Fatalf("genesis: %v", err)
	}

	first, _ := NewBankTransaction(alice, bob, 5)
	block1 := utxoBlock(t, 1, genesis.Hash, first)
	if _, err := set.ApplyBlock(block1, split); err != nil {
		t.Fatalf("apply: %v", err)
	}
	afterFirst, _ := set.LastNonce(alice.GetAddress())

	second, _ := NewBankTransaction(alice, bob, 5)
	third, _ := NewBankTransaction(alice, bob, 5)
	block2 := utxoBlock(t, 2, block1.Hash, second, third)
	if _, err := set.ApplyBlock(block2, split); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if err := set.RevertBlock(block2.Hash); err != nil {
		t.Fatalf("revert: %v", err)
	}

	got, seen := set.LastNonce(alice.GetAddress())
	if !seen || got != afterFirst {
		t.Fatalf("after reverting a two-transaction block the nonce is %d (seen=%v), "+
			"want %d -- the undo must restore the value from before the block, not "+
			"from between its transactions", got, seen, afterFirst)
	}
}

// TestCoinbaseIsExemptFromNonceRules: it has no sender and mints rather than
// spends.
func TestCoinbaseIsExemptFromNonceRules(t *testing.T) {
	alice, _ := utxoWallets(t)
	set := NewUTXOSet()

	cb := mintTo(t, alice, alice, 10)
	cb.SetNonce(0)

	genesis := utxoBlock(t, 0, "", cb)
	if _, err := set.ApplyBlock(genesis, utxoTestSplit()); err != nil {
		t.Fatalf("a coinbase was refused on nonce grounds: %v", err)
	}
}

// TestMempoolRefusesAConfirmedNonce keeps a replay out of the queue rather than
// letting it poison every block that tries to include it.
func TestMempoolRefusesAConfirmedNonce(t *testing.T) {
	alice, bob := utxoWallets(t)
	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	bc := forkTestChainWithGenesisTxs(t, 0, 4, mintTo(t, alice, alice, 100))

	spend, err := NewBankTransaction(alice, bob, 10)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	block := utxoBlock(t, 1, bc.HeadHash(), spend)
	if _, err := bc.AcceptBlockWithResult(block); err != nil {
		t.Fatalf("accept: %v", err)
	}

	if bc.AddTransactionLocal(spend) {
		t.Fatal("an already-mined transaction was admitted to the mempool")
	}
}

// TestNextNonceForCountsQueuedTransactions: two transactions built back to back
// must not claim the same nonce.
func TestNextNonceForCountsQueuedTransactions(t *testing.T) {
	bc := forkTestChain(t, 0, uint32(genesisDifficulty))
	alice := nonceTestWallet(t, "nonce-queued")
	bob := nonceTestWallet(t, "nonce-queued-to")

	if got := bc.NextNonceFor(alice.GetAddress()); got != 0 {
		t.Fatalf("an address with no history is at nonce %d, want 0", got)
	}
	if got := bc.NextNonceFor(""); got != 0 {
		t.Fatalf("an empty address gave %d", got)
	}

	first, err := NewTransaction(MessageProtocolID, alice, bob)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	first.Fee = 0
	if !bc.AddTransactionLocal(first) {
		t.Fatal("the first transaction was refused")
	}

	if got := bc.NextNonceFor(alice.GetAddress()); got != first.GetNonce()+1 {
		t.Fatalf("with one queued transaction the next nonce is %d, want %d",
			got, first.GetNonce()+1)
	}
}

// TestSyncWalletNonceRecoversAfterReload covers the failure mode of holding the
// counter in memory: a wallet loaded from disk starts at zero.
func TestSyncWalletNonceRecoversAfterReload(t *testing.T) {
	alice, bob := utxoWallets(t)
	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	bc := forkTestChainWithGenesisTxs(t, 0, 4, mintTo(t, alice, alice, 100))

	spend, err := NewBankTransaction(alice, bob, 10)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	block := utxoBlock(t, 1, bc.HeadHash(), spend)
	if _, err := bc.AcceptBlockWithResult(block); err != nil {
		t.Fatalf("accept: %v", err)
	}

	// Simulate a reload: the in-memory counter is gone.
	alice.SetNextNonce(0)
	bc.SyncWalletNonce(alice)

	if got := alice.NextNonce(); got != spend.GetNonce()+1 {
		t.Fatalf("after syncing, the wallet is at nonce %d, want %d",
			got, spend.GetNonce()+1)
	}

	// A transaction built now is accepted rather than refused as a replay.
	next, err := NewBankTransaction(alice, bob, 5)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !bc.AddTransactionLocal(next) {
		t.Fatal("a transaction built after syncing the nonce was still refused")
	}

	bc.SyncWalletNonce(nil) // must not panic
}

// TestBlockSelectionKeepsSenderNonceOrder: fee ranking can put a sender's nonce 1
// ahead of its nonce 0, and a block whose nonces run backwards for a sender is
// invalid.
func TestBlockSelectionKeepsSenderNonceOrder(t *testing.T) {
	bc := mempoolTestChain(t, 100)

	alice := nonceTestWallet(t, "nonce-order-alice")
	bob := nonceTestWallet(t, "nonce-order-bob")

	// The later nonce pays far more, so fee ranking alone would put it first.
	cheapFirst := newFakeTx("cheap-nonce-0", 0.01, 250).withParties(alice, bob)
	cheapFirst.Tx.Nonce = 0
	richSecond := newFakeTx("rich-nonce-1", 50.0, 250).withParties(alice, bob)
	richSecond.Tx.Nonce = 1

	bc.mux.Lock()
	bc.TransactionQueue = []Transaction{cheapFirst, richSecond}
	selected := bc.selectBlockTransactionsLocked()
	bc.mux.Unlock()

	if len(selected) != 2 {
		t.Fatalf("expected both transactions, got %d", len(selected))
	}

	var seen []uint64
	for _, tx := range selected {
		if sender, _ := transactionParties(tx); sender == alice.GetAddress() {
			seen = append(seen, tx.GetNonce())
		}
	}
	for i := 1; i < len(seen); i++ {
		if seen[i] <= seen[i-1] {
			t.Fatalf("the block orders one sender's nonces as %v; a block whose "+
				"nonces run backwards for a sender cannot be applied", seen)
		}
	}
}

// TestBlockWithBackwardsNoncesIsRejected proves the ordering above matters.
func TestBlockWithBackwardsNoncesIsRejected(t *testing.T) {
	alice, bob := utxoWallets(t)
	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	bc := forkTestChainWithGenesisTxs(t, 0, 4, mintTo(t, alice, alice, 500))

	first, _ := NewBankTransaction(alice, bob, 5)
	second, _ := NewBankTransaction(alice, bob, 5)

	// Deliberately the wrong way round.
	block := NewBlock([]Transaction{second, first}, bc.HeadHash())
	block.Index = *big.NewInt(1)
	block.Header.Difficulty = uint32(bc.CurrentDifficulty())
	block.Header.Timestamp = bc.Blocks[0].Header.Timestamp.Add(time.Minute)
	block.Header.MerkleRoot = block.CalculateMerkleRoot()
	block.Hash = block.CalculateHash()

	if _, err := bc.AcceptBlockWithResult(block); err == nil {
		t.Fatal("a block whose nonces run backwards for one sender was accepted")
	}
}
