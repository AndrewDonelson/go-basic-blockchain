// Package sdk is a software development kit for building blockchain applications.
// File sdk/availability_test.go - Tests for bonded availability challenges.
package sdk

import (
	"bytes"
	"errors"
	"testing"
)

// availabilityFixture is a registered publisher with an anchored sidechain and a
// challenger holding funds.
type availabilityFixture struct {
	id         SidechainID
	chain      *Sidechain
	identity   *PeerIdentity
	publisher  *Wallet
	challenger *Wallet
	set        *UTXOSet
	anchor     SidechainAnchor
	nextIndex  int64
}

func newAvailabilityFixture(t *testing.T) *availabilityFixture {
	t.Helper()

	publisher := newTestWallet(t, "publisher", 0)
	challenger := newTestWallet(t, "challenger", 0)
	set := NewUTXOSet()

	identity, id := registerPublisher(t, set.Anchors())

	// Fund both parties, and the bond address so a slash has coins to move.
	genesis := utxoBlock(t, 0, "",
		mintTo(t, publisher, publisher, 1000),
		mintTo(t, challenger, challenger, 1000),
		mintTo(t, publisher, addressOnlyWallet(DerivePublisherBondAddress(id.PublisherID)), 1000),
	)
	if _, err := set.ApplyBlock(genesis, utxoTestSplit()); err != nil {
		t.Fatalf("apply genesis: %v", err)
	}

	chain := buildChain(t, id, 4)

	anchor, err := BuildAnchor(chain)
	if err != nil {
		t.Fatalf("BuildAnchor: %v", err)
	}
	signature, err := SignAnchor(identity, *anchor)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}
	anchorTx, err := NewAnchorTransaction(publisher, *anchor, signature)
	if err != nil {
		t.Fatalf("NewAnchorTransaction: %v", err)
	}

	block := utxoBlock(t, 1, genesis.Hash, anchorTx)
	if _, err := set.ApplyBlock(block, utxoTestSplit()); err != nil {
		t.Fatalf("apply anchor: %v", err)
	}
	chain.markAnchored(anchor.ToHeight)

	return &availabilityFixture{
		id: id, chain: chain, identity: identity,
		publisher: publisher, challenger: challenger,
		set: set, anchor: *anchor, nextIndex: 2,
	}
}

// applyWith mines a block at the fixture's next main-chain height.
func (f *availabilityFixture) applyWith(t *testing.T, txs ...Transaction) error {
	t.Helper()
	block := utxoBlock(t, f.nextIndex, "prev", txs...)
	f.nextIndex++
	_, err := f.set.ApplyBlock(block, utxoTestSplit())
	return err
}

// applyAt mines an empty block at an explicit height, to advance time.
func (f *availabilityFixture) applyAt(t *testing.T, height int64) error {
	t.Helper()
	block := utxoBlock(t, height, "prev")
	f.nextIndex = height + 1
	_, err := f.set.ApplyBlock(block, utxoTestSplit())
	return err
}

func TestAvailabilityProofClearsAChallenge(t *testing.T) {
	f := newAvailabilityFixture(t)

	challengeTx, err := NewAvailabilityChallenge(f.challenger, f.id, 2, 0)
	if err != nil {
		t.Fatalf("NewAvailabilityChallenge: %v", err)
	}
	if err := f.applyWith(t, challengeTx); err != nil {
		t.Fatalf("apply challenge: %v", err)
	}
	if f.set.Availability().OpenCount() != 1 {
		t.Fatalf("expected one open challenge, got %d", f.set.Availability().OpenCount())
	}

	// The stake is held in escrow, not by either party.
	escrow := DeriveChallengeEscrowAddress(challengeTx.Challenge.ID())
	if got := f.set.BalanceUnits(escrow); got != AvailabilityChallengeFeeUnits {
		t.Fatalf("escrow holds %d units, want %d", got, AvailabilityChallengeFeeUnits)
	}

	proof, err := BuildProof(challengeTx.Challenge, f.chain, f.anchor)
	if err != nil {
		t.Fatalf("BuildProof: %v", err)
	}
	proofTx, err := NewAvailabilityProof(f.publisher, *proof)
	if err != nil {
		t.Fatalf("NewAvailabilityProof: %v", err)
	}

	before := f.set.BalanceUnits(f.publisher.GetAddress())
	if err := f.applyWith(t, proofTx); err != nil {
		t.Fatalf("apply proof: %v", err)
	}

	if f.set.Availability().OpenCount() != 0 {
		t.Fatal("an answered challenge is still open")
	}
	if f.set.Availability().Failures(f.id) != 0 {
		t.Fatal("answering a challenge recorded a failure")
	}
	// Answering pays the publisher the stake, less the fee for the transaction
	// carrying the answer. That net gain is what makes a frivolous challenge cost
	// the challenger rather than the publisher.
	expected := before + AvailabilityChallengeFeeUnits - AmountToUnits(proofTx.GetFee())
	if after := f.set.BalanceUnits(f.publisher.GetAddress()); after != expected {
		t.Fatalf("publisher balance went %d -> %d, want %d", before, after, expected)
	}
	if got := f.set.BalanceUnits(escrow); got != 0 {
		t.Fatalf("escrow still holds %d units", got)
	}
}

