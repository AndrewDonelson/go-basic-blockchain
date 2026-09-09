// Package sdk is a software development kit for building blockchain applications.
// File sdk/anchortx_test.go - Tests for anchors reaching the main chain.
package sdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

// anchorFixture is a publisher with a sidechain and a main-chain wallet.
type anchorFixture struct {
	id       SidechainID
	chain    *Sidechain
	identity *PeerIdentity
	wallet   *Wallet
	set      *UTXOSet
}

func newAnchorFixture(t *testing.T, blocks int) *anchorFixture {
	t.Helper()

	wallet := newTestWallet(t, "publisher", 0)
	set := NewUTXOSet()
	identity, id := registerPublisher(t, set.Anchors())

	// Fund the publisher so the anchor transaction's fee can be paid.
	genesis := utxoBlock(t, 0, "", mintTo(t, wallet, wallet, 100))
	if _, err := set.ApplyBlock(genesis, utxoTestSplit()); err != nil {
		t.Fatalf("apply genesis: %v", err)
	}

	return &anchorFixture{
		id:       id,
		chain:    buildChain(t, id, blocks),
		identity: identity,
		wallet:   wallet,
		set:      set,
	}
}

// signedAnchorTx builds the anchor transaction for the fixture's pending range.
func (f *anchorFixture) signedAnchorTx(t *testing.T) *AnchorTx {
	t.Helper()

	anchor, err := BuildAnchor(f.chain)
	if err != nil {
		t.Fatalf("BuildAnchor: %v", err)
	}
	if anchor == nil {
		t.Fatal("nothing to anchor")
	}

	signature, err := SignAnchor(f.identity, *anchor)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}

	tx, err := NewAnchorTransaction(f.wallet, *anchor, signature)
	if err != nil {
		t.Fatalf("NewAnchorTransaction: %v", err)
	}
	return tx
}

func TestAnchorTransactionReachesTheMainChain(t *testing.T) {
	f := newAnchorFixture(t, 3)
	tx := f.signedAnchorTx(t)

	block := utxoBlock(t, 1, "genesis", tx)
	if _, err := f.set.ApplyBlock(block, utxoTestSplit()); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// The commitment is now recorded by the main chain, which is what makes it
	// binding: rewriting the sidechain would require rewriting a mined block.
	latest, ok := f.set.Anchors().Latest(f.id)
	if !ok {
		t.Fatal("the main chain recorded no anchor for the sidechain")
	}
	if latest.FromHeight != 0 || latest.ToHeight != 2 {
		t.Fatalf("recorded %d..%d, want 0..2", latest.FromHeight, latest.ToHeight)
	}
	if !bytes.Equal(latest.TipHash, mustBlockHash(t, f.chain, 2)) {
		t.Fatal("the recorded tip hash is not the sidechain's tip")
	}
}

func TestAnchorTransactionFromAnUnregisteredPublisherIsRejected(t *testing.T) {
	f := newAnchorFixture(t, 2)

	// A publisher who never registered cannot commit anything -- otherwise a
	// sidechain id could be squatted by whoever anchors it first.
	other, err := NewSidechainID(99, 1)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}
	foreignChain := buildChain(t, other, 2)
	anchor, err := BuildAnchor(foreignChain)
	if err != nil {
		t.Fatalf("BuildAnchor: %v", err)
	}
	signature, err := SignAnchor(f.identity, *anchor)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}
	tx, err := NewAnchorTransaction(f.wallet, *anchor, signature)
	if err != nil {
		t.Fatalf("NewAnchorTransaction: %v", err)
	}

	block := utxoBlock(t, 1, "genesis", tx)
	if _, err := f.set.ApplyBlock(block, utxoTestSplit()); !errors.Is(err, ErrAnchorUnauthorised) {
		t.Fatalf("expected ErrAnchorUnauthorised, got %v", err)
	}
	if _, ok := f.set.Anchors().Latest(other); ok {
		t.Fatal("a rejected anchor was recorded anyway")
	}
}

func TestRejectedAnchorLeavesNoFeeCharged(t *testing.T) {
	f := newAnchorFixture(t, 2)
	before := f.set.BalanceUnits(f.wallet.GetAddress())

	// An anchor that does not start at height 0 fails contiguity. The whole
	// transaction must have no effect, fee included: charging for a commitment
	// that was not made would be a way to drain a publisher by replaying bad
	// anchors.
	anchor := anchorOver(t, f.chain, 1, 1)
	signature, err := SignAnchor(f.identity, anchor)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}
	tx, err := NewAnchorTransaction(f.wallet, anchor, signature)
	if err != nil {
		t.Fatalf("NewAnchorTransaction: %v", err)
	}

	block := utxoBlock(t, 1, "genesis", tx)
	if _, err := f.set.ApplyBlock(block, utxoTestSplit()); !errors.Is(err, ErrAnchorNotContiguous) {
		t.Fatalf("expected ErrAnchorNotContiguous, got %v", err)
	}
	if after := f.set.BalanceUnits(f.wallet.GetAddress()); after != before {
		t.Fatalf("a rejected anchor changed the balance: %d -> %d", before, after)
	}
}

