package sdk

import (
	"errors"
	"math"
	"math/big"
	"sync"
	"testing"
)

// -----------------------------------------------------------------------------
// Helpers
// -----------------------------------------------------------------------------

// utxoTestSplit pays fees 50/50 to two fixed addresses.
func utxoTestSplit() feeSplit {
	return feeSplit{
		MinerAddress: "miner-address",
		MinerPercent: 50,
		DevAddress:   "dev-address",
		DevPercent:   50,
	}
}

// utxoBlock wraps transactions in a block at the given index.
func utxoBlock(t *testing.T, index int64, previousHash string, txs ...Transaction) *Block {
	t.Helper()
	b := NewBlock(txs, previousHash)
	b.Index = *big.NewInt(index)
	b.Header.Difficulty = 4
	b.Header.Nonce = uint32(index)
	b.Header.MerkleRoot = b.CalculateMerkleRoot()
	b.Hash = b.CalculateHash()
	return b
}

// utxoWallets returns two unlocked wallets for spending tests.
func utxoWallets(t *testing.T) (*Wallet, *Wallet) {
	t.Helper()
	return newTestWallet(t, "utxo-a", 0), newTestWallet(t, "utxo-b", 0)
}

// mintTo builds a coinbase transaction paying `tokens` to `to`.
func mintTo(t *testing.T, from, to *Wallet, tokens int64) *Coinbase {
	t.Helper()
	cfg := NewConfig()
	cb, err := NewCoinbaseTransaction(from, to, cfg)
	if err != nil {
		t.Fatalf("coinbase: %v", err)
	}
	cb.TokenCount = tokens
	return cb
}

// -----------------------------------------------------------------------------
// Amount conversion
// -----------------------------------------------------------------------------

// TestAmountUnitConversion guards the reason balances are integers: float64
// cannot represent 0.1, so accumulating float balances drifts.
func TestAmountUnitConversion(t *testing.T) {
	cases := []struct {
		amount float64
		units  int64
	}{
		{0, 0},
		{1, UnitsPerToken},
		{0.5, UnitsPerToken / 2},
		{0.00000001, 1},
		{123.45678901, 12345678901},
		{-1, -UnitsPerToken},
	}

	for _, tc := range cases {
		if got := AmountToUnits(tc.amount); got != tc.units {
			t.Fatalf("AmountToUnits(%v) = %d, want %d", tc.amount, got, tc.units)
		}
	}

	// Round trip.
	for _, units := range []int64{0, 1, UnitsPerToken, 12345678901} {
		if got := AmountToUnits(UnitsToAmount(units)); got != units {
			t.Fatalf("round trip of %d units gave %d", units, got)
		}
	}

	// Non-finite input must not become a nonsense int64.
	for _, bad := range []float64{math.NaN(), math.Inf(1), math.Inf(-1)} {
		if got := AmountToUnits(bad); got != 0 {
			t.Fatalf("AmountToUnits(%v) = %d, want 0", bad, got)
		}
	}
}

// TestIntegerBalancesDoNotDrift is the concrete case float64 gets wrong.
func TestIntegerBalancesDoNotDrift(t *testing.T) {
	// 0.1 added ten times is famously not 1.0 in float64.
	var asFloat float64
	var asUnits int64
	for i := 0; i < 10; i++ {
		asFloat += 0.1
		asUnits += AmountToUnits(0.1)
	}

	if asFloat == 1.0 {
		t.Skip("this platform's float64 happens to be exact here")
	}
	if asUnits != UnitsPerToken {
		t.Fatalf("integer accumulation drifted: got %d units, want %d", asUnits, UnitsPerToken)
	}
	if UnitsToAmount(asUnits) != 1.0 {
		t.Fatalf("integer accumulation should convert back to exactly 1.0, got %v",
			UnitsToAmount(asUnits))
	}
}

// -----------------------------------------------------------------------------
// Applying blocks
// -----------------------------------------------------------------------------

// TestCoinbaseCreatesSpendableOutput is the base case: minting credits someone.
func TestCoinbaseCreatesSpendableOutput(t *testing.T) {
	alice, _ := utxoWallets(t)
	set := NewUTXOSet()

	block := utxoBlock(t, 0, "", mintTo(t, alice, alice, 100))
	if _, err := set.ApplyBlock(block, utxoTestSplit()); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if got := set.BalanceUnits(alice.GetAddress()); got != 100*UnitsPerToken {
		t.Fatalf("expected 100 tokens, got %s", formatUnits(got))
	}
	if set.Size() != 1 {
		t.Fatalf("expected 1 unspent output, got %d", set.Size())
	}

	outputs := set.OutputsFor(alice.GetAddress())
	if len(outputs) != 1 || !outputs[0].Coinbase {
		t.Fatal("the coinbase output should be marked as such")
	}
}

