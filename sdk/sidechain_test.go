// Package sdk is a software development kit for building blockchain applications.
// File sdk/sidechain_test.go - Tests for sidechain block space and anchoring.
package sdk

import (
	"bytes"
	"errors"
	"testing"
)

// buildChain returns a sidechain carrying `count` blocks of trivial payloads.
func buildChain(t *testing.T, id SidechainID, count int) *Sidechain {
	t.Helper()

	chain, err := NewSidechain(id)
	if err != nil {
		t.Fatalf("NewSidechain: %v", err)
	}

	for i := 0; i < count; i++ {
		block, err := NewSidechainBlock(id, chain.Tip(),
			[][]byte{[]byte{byte(i)}}, int64(1_700_000_000+i))
		if err != nil {
			t.Fatalf("NewSidechainBlock at %d: %v", i, err)
		}
		if err := chain.Append(block); err != nil {
			t.Fatalf("Append at %d: %v", i, err)
		}
	}
	return chain
}

func TestSidechainIDRejectsZeroComponents(t *testing.T) {
	cases := []struct {
		name      string
		publisher uint64
		game      uint64
	}{
		{"both zero", 0, 0},
		{"zero publisher", 0, 7},
		{"zero game", 7, 0},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NewSidechainID(tc.publisher, tc.game); !errors.Is(err, ErrInvalidSidechainID) {
				t.Fatalf("expected ErrInvalidSidechainID, got %v", err)
			}
		})
	}

	// Zero is refused because an omitted protobuf field decodes to zero. If it
	// were a legal id, a message that simply left the field out would address a
	// real sidechain.
	id, err := NewSidechainID(1, 1)
	if err != nil {
		t.Fatalf("NewSidechainID(1,1): %v", err)
	}
	if id.IsZero() {
		t.Fatal("a fully populated id reported itself as zero")
	}
}

func TestSidechainIDBytesRoundTrip(t *testing.T) {
	id, err := NewSidechainID(0xDEADBEEF, 0x0102030405060708)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}

	encoded := id.Bytes()
	if len(encoded) != 16 {
		t.Fatalf("expected a 16-byte encoding, got %d", len(encoded))
	}

	decoded, err := SidechainIDFromBytes(encoded)
	if err != nil {
		t.Fatalf("SidechainIDFromBytes: %v", err)
	}
	if !decoded.Equal(id) {
		t.Fatalf("round trip changed the id: %s -> %s", id, decoded)
	}
}

func TestSidechainRejectsAnotherPublishersBlock(t *testing.T) {
	mine, err := NewSidechainID(1, 1)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}
	theirs, err := NewSidechainID(2, 1)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}

	foreign, err := NewSidechainBlock(theirs, nil, [][]byte{[]byte("intruder")}, 1)
	if err != nil {
		t.Fatalf("NewSidechainBlock: %v", err)
	}

	// The empty chain is the case that matters most. A foreign block at height
	// zero has no tip to contradict, so contiguity has nothing to say about it:
	// the identity check in Append is the only thing standing between publisher
	// 2 and the first block of publisher 1's history.
	empty, err := NewSidechain(mine)
	if err != nil {
		t.Fatalf("NewSidechain: %v", err)
	}
	if err := empty.Append(foreign); !errors.Is(err, ErrSidechainMismatch) {
		t.Fatalf("a foreign genesis was accepted into an empty chain: %v", err)
	}
	if empty.Len() != 0 {
		t.Fatalf("the rejected block was stored anyway: length %d", empty.Len())
	}

	// And once the chain has a tip, extending it with a foreign block is refused
	// as well.
	chain := buildChain(t, mine, 1)
	successor, err := NewSidechainBlock(theirs, nil, [][]byte{[]byte("intruder")}, 2)
	if err != nil {
		t.Fatalf("NewSidechainBlock: %v", err)
	}
	successor.Header.Height = 1
	successor.Header.PreviousHash = mustBlockHash(t, chain, 0)
	successor.Hash = successor.ComputeHash()

	if err := chain.Append(successor); !errors.Is(err, ErrSidechainMismatch) {
		t.Fatalf("expected ErrSidechainMismatch, got %v", err)
	}
	if chain.Len() != 1 {
		t.Fatalf("the rejected block was stored anyway: length %d", chain.Len())
	}
}

