// Package sdk is a software development kit for building blockchain applications.
// File sdk/registrationtx_test.go - Tests for on-chain publisher registration.
package sdk

import (
	"errors"
	"testing"
)

// registrationFixture is a funded wallet and an empty UTXO set.
type registrationFixture struct {
	wallet   *Wallet
	set      *UTXOSet
	genesis  *Block
	nextIdx  int64
	identity *PeerIdentity
}

func newRegistrationFixture(t *testing.T) *registrationFixture {
	t.Helper()

	wallet := newTestWallet(t, "registrant", 0)
	set := NewUTXOSet()
	genesis := utxoBlock(t, 0, "", mintTo(t, wallet, wallet, 10000))
	if _, err := set.ApplyBlock(genesis, utxoTestSplit()); err != nil {
		t.Fatalf("apply genesis: %v", err)
	}

	identity, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("NewPeerIdentity: %v", err)
	}

	return &registrationFixture{wallet: wallet, set: set, genesis: genesis, nextIdx: 1, identity: identity}
}

func (f *registrationFixture) apply(t *testing.T, txs ...Transaction) error {
	t.Helper()
	block := utxoBlock(t, f.nextIdx, "prev", txs...)
	f.nextIdx++
	_, err := f.set.ApplyBlock(block, utxoTestSplit())
	return err
}

func TestRegistrationAllocatesIDsThroughConsensus(t *testing.T) {
	f := newRegistrationFixture(t)

	registration, err := NewPublisherRegistration(f.wallet, f.identity, "Nlaak Studios", MinPublisherBondUnits)
	if err != nil {
		t.Fatalf("NewPublisherRegistration: %v", err)
	}
	// The transaction cannot name the id it will receive -- that is what stops
	// ids being squatted.
	if registration.PublisherID != 0 {
		t.Fatalf("a publisher registration carried id %d", registration.PublisherID)
	}

	if err := f.apply(t, registration); err != nil {
		t.Fatalf("apply registration: %v", err)
	}

	registry := f.set.Anchors().Registry()
	record, ok := registry.Publisher(1)
	if !ok {
		t.Fatal("no publisher was allocated id 1")
	}
	if record.PublicKeyPEM != f.identity.PublicPEM {
		t.Fatal("the wrong key was registered")
	}
	if record.BondUnits != MinPublisherBondUnits {
		t.Fatalf("bond recorded as %d, want %d", record.BondUnits, MinPublisherBondUnits)
	}

	// The bond moved to an address with no private key, so nothing but a
	// consensus rule can move it again.
	bondAddress := DerivePublisherBondAddress(1)
	if got := f.set.BalanceUnits(bondAddress); got != MinPublisherBondUnits {
		t.Fatalf("bond address holds %d units, want %d", got, MinPublisherBondUnits)
	}

	game, err := NewGameRegistration(f.wallet, f.identity, 1, "Space Game")
	if err != nil {
		t.Fatalf("NewGameRegistration: %v", err)
	}
	if err := f.apply(t, game); err != nil {
		t.Fatalf("apply game registration: %v", err)
	}

	record, _ = registry.Publisher(1)
	games := record.Games()
	if len(games) != 1 || games[0].GameID != 1 || games[0].Name != "Space Game" {
		t.Fatalf("game registration produced %+v", games)
	}
}

func TestGameRegistrationRequiresTheOwningPublisher(t *testing.T) {
	f := newRegistrationFixture(t)

	registration, err := NewPublisherRegistration(f.wallet, f.identity, "Owner", MinPublisherBondUnits)
	if err != nil {
		t.Fatalf("NewPublisherRegistration: %v", err)
	}
	if err := f.apply(t, registration); err != nil {
		t.Fatalf("apply: %v", err)
	}

	// Someone else's key signing a game registration inside publisher 1 is the
	// namespace-intrusion case.
	stranger, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("NewPeerIdentity: %v", err)
	}
	game, err := NewGameRegistration(f.wallet, stranger, 1, "Not Mine")
	if err != nil {
		t.Fatalf("NewGameRegistration: %v", err)
	}

	if err := f.apply(t, game); !errors.Is(err, ErrRegistrationInvalid) {
		t.Fatalf("expected ErrRegistrationInvalid, got %v", err)
	}

	record, _ := f.set.Anchors().Registry().Publisher(1)
	if len(record.Games()) != 0 {
		t.Fatal("a game was registered by the wrong key")
	}
}