func TestUnansweredChallengeSlashesTheBond(t *testing.T) {
	f := newAvailabilityFixture(t)

	challengeTx, err := NewAvailabilityChallenge(f.challenger, f.id, 1, 0)
	if err != nil {
		t.Fatalf("NewAvailabilityChallenge: %v", err)
	}
	opened := f.nextIndex
	if err := f.applyWith(t, challengeTx); err != nil {
		t.Fatalf("apply challenge: %v", err)
	}

	bondAddress := DerivePublisherBondAddress(f.id.PublisherID)
	bondBefore := f.set.BalanceUnits(bondAddress)
	challengerBefore := f.set.BalanceUnits(f.challenger.GetAddress())

	// One block before the deadline nothing happens: the publisher still has
	// time, and slashing early would punish a slow answer rather than a missing
	// one.
	if err := f.applyAt(t, opened+AvailabilityResponseWindow); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if f.set.Availability().OpenCount() != 1 {
		t.Fatal("the challenge expired before its deadline")
	}

	// One block past it, the sweep runs without anyone having to claim it.
	if err := f.applyAt(t, opened+AvailabilityResponseWindow+1); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if f.set.Availability().OpenCount() != 0 {
		t.Fatal("an expired challenge is still open")
	}
	if f.set.Availability().Failures(f.id) != 1 {
		t.Fatalf("failures = %d, want 1", f.set.Availability().Failures(f.id))
	}

	if bondAfter := f.set.BalanceUnits(bondAddress); bondAfter != bondBefore-AvailabilitySlashUnits {
		t.Fatalf("bond went %d -> %d, expected a loss of %d",
			bondBefore, bondAfter, AvailabilitySlashUnits)
	}
	// The challenger gets their stake back plus the slash: they asked a fair
	// question and were not answered.
	expected := challengerBefore + AvailabilityChallengeFeeUnits + AvailabilitySlashUnits
	if got := f.set.BalanceUnits(f.challenger.GetAddress()); got != expected {
		t.Fatalf("challenger balance is %d, want %d", got, expected)
	}

	record, ok := f.set.Anchors().Registry().Publisher(f.id.PublisherID)
	if !ok {
		t.Fatal("publisher vanished")
	}
	if record.BondUnits != MinPublisherBondUnits-AvailabilitySlashUnits {
		t.Fatalf("recorded bond is %d, want %d",
			record.BondUnits, MinPublisherBondUnits-AvailabilitySlashUnits)
	}
}

func TestProofOnTheDeadlineBlockStillCounts(t *testing.T) {
	f := newAvailabilityFixture(t)

	challengeTx, err := NewAvailabilityChallenge(f.challenger, f.id, 3, 0)
	if err != nil {
		t.Fatalf("NewAvailabilityChallenge: %v", err)
	}
	opened := f.nextIndex
	if err := f.applyWith(t, challengeTx); err != nil {
		t.Fatalf("apply challenge: %v", err)
	}

	proof, err := BuildProof(challengeTx.Challenge, f.chain, f.anchor)
	if err != nil {
		t.Fatalf("BuildProof: %v", err)
	}
	proofTx, err := NewAvailabilityProof(f.publisher, *proof)
	if err != nil {
		t.Fatalf("NewAvailabilityProof: %v", err)
	}

	// The proof rides in the very block that would otherwise expire it. Expiry
	// is swept after the block's transactions precisely so an ordering accident
	// cannot slash a publisher who answered in time.
	block := utxoBlock(t, opened+AvailabilityResponseWindow+1, "prev", proofTx)
	if _, err := f.set.ApplyBlock(block, utxoTestSplit()); err != nil {
		t.Fatalf("apply: %v", err)
	}

	if f.set.Availability().Failures(f.id) != 0 {
		t.Fatal("a publisher who answered on the deadline block was slashed")
	}
	if f.set.Availability().OpenCount() != 0 {
		t.Fatal("the challenge is still open")
	}
}