func TestSidechainRejectsNonContiguousBlock(t *testing.T) {
	id, err := NewSidechainID(1, 1)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}
	chain := buildChain(t, id, 3)

	// A block built on height 0 while the tip is at height 2.
	stale, ok := chain.BlockAt(0)
	if !ok {
		t.Fatal("no block at height 0")
	}
	fork, err := NewSidechainBlock(id, stale, [][]byte{[]byte("fork")}, 99)
	if err != nil {
		t.Fatalf("NewSidechainBlock: %v", err)
	}

	if err := chain.Append(fork); !errors.Is(err, ErrSidechainNotContiguous) {
		t.Fatalf("expected ErrSidechainNotContiguous, got %v", err)
	}
}

func TestSidechainsAreIndependentBlockSpaces(t *testing.T) {
	set := NewSidechainSet()

	first, err := NewSidechainID(1, 1)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}
	// Same publisher, different game: still a separate space.
	second, err := NewSidechainID(1, 2)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}

	chainA, err := set.GetOrCreate(first)
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	chainB, err := set.GetOrCreate(second)
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}

	for i := 0; i < 4; i++ {
		block, err := NewSidechainBlock(first, chainA.Tip(), [][]byte{[]byte("a")}, int64(i))
		if err != nil {
			t.Fatalf("NewSidechainBlock: %v", err)
		}
		if err := chainA.Append(block); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	block, err := NewSidechainBlock(second, nil, [][]byte{[]byte("b")}, 0)
	if err != nil {
		t.Fatalf("NewSidechainBlock: %v", err)
	}
	if err := chainB.Append(block); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// Heights advance independently -- one publisher's activity does not move
	// another's chain, which is the whole point of separate block space.
	heightA, ok := chainA.Height()
	if !ok || heightA != 3 {
		t.Fatalf("chain A height = %d (%v), want 3", heightA, ok)
	}
	heightB, ok := chainB.Height()
	if !ok || heightB != 0 {
		t.Fatalf("chain B height = %d (%v), want 0", heightB, ok)
	}

	// GetOrCreate must return the same instance, not a fresh empty chain.
	again, err := set.GetOrCreate(first)
	if err != nil {
		t.Fatalf("GetOrCreate: %v", err)
	}
	if again.Len() != 4 {
		t.Fatalf("GetOrCreate returned a chain with %d blocks, want the existing 4", again.Len())
	}
	if len(set.IDs()) != 2 {
		t.Fatalf("set holds %d sidechains, want 2", len(set.IDs()))
	}
}

func TestSidechainBlockHashCoversEveryHeaderField(t *testing.T) {
	id, err := NewSidechainID(3, 4)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}
	base, err := NewSidechainBlock(id, nil, [][]byte{[]byte("x"), []byte("y")}, 1234)
	if err != nil {
		t.Fatalf("NewSidechainBlock: %v", err)
	}

	mutations := map[string]func(*SidechainBlockHeader){
		"version":   func(h *SidechainBlockHeader) { h.Version++ },
		"publisher": func(h *SidechainBlockHeader) { h.PublisherID++ },
		"game":      func(h *SidechainBlockHeader) { h.GameID++ },
		"height":    func(h *SidechainBlockHeader) { h.Height++ },
		"timestamp": func(h *SidechainBlockHeader) { h.TimestampUnixNano++ },
		"count":     func(h *SidechainBlockHeader) { h.PayloadCount++ },
		"root":      func(h *SidechainBlockHeader) { h.PayloadRoot[0] ^= 0xFF },
	}

	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			altered := *base
			altered.Header = base.Header
			altered.Header.PayloadRoot = append([]byte{}, base.Header.PayloadRoot...)
			mutate(&altered.Header)

			if bytes.Equal(altered.ComputeHash(), base.Hash) {
				t.Fatalf("changing %s did not change the hash; it is not committed", name)
			}
		})
	}
}

