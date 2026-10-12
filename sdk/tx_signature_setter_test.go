package sdk

import "testing"

// Every protocol takes a signature through setTransactionSignature; a protocol it
// skipped would leave the wallet's own transactions unsigned and refused.
func TestSetTransactionSignatureCoversEveryProtocol(t *testing.T) {
	for _, tx := range []Transaction{&Bank{}, &Message{}, &Coinbase{}, &Persist{}, &Tx{},
		&AnchorTx{}, &RegistrationTx{}, &AvailabilityTx{}} {
		setTransactionSignature(tx, "sig")
		if got := tx.GetSignature(); got != "sig" {
			t.Errorf("%T: signature %q, want sig", tx, got)
		}
	}
}
