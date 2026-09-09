package sdk

import (
	"math/big"
	"strings"
	"testing"
	"time"
)

// coinbaseTestWallet builds a wallet for supply tests.
func coinbaseTestWallet(t *testing.T, name string) *Wallet {
	t.Helper()
	w, err := NewWallet(NewWalletOptions(
		ThisBlockchainOrganizationID, ThisBlockchainAppID,
		ThisBlockchainAdminUserID, ThisBlockchainDevAssetID,
		name, testPassPhrase, []string{"supply"}))
	if err != nil {
		t.Fatalf("wallet %s: %v", name, err)
	}
	return w
}

// TestCoinbaseOutsideGenesisIsRejected pins a supply-inflation hole.
//
// The UTXO set credits a coinbase's TokenCount to its recipient and consumes
// nothing, and nothing restricted which block a coinbase could appear in. A peer
// could mine an ordinary block containing a coinbase for the entire TokenCount,
// have it accepted, and mint the whole supply to itself from nothing -- once per
// block, indefinitely. Before the fix this test minted 33,554,432 tokens.
func TestCoinbaseOutsideGenesisIsRejected(t *testing.T) {
	bc := forkTestChain(t, 0, uint32(genesisDifficulty))
	attacker := coinbaseTestWallet(t, "coinbase-attacker")

	before := bc.GetBalanceUnits(attacker.GetAddress())

	cb, err := NewCoinbaseTransaction(attacker, attacker, bc.cfg)
	if err != nil {
		t.Fatalf("coinbase: %v", err)
	}
	cb.TokenCount = 33554432 // the entire supply, a second time
	cb.SetStatus(StatusConfirmed)

	head := bc.Blocks[len(bc.Blocks)-1]
	block := NewBlock([]Transaction{cb}, head.Hash)
	block.Index = *big.NewInt(1)
	block.Header.Difficulty = uint32(bc.CurrentDifficulty())
	block.Header.Timestamp = head.Header.Timestamp.Add(time.Minute)
	block.Header.MerkleRoot = block.CalculateMerkleRoot()
	block.Hash = block.CalculateHash()

	_, err = bc.AcceptBlockWithResult(block)
	if err == nil {
		t.Fatal("a block minting the whole supply outside genesis was accepted")
	}
	// A coinbase outside genesis is now the block subsidy, so the rejection comes
	// from the amount rather than from the transaction type. The guarantee is
	// stronger than it was: even a correctly-shaped claim can only move what the
	// schedule allows, out of a finite reserve, instead of minting without limit.
	if !strings.Contains(err.Error(), "subsidy") && !strings.Contains(err.Error(), "coinbase") {
		t.Fatalf("block was rejected, but not for the coinbase or its subsidy: %v", err)
	}

	if after := bc.GetBalanceUnits(attacker.GetAddress()); after != before {
		t.Fatalf("supply was inflated by %d units despite the block being rejected",
			after-before)
	}
}

// TestBlockValidateConfinesCoinbases checks the structural half of the rule.
//
// A coinbase outside genesis is the block subsidy, so it is no longer forbidden
// -- it is confined. Block.Validate enforces the shape (at most one, and first);
// validateSubsidyLocked enforces the amount, where the chain configuration is
// available to say what the height allows. Neither alone is sufficient, which is
// why both exist.
func TestBlockValidateConfinesCoinbases(t *testing.T) {
	bc := forkTestChain(t, 0, uint32(genesisDifficulty))
	w := coinbaseTestWallet(t, "coinbase-direct")

	newCoinbase := func() *Coinbase {
		cb, err := NewCoinbaseTransaction(w, w, bc.cfg)
		if err != nil {
			t.Fatalf("coinbase: %v", err)
		}
		cb.SetStatus(StatusConfirmed)
		return cb
	}

	head := bc.Blocks[0]
	build := func(txs ...Transaction) *Block {
		b := NewBlock(txs, head.Hash)
		b.Index = *big.NewInt(1)
		b.Header.Timestamp = head.Header.Timestamp.Add(time.Minute)
		b.Header.MerkleRoot = b.CalculateMerkleRoot()
		b.Hash = b.CalculateHash()
		return b
	}

	t.Run("a subsidy in first position is structurally allowed", func(t *testing.T) {
		if err := build(newCoinbase()).Validate(head); err != nil {
			t.Fatalf("a first-position coinbase was refused: %v", err)
		}
	})

	t.Run("a coinbase after another transaction is refused", func(t *testing.T) {
		message, err := NewMessageTransaction(w, w, "payload")
		if err != nil {
			t.Fatalf("message: %v", err)
		}
		if err := build(message, newCoinbase()).Validate(head); err == nil {
			t.Fatal("a coinbase in second position was accepted; the subsidy must " +
				"be first so its presence is checkable without a scan")
		}
	})

	t.Run("two coinbases are refused", func(t *testing.T) {
		if err := build(newCoinbase(), newCoinbase()).Validate(head); err == nil {
			t.Fatal("a block paying two subsidies was accepted")
		}
	})
}