func TestSidechainPayloadRootDetectsReordering(t *testing.T) {
	id, err := NewSidechainID(1, 1)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}

	forward, err := NewSidechainBlock(id, nil, [][]byte{[]byte("a"), []byte("b")}, 1)
	if err != nil {
		t.Fatalf("NewSidechainBlock: %v", err)
	}
	reversed, err := NewSidechainBlock(id, nil, [][]byte{[]byte("b"), []byte("a")}, 1)
	if err != nil {
		t.Fatalf("NewSidechainBlock: %v", err)
	}

	// Order matters: a leaderboard that awards first place to whoever appears
	// first cannot have its payload order silently swapped.
	if bytes.Equal(forward.Header.PayloadRoot, reversed.Header.PayloadRoot) {
		t.Fatal("reordering the payloads produced the same root")
	}
}

// newRegisteredPublisher returns an identity registered in the ledger.
func newRegisteredPublisher(t *testing.T, ledger *AnchorLedger, publisherID uint64) *PeerIdentity {
	t.Helper()

	identity, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("NewPeerIdentity: %v", err)
	}
	if err := ledger.Registry().Register(publisherID, identity.PublicPEM); err != nil {
		t.Fatalf("Register: %v", err)
	}
	return identity
}

func TestAnchorCommitsThenAdvancesTheWatermark(t *testing.T) {
	id, err := NewSidechainID(1, 1)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}

	ledger := NewAnchorLedger(NewPublisherRegistry())
	identity := newRegisteredPublisher(t, ledger, id.PublisherID)
	chain := buildChain(t, id, 3)

	anchor, err := AnchorSidechain(chain, identity, ledger)
	if err != nil {
		t.Fatalf("AnchorSidechain: %v", err)
	}
	if anchor == nil {
		t.Fatal("a chain with three blocks had nothing to anchor")
	}
	if anchor.FromHeight != 0 || anchor.ToHeight != 2 {
		t.Fatalf("anchored %d..%d, want 0..2", anchor.FromHeight, anchor.ToHeight)
	}
	if anchor.PayloadCount != 3 {
		t.Fatalf("anchor reports %d payloads, want 3", anchor.PayloadCount)
	}

	// A second call with no new blocks must be a no-op rather than a
	// zero-length anchor, or the ledger fills with duplicates on every tick.
	repeat, err := AnchorSidechain(chain, identity, ledger)
	if err != nil {
		t.Fatalf("second AnchorSidechain: %v", err)
	}
	if repeat != nil {
		t.Fatalf("re-anchored an unchanged chain: %d..%d", repeat.FromHeight, repeat.ToHeight)
	}

	// New blocks continue from where the last anchor stopped.
	for i := 0; i < 2; i++ {
		block, err := NewSidechainBlock(id, chain.Tip(), [][]byte{[]byte("more")}, int64(9000+i))
		if err != nil {
			t.Fatalf("NewSidechainBlock: %v", err)
		}
		if err := chain.Append(block); err != nil {
			t.Fatalf("Append: %v", err)
		}
	}

	next, err := AnchorSidechain(chain, identity, ledger)
	if err != nil {
		t.Fatalf("third AnchorSidechain: %v", err)
	}
	if next.FromHeight != 3 || next.ToHeight != 4 {
		t.Fatalf("anchored %d..%d, want 3..4", next.FromHeight, next.ToHeight)
	}

	latest, ok := ledger.Latest(id)
	if !ok {
		t.Fatal("the ledger has no anchor for the sidechain")
	}
	if latest.ToHeight != 4 {
		t.Fatalf("ledger latest is %d, want 4", latest.ToHeight)
	}
}

func TestAnchorDetectsRewrittenHistory(t *testing.T) {
	id, err := NewSidechainID(1, 1)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}

	ledger := NewAnchorLedger(NewPublisherRegistry())
	identity := newRegisteredPublisher(t, ledger, id.PublisherID)
	chain := buildChain(t, id, 4)

	anchor, err := AnchorSidechain(chain, identity, ledger)
	if err != nil {
		t.Fatalf("AnchorSidechain: %v", err)
	}
	if err := VerifyAgainstChain(*anchor, chain); err != nil {
		t.Fatalf("the anchored chain failed its own verification: %v", err)
	}

	// This is the guarantee the whole mechanism exists for. Rewrite a payload in
	// the middle of already-anchored history -- an achievement retroactively
	// revoked, a leaderboard entry altered -- and the commitment must no longer
	// match.
	victim, ok := chain.BlockAt(1)
	if !ok {
		t.Fatal("no block at height 1")
	}
	victim.Payloads[0] = []byte("rewritten")
	victim.Header.PayloadRoot = sidechainPayloadRoot(victim.Payloads)
	victim.Hash = victim.ComputeHash()

	if err := VerifyAgainstChain(*anchor, chain); err == nil {
		t.Fatal("rewriting anchored history went undetected")
	} else if !errors.Is(err, ErrInvalidAnchor) {
		t.Fatalf("expected ErrInvalidAnchor, got %v", err)
	}
}