func TestProofMustMatchTheAnchoredHistory(t *testing.T) {
	f := newAvailabilityFixture(t)

	challengeTx, err := NewAvailabilityChallenge(f.challenger, f.id, 2, 0)
	if err != nil {
		t.Fatalf("NewAvailabilityChallenge: %v", err)
	}
	if err := f.applyWith(t, challengeTx); err != nil {
		t.Fatalf("apply challenge: %v", err)
	}

	good, err := BuildProof(challengeTx.Challenge, f.chain, f.anchor)
	if err != nil {
		t.Fatalf("BuildProof: %v", err)
	}

	cases := map[string]func(*AvailabilityProof){
		"a different payload": func(p *AvailabilityProof) {
			p.Payload = []byte("not what was committed")
		},
		"a tampered payload path": func(p *AvailabilityProof) {
			p.PayloadPath = append([][]byte{}, p.PayloadPath...)
			if len(p.PayloadPath) == 0 {
				p.PayloadPath = [][]byte{make([]byte, 32)}
				return
			}
			node := append([]byte{}, p.PayloadPath[0]...)
			node[0] ^= 0xFF
			p.PayloadPath[0] = node
		},
		"a rewritten header": func(p *AvailabilityProof) {
			p.Header.TimestampUnixNano++
		},
		"a header from the wrong height": func(p *AvailabilityProof) {
			p.Header.Height = 3
		},
		"a header from another sidechain": func(p *AvailabilityProof) {
			p.Header.GameID++
		},
	}

	for name, corrupt := range cases {
		t.Run(name, func(t *testing.T) {
			bad := *good
			bad.Payload = append([]byte{}, good.Payload...)
			bad.HeaderPath = append([][]byte{}, good.HeaderPath...)
			bad.PayloadPath = append([][]byte{}, good.PayloadPath...)
			corrupt(&bad)

			if err := f.set.Availability().VerifyProof(bad); err == nil {
				t.Fatalf("%s was accepted as a valid proof", name)
			}
		})
	}

	// The honest proof still passes, so the rejections above are about the
	// corruption rather than a broken verifier.
	if err := f.set.Availability().VerifyProof(*good); err != nil {
		t.Fatalf("the honest proof was rejected: %v", err)
	}
}

func TestChallengeMustNameAnchoredHistory(t *testing.T) {
	f := newAvailabilityFixture(t)

	// Height 4 exists on the sidechain but was never anchored. Challenging it
	// would demand a publisher produce something they have not committed to, and
	// there would be nothing to check the answer against.
	block, err := NewSidechainBlock(f.id, f.chain.Tip(), [][]byte{[]byte("fresh")}, 1)
	if err != nil {
		t.Fatalf("NewSidechainBlock: %v", err)
	}
	if err := f.chain.Append(block); err != nil {
		t.Fatalf("Append: %v", err)
	}

	challengeTx, err := NewAvailabilityChallenge(f.challenger, f.id, 4, 0)
	if err != nil {
		t.Fatalf("NewAvailabilityChallenge: %v", err)
	}
	if err := f.applyWith(t, challengeTx); !errors.Is(err, ErrChallengeInvalid) {
		t.Fatalf("expected ErrChallengeInvalid, got %v", err)
	}
}

func TestRevertingABlockReopensItsChallenge(t *testing.T) {
	f := newAvailabilityFixture(t)

	challengeTx, err := NewAvailabilityChallenge(f.challenger, f.id, 1, 0)
	if err != nil {
		t.Fatalf("NewAvailabilityChallenge: %v", err)
	}
	if err := f.applyWith(t, challengeTx); err != nil {
		t.Fatalf("apply challenge: %v", err)
	}

	proof, err := BuildProof(challengeTx.Challenge, f.chain, f.anchor)
	if err != nil {
		t.Fatalf("BuildProof: %v", err)
	}
	proofTx, err := NewAvailabilityProof(f.publisher, *proof)
	if err != nil {
		t.Fatalf("NewAvailabilityProof: %v", err)
	}

	answering := utxoBlock(t, f.nextIndex, "prev", proofTx)
	if _, err := f.set.ApplyBlock(answering, utxoTestSplit()); err != nil {
		t.Fatalf("apply proof: %v", err)
	}
	if f.set.Availability().OpenCount() != 0 {
		t.Fatal("the challenge did not close")
	}

	// A reorg drops the answering block. The obligation must come back: a
	// challenge that stayed closed would let a publisher clear it with a proof
	// no longer on the chain.
	if err := f.set.RevertBlock(answering.Hash); err != nil {
		t.Fatalf("revert: %v", err)
	}
	if f.set.Availability().OpenCount() != 1 {
		t.Fatal("reverting the answering block left the challenge closed")
	}
	if got := f.set.BalanceUnits(DeriveChallengeEscrowAddress(challengeTx.Challenge.ID())); got != AvailabilityChallengeFeeUnits {
		t.Fatalf("the stake did not return to escrow: %d units", got)
	}
}