func TestPublisherRegistrationProvesKeyPossession(t *testing.T) {
	f := newRegistrationFixture(t)

	victim, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("NewPeerIdentity: %v", err)
	}

	// Registering somebody else's public key would burn an id and create an
	// identity its supposed owner never asked for, so the registration must be
	// signed by the key it claims.
	registration, err := NewPublisherRegistration(f.wallet, f.identity, "Impostor", MinPublisherBondUnits)
	if err != nil {
		t.Fatalf("NewPublisherRegistration: %v", err)
	}
	registration.PublicKeyPEM = victim.PublicPEM

	if err := f.apply(t, registration); !errors.Is(err, ErrRegistrationInvalid) {
		t.Fatalf("expected ErrRegistrationInvalid, got %v", err)
	}
	if f.set.Anchors().Registry().Count() != 0 {
		t.Fatal("a registration for an unheld key was accepted")
	}
}

func TestRevertingARegistrationReleasesTheID(t *testing.T) {
	f := newRegistrationFixture(t)

	registration, err := NewPublisherRegistration(f.wallet, f.identity, "First", MinPublisherBondUnits)
	if err != nil {
		t.Fatalf("NewPublisherRegistration: %v", err)
	}

	block := utxoBlock(t, 1, "prev", registration)
	if _, err := f.set.ApplyBlock(block, utxoTestSplit()); err != nil {
		t.Fatalf("apply: %v", err)
	}

	registry := f.set.Anchors().Registry()
	if registry.Count() != 1 {
		t.Fatalf("expected one publisher, got %d", registry.Count())
	}

	// A reorg drops the registration. The id must be released: leaving it
	// allocated would hand the next registrant an id that a still-circulating
	// anchor refers to.
	if err := f.set.RevertBlock(block.Hash); err != nil {
		t.Fatalf("revert: %v", err)
	}
	if registry.Count() != 0 {
		t.Fatalf("reverting left %d publishers registered", registry.Count())
	}
	if got := f.set.BalanceUnits(DerivePublisherBondAddress(1)); got != 0 {
		t.Fatalf("the bond survived the revert: %d units", got)
	}

	// The same key can now register again, and receives id 1 once more.
	replacement, err := NewPublisherRegistration(f.wallet, f.identity, "First again", MinPublisherBondUnits)
	if err != nil {
		t.Fatalf("NewPublisherRegistration: %v", err)
	}
	replacementBlock := utxoBlock(t, 1, "prev", replacement)
	if _, err := f.set.ApplyBlock(replacementBlock, utxoTestSplit()); err != nil {
		t.Fatalf("re-registering after a revert failed: %v", err)
	}
	if _, ok := registry.Publisher(1); !ok {
		t.Fatal("the re-registration did not receive id 1")
	}
}

func TestRegistrationWireFormatCarriesEveryField(t *testing.T) {
	f := newRegistrationFixture(t)

	registration, err := NewPublisherRegistration(f.wallet, f.identity, "Wire Test", MinPublisherBondUnits)
	if err != nil {
		t.Fatalf("NewPublisherRegistration: %v", err)
	}
	if _, err := registration.Sign([]byte(f.wallet.PrivatePEM())); err != nil {
		t.Fatalf("Sign: %v", err)
	}

	encoded, err := EncodeTransaction(registration)
	if err != nil {
		t.Fatalf("EncodeTransaction: %v", err)
	}
	decoded, err := DecodeTransaction(encoded)
	if err != nil {
		t.Fatalf("DecodeTransaction: %v", err)
	}
	got, ok := decoded.(*RegistrationTx)
	if !ok {
		t.Fatalf("decoded to %T, want *RegistrationTx", decoded)
	}

	if got.Kind != registration.Kind || got.Name != registration.Name ||
		got.PublicKeyPEM != registration.PublicKeyPEM ||
		got.BondUnits != registration.BondUnits {
		t.Fatalf("registration fields did not survive the round trip: %+v", got)
	}

	// The registration signature must survive, or a reloaded block's
	// registration would be unverifiable and the whole block rejected on restart.
	if err := got.VerifyRegistrationSignature(NewPublisherRegistry()); err != nil {
		t.Fatalf("the decoded registration no longer verifies: %v", err)
	}
}

func TestRegistrationSignatureCoversTheBond(t *testing.T) {
	f := newRegistrationFixture(t)

	registration, err := NewPublisherRegistration(f.wallet, f.identity, "Bonded", MinPublisherBondUnits*2)
	if err != nil {
		t.Fatalf("NewPublisherRegistration: %v", err)
	}

	// Lowering the bond after signing must break the signature; otherwise a
	// relaying node could reduce what a publisher has at stake in transit.
	registration.BondUnits = MinPublisherBondUnits
	if err := registration.VerifyRegistrationSignature(NewPublisherRegistry()); err == nil {
		t.Fatal("the bond is not covered by the registration signature")
	}
}