// TestUTXOSetRefusesACoinbaseOutsideGenesis is the second line of defence: the
// point that actually mints must refuse regardless of how it was reached.
func TestUTXOSetRefusesACoinbaseOutsideGenesis(t *testing.T) {
	bc := forkTestChain(t, 0, uint32(genesisDifficulty))
	w := coinbaseTestWallet(t, "coinbase-utxo")

	cb, err := NewCoinbaseTransaction(w, w, bc.cfg)
	if err != nil {
		t.Fatalf("coinbase: %v", err)
	}
	cb.TokenCount = 1000

	block := NewBlock([]Transaction{cb}, "prev")
	block.Index = *big.NewInt(7)

	set := NewUTXOSet()
	if _, err := set.ApplyBlock(block, feeSplit{}); err == nil {
		t.Fatal("the UTXO set applied a coinbase from block 7")
	}

	if got := set.BalanceUnits(w.GetAddress()); got != 0 {
		t.Fatalf("the refused coinbase still credited %d units", got)
	}
}

// TestGenesisCoinbaseStillApplies: the rule must not break the one place a
// coinbase is legitimate, or the chain has no supply at all.
func TestGenesisCoinbaseStillApplies(t *testing.T) {
	bc := forkTestChain(t, 0, uint32(genesisDifficulty))
	w := coinbaseTestWallet(t, "coinbase-genesis")

	cb, err := NewCoinbaseTransaction(w, w, bc.cfg)
	if err != nil {
		t.Fatalf("coinbase: %v", err)
	}
	cb.TokenCount = 1000

	block := NewBlock([]Transaction{cb}, "")
	block.Index = *big.NewInt(0)

	set := NewUTXOSet()
	if _, err := set.ApplyBlock(block, feeSplit{}); err != nil {
		t.Fatalf("the genesis coinbase was refused: %v", err)
	}

	want := int64(1000) * UnitsPerToken
	if got := set.BalanceUnits(w.GetAddress()); got != want {
		t.Fatalf("genesis credited %d units, want %d", got, want)
	}
}

// TestCoinbaseCannotBeSubmitted keeps minting out of the mempool, so a coinbase
// can never reach a block a node mines itself.
func TestCoinbaseCannotBeSubmitted(t *testing.T) {
	bc := forkTestChain(t, 0, uint32(genesisDifficulty))
	w := coinbaseTestWallet(t, "coinbase-mempool")

	cb, err := NewCoinbaseTransaction(w, w, bc.cfg)
	if err != nil {
		t.Fatalf("coinbase: %v", err)
	}

	if bc.AddTransactionLocal(cb) {
		t.Fatal("a coinbase was accepted into the mempool")
	}
	if bc.GetMempoolSize() != 0 {
		t.Fatalf("mempool holds %d transactions after refusing a coinbase", bc.GetMempoolSize())
	}
}

// TestBranchCannotSmuggleACoinbase: a side branch is validated by a different
// path, and fork choice would otherwise let a reorg mint supply.
func TestBranchCannotSmuggleACoinbase(t *testing.T) {
	bc := forkTestChain(t, 3, uint32(genesisDifficulty))
	attacker := coinbaseTestWallet(t, "coinbase-branch")

	before := bc.GetBalanceUnits(attacker.GetAddress())

	cb, err := NewCoinbaseTransaction(attacker, attacker, bc.cfg)
	if err != nil {
		t.Fatalf("coinbase: %v", err)
	}
	cb.TokenCount = 33554432
	cb.SetStatus(StatusConfirmed)

	// Build a rival branch from block 1 carrying the coinbase.
	fork := bc.Blocks[1]
	block := NewBlock([]Transaction{cb}, fork.Hash)
	block.Index = *big.NewInt(2)
	block.Header.Difficulty = uint32(bc.ExpectedDifficulty(bc.Blocks[:2]))
	block.Header.Timestamp = fork.Header.Timestamp.Add(time.Second)
	block.Header.MerkleRoot = block.CalculateMerkleRoot()
	block.Hash = block.CalculateHash()

	_, _ = bc.AcceptBlockWithResult(block)

	if after := bc.GetBalanceUnits(attacker.GetAddress()); after != before {
		t.Fatalf("a branch minted %d units", after-before)
	}
}