// TestBankTransferMovesValueAndPaysFees covers the full accounting of a transfer:
// the recipient is credited, fees reach the miner and developer, and the sender
// gets change. Nothing is created or destroyed.
func TestBankTransferMovesValueAndPaysFees(t *testing.T) {
	alice, bob := utxoWallets(t)
	set := NewUTXOSet()
	split := utxoTestSplit()

	genesis := utxoBlock(t, 0, "", mintTo(t, alice, alice, 100))
	if _, err := set.ApplyBlock(genesis, split); err != nil {
		t.Fatalf("genesis: %v", err)
	}
	supplyBefore := set.TotalUnits()

	if err := alice.SetData("balance", 100.0); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	transfer, err := NewBankTransaction(alice, bob, 30)
	if err != nil {
		t.Fatalf("bank tx: %v", err)
	}

	block := utxoBlock(t, 1, genesis.Hash, transfer)
	if _, err := set.ApplyBlock(block, split); err != nil {
		t.Fatalf("apply transfer: %v", err)
	}

	feeUnits := AmountToUnits(transfer.GetFee())

	if got := set.BalanceUnits(bob.GetAddress()); got != 30*UnitsPerToken {
		t.Fatalf("recipient balance = %s, want 30", formatUnits(got))
	}
	wantAlice := 100*UnitsPerToken - 30*UnitsPerToken - feeUnits
	if got := set.BalanceUnits(alice.GetAddress()); got != wantAlice {
		t.Fatalf("sender balance = %s, want %s", formatUnits(got), formatUnits(wantAlice))
	}

	minerUnits := set.BalanceUnits(split.MinerAddress)
	devUnits := set.BalanceUnits(split.DevAddress)
	if minerUnits+devUnits != feeUnits {
		t.Fatalf("fees paid out (%s) do not match the fee charged (%s) -- fees used to "+
			"be deducted from the sender and credited to nobody",
			formatUnits(minerUnits+devUnits), formatUnits(feeUnits))
	}

	// Conservation: a transfer must not create or destroy supply.
	if after := set.TotalUnits(); after != supplyBefore {
		t.Fatalf("supply changed across a transfer: %s -> %s",
			formatUnits(supplyBefore), formatUnits(after))
	}
}

// TestDoubleSpendIsRejected is the property the chain had no defence against.
func TestDoubleSpendIsRejected(t *testing.T) {
	alice, bob := utxoWallets(t)
	set := NewUTXOSet()
	split := utxoTestSplit()

	genesis := utxoBlock(t, 0, "", mintTo(t, alice, alice, 10))
	if _, err := set.ApplyBlock(genesis, split); err != nil {
		t.Fatalf("genesis: %v", err)
	}

	if err := alice.SetData("balance", 1000.0); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}

	// Two transfers of 8 tokens each: Alice only has 10.
	first, err := NewBankTransaction(alice, bob, 8)
	if err != nil {
		t.Fatalf("tx: %v", err)
	}
	second, err := NewBankTransaction(alice, bob, 8)
	if err != nil {
		t.Fatalf("tx: %v", err)
	}

	block := utxoBlock(t, 1, genesis.Hash, first, second)
	_, err = set.ApplyBlock(block, split)
	if err == nil {
		t.Fatal("a block spending the same funds twice was accepted")
	}
	if !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("expected an insufficient-funds error, got %v", err)
	}

	// The failed block must leave the set exactly as it was.
	if got := set.BalanceUnits(alice.GetAddress()); got != 10*UnitsPerToken {
		t.Fatalf("a rejected block modified the set: balance is %s", formatUnits(got))
	}
	if set.BalanceUnits(bob.GetAddress()) != 0 {
		t.Fatal("a rejected block credited the recipient")
	}
}