func TestAnchorRequiresContiguity(t *testing.T) {
	id, err := NewSidechainID(1, 1)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}

	ledger := NewAnchorLedger(NewPublisherRegistry())
	identity := newRegisteredPublisher(t, ledger, id.PublisherID)
	chain := buildChain(t, id, 5)

	// The first anchor must start at height 0; starting later would leave the
	// beginning of the history permanently uncommitted.
	gapFirst := SidechainAnchor{
		PublisherID: id.PublisherID, GameID: id.GameID,
		FromHeight: 2, ToHeight: 4,
		TipHash: mustBlockHash(t, chain, 4),
	}
	signature, err := SignAnchor(identity, gapFirst)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}
	if err := ledger.Record(gapFirst, signature); !errors.Is(err, ErrAnchorNotContiguous) {
		t.Fatalf("expected ErrAnchorNotContiguous for a late first anchor, got %v", err)
	}

	good := SidechainAnchor{
		PublisherID: id.PublisherID, GameID: id.GameID,
		FromHeight: 0, ToHeight: 2,
		TipHash: mustBlockHash(t, chain, 2),
	}
	signature, err = SignAnchor(identity, good)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}
	if err := ledger.Record(good, signature); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// Re-anchoring a range already committed is the rewrite path: it would let a
	// publisher replace 1..2 with different contents under a fresh commitment.
	overlap := SidechainAnchor{
		PublisherID: id.PublisherID, GameID: id.GameID,
		FromHeight: 1, ToHeight: 4,
		TipHash: mustBlockHash(t, chain, 4),
	}
	signature, err = SignAnchor(identity, overlap)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}
	if err := ledger.Record(overlap, signature); !errors.Is(err, ErrAnchorNotContiguous) {
		t.Fatalf("expected ErrAnchorNotContiguous for an overlapping anchor, got %v", err)
	}

	// A gap would leave heights 3 uncommitted forever.
	skip := SidechainAnchor{
		PublisherID: id.PublisherID, GameID: id.GameID,
		FromHeight: 4, ToHeight: 4,
		TipHash: mustBlockHash(t, chain, 4),
	}
	signature, err = SignAnchor(identity, skip)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}
	if err := ledger.Record(skip, signature); !errors.Is(err, ErrAnchorNotContiguous) {
		t.Fatalf("expected ErrAnchorNotContiguous for a gap, got %v", err)
	}
}

func mustBlockHash(t *testing.T, chain *Sidechain, height uint64) []byte {
	t.Helper()
	block, ok := chain.BlockAt(height)
	if !ok {
		t.Fatalf("no block at height %d", height)
	}
	return append([]byte{}, block.Hash...)
}

func TestAnchorRejectsUnauthorisedPublishers(t *testing.T) {
	id, err := NewSidechainID(1, 1)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}

	ledger := NewAnchorLedger(NewPublisherRegistry())
	chain := buildChain(t, id, 2)

	anchor := SidechainAnchor{
		PublisherID: id.PublisherID, GameID: id.GameID,
		FromHeight: 0, ToHeight: 1,
		TipHash: mustBlockHash(t, chain, 1),
	}

	stranger, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("NewPeerIdentity: %v", err)
	}
	signature, err := SignAnchor(stranger, anchor)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}

	// Nothing is registered yet: an unknown publisher cannot anchor at all.
	if err := ledger.Record(anchor, signature); !errors.Is(err, ErrAnchorUnauthorised) {
		t.Fatalf("expected ErrAnchorUnauthorised for an unregistered publisher, got %v", err)
	}

	owner := newRegisteredPublisher(t, ledger, id.PublisherID)

	// Now registered -- but the signature belongs to somebody else. This is the
	// takeover case: another party committing a history under this publisher's id.
	if err := ledger.Record(anchor, signature); !errors.Is(err, ErrAnchorUnauthorised) {
		t.Fatalf("expected ErrAnchorUnauthorised for a foreign signature, got %v", err)
	}

	if err := ledger.Record(anchor, nil); !errors.Is(err, ErrAnchorUnauthorised) {
		t.Fatalf("expected ErrAnchorUnauthorised for an unsigned anchor, got %v", err)
	}

	signature, err = SignAnchor(owner, anchor)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}
	if err := ledger.Record(anchor, signature); err != nil {
		t.Fatalf("the owner's own anchor was rejected: %v", err)
	}
}