func TestRevertingAnExpirySweepRestoresTheBond(t *testing.T) {
	f := newAvailabilityFixture(t)

	challengeTx, err := NewAvailabilityChallenge(f.challenger, f.id, 1, 0)
	if err != nil {
		t.Fatalf("NewAvailabilityChallenge: %v", err)
	}
	opened := f.nextIndex
	if err := f.applyWith(t, challengeTx); err != nil {
		t.Fatalf("apply challenge: %v", err)
	}

	bondAddress := DerivePublisherBondAddress(f.id.PublisherID)
	bondBefore := f.set.BalanceUnits(bondAddress)

	sweep := utxoBlock(t, opened+AvailabilityResponseWindow+1, "prev")
	if _, err := f.set.ApplyBlock(sweep, utxoTestSplit()); err != nil {
		t.Fatalf("apply sweep: %v", err)
	}
	if f.set.Availability().Failures(f.id) != 1 {
		t.Fatal("the sweep did not record a failure")
	}

	if err := f.set.RevertBlock(sweep.Hash); err != nil {
		t.Fatalf("revert: %v", err)
	}

	if f.set.Availability().Failures(f.id) != 0 {
		t.Fatal("reverting the sweep left the failure recorded")
	}
	if f.set.Availability().OpenCount() != 1 {
		t.Fatal("reverting the sweep left the challenge closed")
	}
	if got := f.set.BalanceUnits(bondAddress); got != bondBefore {
		t.Fatalf("bond is %d after the revert, want %d", got, bondBefore)
	}
	record, _ := f.set.Anchors().Registry().Publisher(f.id.PublisherID)
	if record.BondUnits != MinPublisherBondUnits {
		t.Fatalf("recorded bond is %d after the revert, want %d",
			record.BondUnits, MinPublisherBondUnits)
	}
}

func TestChallengeIDIsDerivedFromItsContents(t *testing.T) {
	base := AvailabilityChallenge{
		PublisherID: 1, GameID: 2, Height: 3, PayloadIndex: 4, Challenger: "alice",
	}

	// Two challengers asking the identical question collide by construction, so a
	// publisher cannot be made to answer the same thing twice in parallel.
	other := base
	other.Challenger = "bob"
	if base.ID() != other.ID() {
		t.Fatal("the challenger changed the challenge id; the same question must be one challenge")
	}

	for name, mutate := range map[string]func(*AvailabilityChallenge){
		"publisher": func(c *AvailabilityChallenge) { c.PublisherID++ },
		"game":      func(c *AvailabilityChallenge) { c.GameID++ },
		"height":    func(c *AvailabilityChallenge) { c.Height++ },
		"payload":   func(c *AvailabilityChallenge) { c.PayloadIndex++ },
	} {
		t.Run(name, func(t *testing.T) {
			altered := base
			mutate(&altered)
			if altered.ID() == base.ID() {
				t.Fatalf("changing the %s did not change the challenge id", name)
			}
		})
	}
}

func TestAvailabilityWireFormatCarriesEveryField(t *testing.T) {
	f := newAvailabilityFixture(t)

	challengeTx, err := NewAvailabilityChallenge(f.challenger, f.id, 2, 0)
	if err != nil {
		t.Fatalf("NewAvailabilityChallenge: %v", err)
	}
	proof, err := BuildProof(challengeTx.Challenge, f.chain, f.anchor)
	if err != nil {
		t.Fatalf("BuildProof: %v", err)
	}
	proofTx, err := NewAvailabilityProof(f.publisher, *proof)
	if err != nil {
		t.Fatalf("NewAvailabilityProof: %v", err)
	}

	for name, original := range map[string]*AvailabilityTx{
		"challenge": challengeTx,
		"proof":     proofTx,
	} {
		t.Run(name, func(t *testing.T) {
			encoded, err := EncodeTransaction(original)
			if err != nil {
				t.Fatalf("EncodeTransaction: %v", err)
			}
			decoded, err := DecodeTransaction(encoded)
			if err != nil {
				t.Fatalf("DecodeTransaction: %v", err)
			}
			got, ok := decoded.(*AvailabilityTx)
			if !ok {
				t.Fatalf("decoded to %T, want *AvailabilityTx", decoded)
			}

			if got.Kind != original.Kind {
				t.Fatalf("kind %d survived as %d", original.Kind, got.Kind)
			}
			if got.Challenge != original.Challenge {
				t.Fatalf("challenge did not survive: %+v", got.Challenge)
			}
			if got.Proof.ChallengeID != original.Proof.ChallengeID {
				t.Fatal("challenge id did not survive")
			}
			if !bytes.Equal(got.Proof.Payload, original.Proof.Payload) {
				t.Fatal("payload did not survive")
			}
			if len(got.Proof.HeaderPath) != len(original.Proof.HeaderPath) {
				t.Fatal("header path did not survive")
			}
			if len(got.Proof.PayloadPath) != len(original.Proof.PayloadPath) {
				t.Fatal("payload path did not survive")
			}
		})
	}
}
