package sdk

import (
	"errors"
	"strings"
	"testing"
)

// forgedSpend is a bank transaction naming victim as the sender, signed by someone else's key:
// the signature is valid for the key the transaction carries, which isn't the victim's.
func forgedSpend(t *testing.T, victim *Wallet) *Bank {
	t.Helper()
	forger := newTestWallet(t, "authority-forger", 0)
	if err := forger.SetData("balance", 1000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}
	tx, err := NewBankTransaction(forger, newTestWallet(t, "authority-payee", 0), 10)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	tx.From = walletFromWire(victim.GetAddress(), forger.PublicPEM())
	tx.Signature, err = tx.Sign([]byte(forger.PrivatePEM()))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	return tx
}

func TestVerifyTransactionAuthority(t *testing.T) {
	alice, bob := utxoWallets(t)
	if err := alice.SetData("balance", 1000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}
	honest, err := NewBankTransaction(alice, bob, 5)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if err := VerifyTransactionAuthority(honest); err == nil {
		t.Fatal("an unsigned transaction passed")
	}
	signForTest(t, honest)
	if err := VerifyTransactionAuthority(honest); err != nil {
		t.Fatalf("an honest, signed transaction was refused: %v", err)
	}
	honest.Amount = 500 // signed fields can't change afterwards
	if err := VerifyTransactionAuthority(honest); err == nil {
		t.Fatal("a transaction edited after signing passed")
	}
	if err := VerifyTransactionAuthority(forgedSpend(t, alice)); !errors.Is(err, ErrSenderNotOwner) {
		t.Fatalf("a forged sender gave %v, want ErrSenderNotOwner", err)
	}
	if err := VerifyTransactionAuthority(mintTo(t, alice, alice, 1)); err != nil {
		t.Fatalf("a coinbase has no signing sender and is checked elsewhere: %v", err)
	}
}

// TestBlocksMayOnlyCarryAuthorisedTransactions is the consensus rule: a miner can't include a
// spend its sender never signed, even though no relay path would have let it in.
func TestBlocksMayOnlyCarryAuthorisedTransactions(t *testing.T) {
	alice, bob := utxoWallets(t)
	if err := alice.SetData("balance", 1000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}
	bc := forkTestChainWithGenesisTxs(t, 0, 4, mintTo(t, alice, alice, 100))

	forged := forgedSpend(t, alice)
	block := utxoBlock(t, 1, bc.HeadHash(), forged) // already signed (by the forger): not re-signed
	if _, err := bc.AcceptBlockWithResult(block); err == nil || !strings.Contains(err.Error(), "unauthorised transaction") {
		t.Fatalf("a block carrying a forged spend gave %v", err)
	}

	unsigned, err := NewBankTransaction(alice, bob, 5)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if bc.AddTransactionLocal(unsigned) {
		t.Fatal("the mempool queued an unsigned transaction a block could never carry")
	}
	if bc.AddTransactionLocal(forged) {
		t.Fatal("the mempool queued a forged spend")
	}
}

func TestWalletOnlySendsItsOwnTransactions(t *testing.T) {
	alice, bob := utxoWallets(t)
	if err := alice.SetData("balance", 1000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}
	bc := forkTestChainWithGenesisTxs(t, 0, 4, mintTo(t, alice, alice, 100))
	if err := bob.SetData("balance", 100.0); err != nil {
		t.Fatalf("seed: %v", err)
	}
	tx, err := NewBankTransaction(alice, bob, 5)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if _, err := bob.SendTransaction(tx, bc); err == nil {
		t.Fatal("bob sent alice's transaction")
	}
}