func TestAnchorSignatureIsBoundToItsRange(t *testing.T) {
	id, err := NewSidechainID(1, 1)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}

	ledger := NewAnchorLedger(NewPublisherRegistry())
	identity := newRegisteredPublisher(t, ledger, id.PublisherID)
	chain := buildChain(t, id, 3)

	anchor := SidechainAnchor{
		PublisherID: id.PublisherID, GameID: id.GameID,
		FromHeight: 0, ToHeight: 2,
		TipHash: mustBlockHash(t, chain, 2), PayloadCount: 3,
	}
	signature, err := SignAnchor(identity, anchor)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}

	// Every field the anchor asserts must be covered. If the range were outside
	// the signature, one captured signature could be replayed to commit a
	// different span of history under the same authorisation.
	mutations := map[string]func(*SidechainAnchor){
		"from height":   func(a *SidechainAnchor) { a.FromHeight = 1 },
		"to height":     func(a *SidechainAnchor) { a.ToHeight = 1 },
		"game id":       func(a *SidechainAnchor) { a.GameID = 2 },
		"tip hash":      func(a *SidechainAnchor) { a.TipHash[0] ^= 0xFF },
		"payload count": func(a *SidechainAnchor) { a.PayloadCount = 99 },
	}

	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			altered := anchor
			altered.TipHash = append([]byte{}, anchor.TipHash...)
			mutate(&altered)

			if err := ledger.Verify(altered, signature); err == nil {
				t.Fatalf("changing the %s left the signature valid", name)
			}
		})
	}
}

func TestPublisherRegistryRefusesTakeover(t *testing.T) {
	registry := NewPublisherRegistry()

	owner, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("NewPeerIdentity: %v", err)
	}
	if err := registry.Register(7, owner.PublicPEM); err != nil {
		t.Fatalf("Register: %v", err)
	}

	// Re-registering the same key is a harmless retry.
	if err := registry.Register(7, owner.PublicPEM); err != nil {
		t.Fatalf("re-registering an identical key failed: %v", err)
	}

	attacker, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("NewPeerIdentity: %v", err)
	}
	if err := registry.Register(7, attacker.PublicPEM); err == nil {
		t.Fatal("a second key claimed an already-registered publisher id")
	}

	stored, ok := registry.PublicKey(7)
	if !ok {
		t.Fatal("publisher 7 is not registered")
	}
	if stored != owner.PublicPEM {
		t.Fatal("the registered key was replaced by the attacker's")
	}
	if err := registry.Register(0, owner.PublicPEM); !errors.Is(err, ErrInvalidSidechainID) {
		t.Fatalf("expected ErrInvalidSidechainID for publisher 0, got %v", err)
	}
}

func TestVerifyAgainstChainRejectsTheWrongSidechain(t *testing.T) {
	mine, err := NewSidechainID(1, 1)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}
	theirs, err := NewSidechainID(2, 2)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}

	chain := buildChain(t, mine, 2)
	other := buildChain(t, theirs, 2)

	anchor := SidechainAnchor{
		PublisherID: mine.PublisherID, GameID: mine.GameID,
		FromHeight: 0, ToHeight: 1,
		TipHash: mustBlockHash(t, chain, 1),
	}

	if err := VerifyAgainstChain(anchor, other); !errors.Is(err, ErrSidechainMismatch) {
		t.Fatalf("expected ErrSidechainMismatch, got %v", err)
	}
}