// TestSpendingMoreThanHeldIsRejected covers a single oversized transfer.
func TestSpendingMoreThanHeldIsRejected(t *testing.T) {
	alice, bob := utxoWallets(t)
	set := NewUTXOSet()
	split := utxoTestSplit()

	genesis := utxoBlock(t, 0, "", mintTo(t, alice, alice, 5))
	if _, err := set.ApplyBlock(genesis, split); err != nil {
		t.Fatalf("genesis: %v", err)
	}

	if err := alice.SetData("balance", 1000.0); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	tx, err := NewBankTransaction(alice, bob, 50) // she has 5
	if err != nil {
		t.Fatalf("tx: %v", err)
	}

	block := utxoBlock(t, 1, genesis.Hash, tx)
	if _, err := set.ApplyBlock(block, split); !errors.Is(err, ErrInsufficientFunds) {
		t.Fatalf("expected an insufficient-funds error, got %v", err)
	}
}

// TestSpendingAnOutputCreatedInTheSameBlock covers the genesis pattern: the block
// mints to the developer and funds the miner from that same coinbase.
func TestSpendingAnOutputCreatedInTheSameBlock(t *testing.T) {
	alice, bob := utxoWallets(t)
	set := NewUTXOSet()
	split := utxoTestSplit()

	if err := alice.SetData("balance", 1000.0); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}

	coinbase := mintTo(t, alice, alice, 100)
	transfer, err := NewBankTransaction(alice, bob, 25)
	if err != nil {
		t.Fatalf("tx: %v", err)
	}

	// Both in one block, coinbase first.
	block := utxoBlock(t, 0, "", coinbase, transfer)
	if _, err := set.ApplyBlock(block, split); err != nil {
		t.Fatalf("a transaction must be able to spend an output created earlier in "+
			"the same block: %v", err)
	}

	if got := set.BalanceUnits(bob.GetAddress()); got != 25*UnitsPerToken {
		t.Fatalf("recipient balance = %s, want 25", formatUnits(got))
	}
}

// TestFeeOnlyTransactionsChargeTheSender covers MESSAGE and PERSIST.
func TestFeeOnlyTransactionsChargeTheSender(t *testing.T) {
	alice, bob := utxoWallets(t)
	set := NewUTXOSet()
	split := utxoTestSplit()

	genesis := utxoBlock(t, 0, "", mintTo(t, alice, alice, 10))
	if _, err := set.ApplyBlock(genesis, split); err != nil {
		t.Fatalf("genesis: %v", err)
	}

	message, err := NewMessageTransaction(alice, bob, "hello")
	if err != nil {
		t.Fatalf("message tx: %v", err)
	}

	block := utxoBlock(t, 1, genesis.Hash, message)
	if _, err := set.ApplyBlock(block, split); err != nil {
		t.Fatalf("apply: %v", err)
	}

	feeUnits := AmountToUnits(message.GetFee())
	if got := set.BalanceUnits(alice.GetAddress()); got != 10*UnitsPerToken-feeUnits {
		t.Fatalf("sender balance = %s, want %s",
			formatUnits(got), formatUnits(10*UnitsPerToken-feeUnits))
	}
	if set.BalanceUnits(bob.GetAddress()) != 0 {
		t.Fatal("a message transaction must not move value")
	}
}

// TestApplyBlockIsAllOrNothing: a block whose second transaction fails must leave
// the set untouched, not half-applied.
func TestApplyBlockIsAllOrNothing(t *testing.T) {
	alice, bob := utxoWallets(t)
	set := NewUTXOSet()
	split := utxoTestSplit()

	genesis := utxoBlock(t, 0, "", mintTo(t, alice, alice, 10))
	if _, err := set.ApplyBlock(genesis, split); err != nil {
		t.Fatalf("genesis: %v", err)
	}
	before := set.BalanceUnits(alice.GetAddress())
	sizeBefore := set.Size()

	if err := alice.SetData("balance", 1000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}
	good, _ := NewBankTransaction(alice, bob, 1)
	bad, _ := NewBankTransaction(alice, bob, 500) // unaffordable

	block := utxoBlock(t, 1, genesis.Hash, good, bad)
	if _, err := set.ApplyBlock(block, split); err == nil {
		t.Fatal("expected the block to be rejected")
	}

	if got := set.BalanceUnits(alice.GetAddress()); got != before {
		t.Fatalf("a partially applied block changed the sender's balance: %s -> %s",
			formatUnits(before), formatUnits(got))
	}
	if set.Size() != sizeBefore {
		t.Fatalf("a partially applied block changed the set size: %d -> %d",
			sizeBefore, set.Size())
	}
	if set.BalanceUnits(bob.GetAddress()) != 0 {
		t.Fatal("the first transaction of a rejected block was applied")
	}
}

