// Package sdk is a software development kit for building blockchain applications.
// File sdk/tx_authority.go - Who may spend: the one check every entry point shares.
package sdk

import (
	"errors"
	"fmt"
)

// ErrSenderNotOwner is returned when a transaction's sender address does not belong to the key
// that signed it.
var ErrSenderNotOwner = errors.New("sender address does not belong to the key that signed the transaction")

// VerifyTransactionAuthority checks that a transaction was authorised by its sender: the
// signature is valid for the public key it carries, AND that key is the one the sending address
// belongs to.
//
// The second half is the one that matters. A signature is checked against the key the
// transaction itself carries, so without tying that key to the address, anyone could name a
// victim's funded address as the sender, attach their own key, sign with it, and pass -- and the
// outputs spent would be the victim's. An address is the SHA-256 of the key's PKIX DER encoding,
// which is exactly what PeerIDFromPublicPEM computes.
//
// Every way a transaction enters a node runs this: the public API, the consensus endpoint, the
// P2P relay, node gossip, mempool submission, and -- the consensus rule -- every transaction in
// a block accepted from a peer. A coinbase has no signing sender; its rules (genesis only, the
// subsidy schedule) are checked where blocks are validated.
func VerifyTransactionAuthority(tx Transaction) error {
	if tx == nil {
		return errors.New("transaction is nil")
	}
	if tx.GetProtocol() == CoinbaseProtocolID {
		return nil
	}
	sender := tx.GetSenderWallet()
	if sender == nil {
		return errors.New("transaction has no sender wallet")
	}
	owner, err := PeerIDFromPublicPEM(sender.PublicPEM())
	if err != nil {
		return fmt.Errorf("sender public key is unusable: %w", err)
	}
	if owner != sender.GetAddress() {
		return ErrSenderNotOwner
	}
	valid, err := tx.Verify([]byte(sender.PublicPEM()), tx.GetSignature())
	if err != nil {
		return err
	}
	if !valid {
		return errors.New("signature verification failed")
	}
	return nil
}