func TestRevertingABlockRollsBackItsAnchor(t *testing.T) {
	f := newAnchorFixture(t, 2)

	first := f.signedAnchorTx(t)
	blockOne := utxoBlock(t, 1, "genesis", first)
	if _, err := f.set.ApplyBlock(blockOne, utxoTestSplit()); err != nil {
		t.Fatalf("apply first: %v", err)
	}
	f.chain.markAnchored(first.Anchor.ToHeight)

	for i := 0; i < 2; i++ {
		block, err := NewSidechainBlock(f.id, f.chain.Tip(), [][]byte{[]byte("more")}, int64(500+i))
		if err != nil {
			t.Fatalf("NewSidechainBlock: %v", err)
		}
		if err := f.chain.Append(block); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	second := f.signedAnchorTx(t)
	blockTwo := utxoBlock(t, 2, blockOne.Hash, second)
	if _, err := f.set.ApplyBlock(blockTwo, utxoTestSplit()); err != nil {
		t.Fatalf("apply second: %v", err)
	}

	latest, _ := f.set.Anchors().Latest(f.id)
	if latest.ToHeight != 3 {
		t.Fatalf("after two anchors the ledger is at %d, want 3", latest.ToHeight)
	}

	// A reorg drops the second block. The commitment it carried must stop
	// counting, or the publisher's next anchor is measured against a range no
	// block on the chain ever committed -- and fails contiguity forever.
	if err := f.set.RevertBlock(blockTwo.Hash); err != nil {
		t.Fatalf("revert: %v", err)
	}

	latest, ok := f.set.Anchors().Latest(f.id)
	if !ok {
		t.Fatal("reverting the second block erased the first block's anchor too")
	}
	if latest.ToHeight != 1 {
		t.Fatalf("after the revert the ledger is at %d, want 1", latest.ToHeight)
	}

	// Re-anchoring the same range on the new branch must now succeed.
	replacement := f.signedAnchorTx(t)
	blockTwoPrime := utxoBlock(t, 2, blockOne.Hash, replacement)
	if _, err := f.set.ApplyBlock(blockTwoPrime, utxoTestSplit()); err != nil {
		t.Fatalf("re-anchoring on the new branch failed: %v", err)
	}

	// And reverting the first block must leave nothing behind.
	if err := f.set.RevertBlock(blockTwoPrime.Hash); err != nil {
		t.Fatalf("revert: %v", err)
	}
	if err := f.set.RevertBlock(blockOne.Hash); err != nil {
		t.Fatalf("revert: %v", err)
	}
	if _, ok := f.set.Anchors().Latest(f.id); ok {
		t.Fatal("reverting every anchoring block left an anchor recorded")
	}
}

func TestAnchorTransactionWireFormatCarriesEveryField(t *testing.T) {
	f := newAnchorFixture(t, 3)
	tx := f.signedAnchorTx(t)

	if _, err := tx.Sign([]byte(f.wallet.PrivatePEM())); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	encoded, err := EncodeTransaction(tx)
	if err != nil {
		t.Fatalf("EncodeTransaction: %v", err)
	}

	// Path one: the generic decoder used when reading blocks back from disk.
	decoded, err := DecodeTransaction(encoded)
	if err != nil {
		t.Fatalf("DecodeTransaction: %v", err)
	}
	viaDecoder, ok := decoded.(*AnchorTx)
	if !ok {
		t.Fatalf("DecodeTransaction returned %T, want *AnchorTx", decoded)
	}

	// Path two: the type's own UnmarshalJSON. These are two decoders for the
	// same bytes; when the coinbase's two drifted, reloaded blocks paid nothing
	// and were rejected on restart.
	var viaUnmarshal AnchorTx
	if err := json.Unmarshal(encoded, &viaUnmarshal); err != nil {
		t.Fatalf("UnmarshalJSON: %v", err)
	}

	for name, got := range map[string]*AnchorTx{"decoder": viaDecoder, "unmarshal": &viaUnmarshal} {
		t.Run(name, func(t *testing.T) {
			if got.Anchor.PublisherID != tx.Anchor.PublisherID ||
				got.Anchor.GameID != tx.Anchor.GameID ||
				got.Anchor.FromHeight != tx.Anchor.FromHeight ||
				got.Anchor.ToHeight != tx.Anchor.ToHeight ||
				got.Anchor.PayloadCount != tx.Anchor.PayloadCount {
				t.Fatalf("anchor fields did not survive the round trip: %+v", got.Anchor)
			}
			if !bytes.Equal(got.Anchor.TipHash, tx.Anchor.TipHash) {
				t.Fatal("tip hash did not survive the round trip")
			}
			if !bytes.Equal(got.AnchorSignature, tx.AnchorSignature) {
				t.Fatal("publisher signature did not survive the round trip")
			}
			if got.GetSignature() != tx.GetSignature() {
				t.Fatal("transaction signature did not survive the round trip")
			}
		})
	}
}

func TestAnchorTransactionSignatureCoversTheAnchor(t *testing.T) {
	f := newAnchorFixture(t, 3)
	tx := f.signedAnchorTx(t)

	signature, err := tx.Sign([]byte(f.wallet.PrivatePEM()))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	publicKey := []byte(f.wallet.PublicPEM())

	valid, err := tx.Verify(publicKey, signature)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if !valid {
		t.Fatal("a freshly signed anchor transaction did not verify")
	}

	// Editing the committed range after signing must invalidate the signature,
	// or a relaying node could redirect the commitment in transit.
	tx.Anchor.ToHeight = 1
	valid, err = tx.Verify(publicKey, signature)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if valid {
		t.Fatal("the anchor range is not covered by the transaction signature")
	}
}