// TestApplyingTheSameBlockTwiceIsRejected guards double-counting.
func TestApplyingTheSameBlockTwiceIsRejected(t *testing.T) {
	alice, _ := utxoWallets(t)
	set := NewUTXOSet()

	block := utxoBlock(t, 0, "", mintTo(t, alice, alice, 50))
	if _, err := set.ApplyBlock(block, utxoTestSplit()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if _, err := set.ApplyBlock(block, utxoTestSplit()); err == nil {
		t.Fatal("applying the same block twice must be refused")
	}
	if got := set.BalanceUnits(alice.GetAddress()); got != 50*UnitsPerToken {
		t.Fatalf("balance was double-counted: %s", formatUnits(got))
	}
}

// -----------------------------------------------------------------------------
// Determinism
// -----------------------------------------------------------------------------

// TestInputSelectionIsDeterministic is what makes implicit inputs sound: every
// node must select the same outputs from the same blocks. Go randomises map
// iteration, so selecting without the sort would give nodes divergent states.
//
// The same block objects are applied to independent sets, which is what different
// nodes do -- transaction IDs are generated once, at construction, so replaying
// identical blocks is the meaningful comparison.
func TestInputSelectionIsDeterministic(t *testing.T) {
	alice, bob := utxoWallets(t)
	split := utxoTestSplit()

	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Build one chain of blocks: several coinbase outputs, then a spend that has
	// real choices to make about which of them to consume.
	var blocks []*Block
	var previousHash string
	for i := int64(0); i < 5; i++ {
		b := utxoBlock(t, i, previousHash, mintTo(t, alice, alice, 10))
		blocks = append(blocks, b)
		previousHash = b.Hash
	}
	transfer, err := NewBankTransaction(alice, bob, 25)
	if err != nil {
		t.Fatalf("tx: %v", err)
	}
	blocks = append(blocks, utxoBlock(t, 5, previousHash, transfer))

	fingerprintOf := func(set *UTXOSet) []string {
		var out []string
		for _, addr := range []string{alice.GetAddress(), bob.GetAddress(),
			split.MinerAddress, split.DevAddress} {
			for _, u := range set.OutputsFor(addr) {
				out = append(out, u.Outpoint.String()+"="+formatUnits(u.Units))
			}
		}
		return out
	}

	var reference []string
	for run := 0; run < 25; run++ {
		set := NewUTXOSet()
		for i, b := range blocks {
			if _, err := set.ApplyBlock(b, split); err != nil {
				t.Fatalf("run %d block %d: %v", run, i, err)
			}
		}

		fingerprint := fingerprintOf(set)
		if reference == nil {
			reference = fingerprint
			continue
		}

		if len(fingerprint) != len(reference) {
			t.Fatalf("run %d produced %d outputs, run 0 produced %d",
				run, len(fingerprint), len(reference))
		}
		for i := range fingerprint {
			if fingerprint[i] != reference[i] {
				t.Fatalf("run %d diverged from run 0 at output %d: %q vs %q -- "+
					"input selection is not deterministic, so nodes would derive "+
					"different states from the same blocks",
					run, i, fingerprint[i], reference[i])
			}
		}
	}
}

// TestOutputsAreReturnedInDeterministicOrder covers the ordering directly.
func TestOutputsAreReturnedInDeterministicOrder(t *testing.T) {
	set := NewUTXOSet()
	set.mu.Lock()
	// Insert deliberately out of order. Every outpoint is distinct: outpoints are
	// the map key, so a repeated one would silently replace an earlier entry.
	for _, u := range []*UTXO{
		{Outpoint: Outpoint{TxID: "b", Index: 1}, Address: "x", Units: 1, BlockIndex: 2},
		{Outpoint: Outpoint{TxID: "a", Index: 0}, Address: "x", Units: 1, BlockIndex: 2},
		{Outpoint: Outpoint{TxID: "a", Index: 1}, Address: "x", Units: 1, BlockIndex: 1},
		{Outpoint: Outpoint{TxID: "c", Index: 0}, Address: "x", Units: 1, BlockIndex: 5},
	} {
		set.addLocked(u)
	}
	set.mu.Unlock()

	// Sorted by block index, then transaction ID, then output index:
	//   block 1: a:1   |   block 2: a:0, b:1   |   block 5: c:0
	want := []string{"a:1", "a:0", "b:1", "c:0"}
	for run := 0; run < 50; run++ {
		got := set.OutputsFor("x")
		if len(got) != len(want) {
			t.Fatalf("expected %d outputs, got %d", len(want), len(got))
		}
		for i := range got {
			if got[i].Outpoint.String() != want[i] {
				t.Fatalf("run %d: output %d is %s, want %s",
					run, i, got[i].Outpoint, want[i])
			}
		}
	}
}

// -----------------------------------------------------------------------------
// Revert
// -----------------------------------------------------------------------------

// TestRevertRestoresThePreviousState is what makes reorganisation possible
// without replaying the chain from genesis.
func TestRevertRestoresThePreviousState(t *testing.T) {
	alice, bob := utxoWallets(t)
	set := NewUTXOSet()
	split := utxoTestSplit()

	genesis := utxoBlock(t, 0, "", mintTo(t, alice, alice, 100))
	if _, err := set.ApplyBlock(genesis, split); err != nil {
		t.Fatalf("genesis: %v", err)
	}

	aliceBefore := set.BalanceUnits(alice.GetAddress())
	sizeBefore := set.Size()
	supplyBefore := set.TotalUnits()

	if err := alice.SetData("balance", 1000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}
	transfer, _ := NewBankTransaction(alice, bob, 40)
	spend := utxoBlock(t, 1, genesis.Hash, transfer)
	if _, err := set.ApplyBlock(spend, split); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if set.BalanceUnits(bob.GetAddress()) == 0 {
		t.Fatal("the transfer did not apply")
	}

	if err := set.RevertBlock(spend.Hash); err != nil {
		t.Fatalf("revert: %v", err)
	}

	if got := set.BalanceUnits(alice.GetAddress()); got != aliceBefore {
		t.Fatalf("sender balance after revert = %s, want %s",
			formatUnits(got), formatUnits(aliceBefore))
	}
	if got := set.BalanceUnits(bob.GetAddress()); got != 0 {
		t.Fatalf("recipient balance after revert = %s, want 0", formatUnits(got))
	}
	if set.Size() != sizeBefore {
		t.Fatalf("set size after revert = %d, want %d", set.Size(), sizeBefore)
	}
	if set.TotalUnits() != supplyBefore {
		t.Fatal("supply changed across apply/revert")
	}
	if set.HasBlock(spend.Hash) {
		t.Fatal("the reverted block should no longer be recorded as applied")
	}
}

// TestRevertMustBeInReverseOrder guards resurrecting an output that a later block
// spent.
func TestRevertMustBeInReverseOrder(t *testing.T) {
	alice, _ := utxoWallets(t)
	set := NewUTXOSet()
	split := utxoTestSplit()

	first := utxoBlock(t, 0, "", mintTo(t, alice, alice, 10))
	if _, err := set.ApplyBlock(first, split); err != nil {
		t.Fatalf("apply: %v", err)
	}
	second := utxoBlock(t, 1, first.Hash, mintTo(t, alice, alice, 10))
	if _, err := set.ApplyBlock(second, split); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if err := set.RevertBlock(first.Hash); err == nil {
		t.Fatal("reverting out of order must be refused: it could resurrect an " +
			"output that a later block already spent")
	}

	// In order it works.
	if err := set.RevertBlock(second.Hash); err != nil {
		t.Fatalf("revert second: %v", err)
	}
	if err := set.RevertBlock(first.Hash); err != nil {
		t.Fatalf("revert first: %v", err)
	}
	if set.Size() != 0 {
		t.Fatalf("expected an empty set, got %d outputs", set.Size())
	}
}

func TestRevertUnknownBlockIsRefused(t *testing.T) {
	set := NewUTXOSet()
	if err := set.RevertBlock("never-applied"); err == nil {
		t.Fatal("expected an error for a block that was never applied")
	}
}

// TestApplyRevertApplyIsStable exercises the cycle a reorganisation performs.
func TestApplyRevertApplyIsStable(t *testing.T) {
	alice, bob := utxoWallets(t)
	set := NewUTXOSet()
	split := utxoTestSplit()

	genesis := utxoBlock(t, 0, "", mintTo(t, alice, alice, 100))
	if _, err := set.ApplyBlock(genesis, split); err != nil {
		t.Fatalf("genesis: %v", err)
	}
	if err := alice.SetData("balance", 1000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	transfer, _ := NewBankTransaction(alice, bob, 40)
	spend := utxoBlock(t, 1, genesis.Hash, transfer)

	var fingerprint string
	for cycle := 0; cycle < 5; cycle++ {
		if _, err := set.ApplyBlock(spend, split); err != nil {
			t.Fatalf("cycle %d apply: %v", cycle, err)
		}

		current := formatUnits(set.BalanceUnits(alice.GetAddress())) + "/" +
			formatUnits(set.BalanceUnits(bob.GetAddress())) + "/" +
			formatUnits(set.TotalUnits())
		if cycle == 0 {
			fingerprint = current
		} else if current != fingerprint {
			t.Fatalf("cycle %d produced a different state: %s vs %s", cycle, current, fingerprint)
		}

		if err := set.RevertBlock(spend.Hash); err != nil {
			t.Fatalf("cycle %d revert: %v", cycle, err)
		}
	}
}

// TestCloneIsIndependent guards the copy used to test a branch safely.
func TestCloneIsIndependent(t *testing.T) {
	alice, _ := utxoWallets(t)
	set := NewUTXOSet()

	block := utxoBlock(t, 0, "", mintTo(t, alice, alice, 10))
	if _, err := set.ApplyBlock(block, utxoTestSplit()); err != nil {
		t.Fatalf("apply: %v", err)
	}

	clone := set.Clone()
	if clone.BalanceUnits(alice.GetAddress()) != set.BalanceUnits(alice.GetAddress()) {
		t.Fatal("the clone does not match the original")
	}

	// Mutating the clone must not touch the original.
	if err := clone.RevertBlock(block.Hash); err != nil {
		t.Fatalf("revert clone: %v", err)
	}
	if clone.Size() != 0 {
		t.Fatal("the clone was not reverted")
	}
	if set.Size() != 1 {
		t.Fatal("reverting the clone modified the original set")
	}
}

// -----------------------------------------------------------------------------
// Chain integration
// -----------------------------------------------------------------------------

// TestChainBalanceComesFromTheUTXOSet: the chain must read balances from the set,
// not rescan every block.
func TestChainBalanceComesFromTheUTXOSet(t *testing.T) {
	bc := forkTestChain(t, 0, 4)
	alice, _ := utxoWallets(t)

	if got := bc.GetBalance(alice.GetAddress()); got != 0 {
		t.Fatalf("expected a zero balance, got %v", got)
	}

	creditAddressForTest(t, bc, alice.GetAddress(), 42.5)

	if got := bc.GetBalance(alice.GetAddress()); got != 42.5 {
		t.Fatalf("chain balance = %v, want 42.5", got)
	}
	if got := bc.GetBalanceUnits(alice.GetAddress()); got != AmountToUnits(42.5) {
		t.Fatalf("chain balance units = %d, want %d", got, AmountToUnits(42.5))
	}
	if got := len(bc.UTXOs(alice.GetAddress())); got != 1 {
		t.Fatalf("expected 1 unspent output, got %d", got)
	}
}

// TestMempoolRejectsUnfundedTransactions is the double-spend guard at the
// submission boundary. Nothing used to compare a transaction against unspent
// outputs, so the same funds could be committed any number of times.
func TestMempoolRejectsUnfundedTransactions(t *testing.T) {
	bc := forkTestChain(t, 0, 4)
	alice, bob := utxoWallets(t)

	// The wallet's own cached balance says she is rich; the chain says otherwise.
	if err := alice.SetData("balance", 1000.0); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}

	tx, err := NewBankTransaction(alice, bob, 100)
	if err != nil {
		t.Fatalf("tx: %v", err)
	}

	if bc.AddTransactionLocal(tx) {
		t.Fatal("a transaction with no chain funds was accepted; the wallet's own " +
			"cached balance is not authoritative")
	}
	if len(bc.GetPendingTransactions()) != 0 {
		t.Fatal("the unfunded transaction reached the mempool")
	}

	// With real chain funds it goes through.
	creditAddressForTest(t, bc, alice.GetAddress(), 1000)
	if !bc.AddTransactionLocal(tx) {
		t.Fatal("a funded transaction was rejected")
	}
}

// TestMempoolCountsAlreadyQueuedSpends: a sender must not be able to queue the
// same coins repeatedly by submitting several transactions.
func TestMempoolCountsAlreadyQueuedSpends(t *testing.T) {
	bc := forkTestChain(t, 0, 4)
	alice, bob := utxoWallets(t)

	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed wallet: %v", err)
	}
	creditAddressForTest(t, bc, alice.GetAddress(), 100)

	first, err := NewBankTransaction(alice, bob, 60)
	if err != nil {
		t.Fatalf("tx: %v", err)
	}
	if !bc.AddTransactionLocal(first) {
		t.Fatal("the first transaction should have been accepted")
	}

	// 60 is already committed, so a second 60 exceeds the 100 she holds.
	second, err := NewBankTransaction(alice, bob, 60)
	if err != nil {
		t.Fatalf("tx: %v", err)
	}
	if bc.AddTransactionLocal(second) {
		t.Fatal("a second transaction spending funds already committed in the " +
			"mempool was accepted")
	}
}

