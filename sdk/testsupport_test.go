package sdk

import (
	"fmt"
	"sync/atomic"
	"testing"
)

// Lower the scrypt cost for the whole test binary.
//
// This used to live in production code as a testing.Testing() branch inside
// deriveKey, which (a) pulled the "testing" package into the shipped binary and
// (b) was never recorded in the wallet file, so a wallet created under test could
// not be opened in production. The cost profile is now chosen here and persisted
// in the wallet's EncryptionParams.
func init() {
	defaultScryptParams = fastScryptParams
}

// creditAddressForTest gives an address spendable funds in the UTXO set.
//
// The UTXO set is now the authoritative record of who owns what, so seeding a
// wallet's own cached balance is no longer enough to make a transaction
// spendable -- which is the whole point of the change. This arranges chain state
// directly rather than mining a block per test, which would dominate the runtime.
func creditAddressForTest(tb testing.TB, bc *Blockchain, address string, tokens float64) {
	tb.Helper()

	if address == "" {
		tb.Fatal("cannot credit an empty address")
	}

	set := bc.UTXOSet()
	set.mu.Lock()
	defer set.mu.Unlock()

	set.addLocked(&UTXO{
		Outpoint: Outpoint{
			TxID:  fmt.Sprintf("test-credit-%s-%d", address, atomic.AddInt64(&testCreditSeq, 1)),
			Index: 0,
		},
		Address:  address,
		Units:    AmountToUnits(tokens),
		Coinbase: true,
	})
}

// testCreditSeq keeps synthetic credit outpoints unique.
var testCreditSeq int64

// fundWalletForTest credits a wallet both on-chain and in its own cache, so both
// the authoritative check and the advisory one in NewBankTransaction pass.
func fundWalletForTest(tb testing.TB, bc *Blockchain, w *Wallet, tokens float64) {
	tb.Helper()

	if err := w.SetData("balance", tokens); err != nil {
		tb.Fatalf("set wallet balance: %v", err)
	}
	creditAddressForTest(tb, bc, w.GetAddress(), tokens)
}