// TestRebuildUTXOSetMatchesIncrementalApplication: replaying the chain must give
// the same set as applying blocks as they arrived. The set is derived state, and
// this is what makes deriving it safe.
func TestRebuildUTXOSetMatchesIncrementalApplication(t *testing.T) {
	bc := forkTestChain(t, 0, 4)
	alice, bob := utxoWallets(t)

	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Mint, then spend, through real blocks.
	coinbase := mintTo(t, alice, alice, 500)
	block1 := utxoBlock(t, 1, bc.HeadHash(), coinbase)
	if _, err := bc.AcceptBlockWithResult(block1); err != nil {
		t.Fatalf("accept block 1: %v", err)
	}

	transfer, _ := NewBankTransaction(alice, bob, 120)
	block2 := utxoBlock(t, 2, bc.HeadHash(), transfer)
	if _, err := bc.AcceptBlockWithResult(block2); err != nil {
		t.Fatalf("accept block 2: %v", err)
	}

	incremental := map[string]int64{}
	for _, addr := range []string{alice.GetAddress(), bob.GetAddress(),
		bc.cfg.MinerAddress, bc.cfg.DevAddress} {
		incremental[addr] = bc.GetBalanceUnits(addr)
	}
	incrementalSupply := bc.UTXOSet().TotalUnits()

	if err := bc.RebuildUTXOSet(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	for addr, want := range incremental {
		if got := bc.GetBalanceUnits(addr); got != want {
			t.Fatalf("after rebuild, %s has %s, want %s -- replaying the chain must "+
				"reproduce the incrementally built set exactly",
				addr, formatUnits(got), formatUnits(want))
		}
	}
	if got := bc.UTXOSet().TotalUnits(); got != incrementalSupply {
		t.Fatalf("supply after rebuild = %s, want %s",
			formatUnits(got), formatUnits(incrementalSupply))
	}
}

// TestReorgRebuildsTheUTXOSet: switching branches must move balances with the
// chain, not leave the set describing the abandoned history.
func TestReorgRebuildsTheUTXOSet(t *testing.T) {
	bc := forkTestChain(t, 2, 4)
	alice, bob := utxoWallets(t)

	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Fund Alice on the chain.
	fundBlock := utxoBlock(t, 3, bc.HeadHash(), mintTo(t, alice, alice, 200))
	if _, err := bc.AcceptBlockWithResult(fundBlock); err != nil {
		t.Fatalf("accept fund block: %v", err)
	}

	// A block that pays Bob, on the branch that will be abandoned.
	transfer, _ := NewBankTransaction(alice, bob, 75)
	doomed := utxoBlock(t, 4, bc.HeadHash(), transfer)
	if _, err := bc.AcceptBlockWithResult(doomed); err != nil {
		t.Fatalf("accept doomed block: %v", err)
	}

	if bc.GetBalanceUnits(bob.GetAddress()) != 75*UnitsPerToken {
		t.Fatalf("expected Bob to hold 75 tokens, got %s",
			formatUnits(bc.GetBalanceUnits(bob.GetAddress())))
	}

	// A heavier branch from block 3 that does not contain the transfer.
	rival := extendChain(bc.Blocks[3], 3, 4, "utxo-reorg")
	reorged := false
	for _, block := range rival {
		r, err := bc.AcceptBlockWithResult(block)
		if err != nil && !errors.Is(err, ErrWeakerBranch) {
			t.Fatalf("rival block: %v", err)
		}
		if r.Reorganised {
			reorged = true
		}
	}
	if !reorged {
		t.Fatal("expected a reorganisation")
	}

	// Bob's payment was only on the abandoned branch, so it must be gone.
	if got := bc.GetBalanceUnits(bob.GetAddress()); got != 0 {
		t.Fatalf("after the reorganisation Bob still holds %s -- the UTXO set was "+
			"not moved with the chain", formatUnits(got))
	}
	// Alice's coinbase was before the fork, so it survives.
	if got := bc.GetBalanceUnits(alice.GetAddress()); got != 200*UnitsPerToken {
		t.Fatalf("Alice should still hold her pre-fork coinbase, has %s", formatUnits(got))
	}

	// And the set must match a full replay of the new chain.
	before := bc.GetBalanceUnits(alice.GetAddress())
	if err := bc.RebuildUTXOSet(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}
	if after := bc.GetBalanceUnits(alice.GetAddress()); after != before {
		t.Fatalf("the set after the reorganisation disagrees with a full replay: %s vs %s",
			formatUnits(before), formatUnits(after))
	}
}

// TestBlockWithADoubleSpendIsRefusedByTheChain covers the chain-level guard.
func TestBlockWithADoubleSpendIsRefusedByTheChain(t *testing.T) {
	bc := forkTestChain(t, 0, 4)
	alice, bob := utxoWallets(t)

	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	fundBlock := utxoBlock(t, 1, bc.HeadHash(), mintTo(t, alice, alice, 10))
	if _, err := bc.AcceptBlockWithResult(fundBlock); err != nil {
		t.Fatalf("accept: %v", err)
	}

	first, _ := NewBankTransaction(alice, bob, 8)
	second, _ := NewBankTransaction(alice, bob, 8)

	doubleSpend := utxoBlock(t, 2, bc.HeadHash(), first, second)
	if _, err := bc.AcceptBlockWithResult(doubleSpend); err == nil {
		t.Fatal("a block containing a double spend was accepted onto the chain")
	}

	if bc.Height() != 1 {
		t.Fatalf("the rejected block joined the chain; height is %d", bc.Height())
	}
	if bc.GetBalanceUnits(bob.GetAddress()) != 0 {
		t.Fatal("a rejected block credited the recipient")
	}
}

// TestTotalSupplyReflectsCirculation: supply is the sum of unspent outputs, not
// the sum of everything ever minted.
func TestTotalSupplyReflectsCirculation(t *testing.T) {
	bc := forkTestChain(t, 0, 4)
	alice, bob := utxoWallets(t)

	if got := bc.CalculateTotalSupply(); got != 0 {
		t.Fatalf("expected zero supply on an empty chain, got %v", got)
	}

	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}
	fundBlock := utxoBlock(t, 1, bc.HeadHash(), mintTo(t, alice, alice, 100))
	if _, err := bc.AcceptBlockWithResult(fundBlock); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if got := bc.CalculateTotalSupply(); got != 100 {
		t.Fatalf("supply = %v, want 100", got)
	}

	// A transfer moves value but must not change the supply.
	transfer, _ := NewBankTransaction(alice, bob, 30)
	spend := utxoBlock(t, 2, bc.HeadHash(), transfer)
	if _, err := bc.AcceptBlockWithResult(spend); err != nil {
		t.Fatalf("accept: %v", err)
	}
	if got := bc.CalculateTotalSupply(); got != 100 {
		t.Fatalf("a transfer changed the supply: %v", got)
	}
}

// TestUTXOSetIsSafeUnderConcurrency exercises the locking.
func TestUTXOSetIsSafeUnderConcurrency(t *testing.T) {
	alice, _ := utxoWallets(t)
	set := NewUTXOSet()
	split := utxoTestSplit()

	genesis := utxoBlock(t, 0, "", mintTo(t, alice, alice, 1000))
	if _, err := set.ApplyBlock(genesis, split); err != nil {
		t.Fatalf("genesis: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = set.BalanceUnits(alice.GetAddress())
				_ = set.OutputsFor(alice.GetAddress())
				_ = set.TotalUnits()
				_ = set.Size()
				_ = set.Has(Outpoint{TxID: "nope", Index: 0})
				_ = set.Clone()
			}
		}()
	}
	wg.Wait()

	if got := set.BalanceUnits(alice.GetAddress()); got != 1000*UnitsPerToken {
		t.Fatalf("balance changed under concurrent reads: %s", formatUnits(got))
	}
}

// TestOutpointString covers the identifier format.
func TestOutpointString(t *testing.T) {
	if got := (Outpoint{TxID: "abc", Index: 3}).String(); got != "abc:3" {
		t.Fatalf("Outpoint.String() = %q, want %q", got, "abc:3")
	}
}
