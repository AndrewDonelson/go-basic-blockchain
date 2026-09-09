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

// anchorOver builds a well-formed anchor for an explicit range, so a test can
// choose a range BuildAnchor would never produce.
func anchorOver(t *testing.T, chain *Sidechain, from, to uint64) SidechainAnchor {
	t.Helper()

	id := chain.ID()
	hashes := make([][]byte, 0, to-from+1)
	var payloads uint64
	for height := from; height <= to; height++ {
		block, ok := chain.BlockAt(height)
		if !ok {
			t.Fatalf("no block at height %d", height)
		}
		hashes = append(hashes, block.Hash)
		payloads += uint64(block.Header.PayloadCount)
	}

	return SidechainAnchor{
		PublisherID:  id.PublisherID,
		GameID:       id.GameID,
		FromHeight:   from,
		ToHeight:     to,
		TipHash:      append([]byte{}, hashes[len(hashes)-1]...),
		HeaderRoot:   merkleRoot(sidechainHeaderDomain, hashes),
		PayloadCount: payloads,
	}
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

// registerPublisher allocates a publisher and one game through the real
// registration path, and returns the identity plus the allocated sidechain id.
//
// Tests use the allocating path rather than a shortcut so that the ids they
// exercise are the ids consensus would actually hand out.
func registerPublisher(t *testing.T, ledger *AnchorLedger) (*PeerIdentity, SidechainID) {
	t.Helper()

	identity, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("NewPeerIdentity: %v", err)
	}

	registry := ledger.Registry()
	publisherID, err := registry.RegisterPublisher(identity.PublicPEM, "test publisher",
		MinPublisherBondUnits, 0)
	if err != nil {
		t.Fatalf("RegisterPublisher: %v", err)
	}
	gameID, err := registry.RegisterGame(publisherID, "test game", 0)
	if err != nil {
		t.Fatalf("RegisterGame: %v", err)
	}

	id, err := NewSidechainID(publisherID, gameID)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}
	return identity, id
}

func TestAnchorCommitsThenAdvancesTheWatermark(t *testing.T) {
	ledger := NewAnchorLedger(NewPublisherRegistry())
	identity, id := registerPublisher(t, ledger)
	chain := buildChain(t, id, 3)

	anchor, err := AnchorSidechain(chain, identity, ledger, 1)
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
	repeat, err := AnchorSidechain(chain, identity, ledger, 2)
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

	next, err := AnchorSidechain(chain, identity, ledger, 3)
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
	ledger := NewAnchorLedger(NewPublisherRegistry())
	identity, id := registerPublisher(t, ledger)
	chain := buildChain(t, id, 4)

	anchor, err := AnchorSidechain(chain, identity, ledger, 1)
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
	ledger := NewAnchorLedger(NewPublisherRegistry())
	identity, id := registerPublisher(t, ledger)
	chain := buildChain(t, id, 5)

	// The first anchor must start at height 0; starting later would leave the
	// beginning of the history permanently uncommitted.
	gapFirst := anchorOver(t, chain, 2, 4)
	signature, err := SignAnchor(identity, gapFirst)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}
	if err := ledger.Record(gapFirst, signature, 1); !errors.Is(err, ErrAnchorNotContiguous) {
		t.Fatalf("expected ErrAnchorNotContiguous for a late first anchor, got %v", err)
	}

	good := anchorOver(t, chain, 0, 2)
	signature, err = SignAnchor(identity, good)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}
	if err := ledger.Record(good, signature, 1); err != nil {
		t.Fatalf("Record: %v", err)
	}

	// Re-anchoring a range already committed is the rewrite path: it would let a
	// publisher replace 1..2 with different contents under a fresh commitment.
	overlap := anchorOver(t, chain, 1, 4)
	signature, err = SignAnchor(identity, overlap)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}
	if err := ledger.Record(overlap, signature, 2); !errors.Is(err, ErrAnchorNotContiguous) {
		t.Fatalf("expected ErrAnchorNotContiguous for an overlapping anchor, got %v", err)
	}

	// A gap would leave heights 3 uncommitted forever.
	skip := anchorOver(t, chain, 4, 4)
	signature, err = SignAnchor(identity, skip)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}
	if err := ledger.Record(skip, signature, 2); !errors.Is(err, ErrAnchorNotContiguous) {
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
	ledger := NewAnchorLedger(NewPublisherRegistry())

	// The id the first registration will allocate, built before anyone has
	// registered so the unregistered case can be exercised against it.
	id, err := NewSidechainID(1, 1)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}
	chain := buildChain(t, id, 2)

	anchor := anchorOver(t, chain, 0, 1)

	stranger, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("NewPeerIdentity: %v", err)
	}
	signature, err := SignAnchor(stranger, anchor)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}

	// Nothing is registered yet: an unknown publisher cannot anchor at all.
	if err := ledger.Record(anchor, signature, 1); !errors.Is(err, ErrAnchorUnauthorised) {
		t.Fatalf("expected ErrAnchorUnauthorised for an unregistered publisher, got %v", err)
	}

	owner, allocated := registerPublisher(t, ledger)
	if !allocated.Equal(id) {
		t.Fatalf("the first registration allocated %s, expected %s", allocated, id)
	}

	// Now registered -- but the signature belongs to somebody else. This is the
	// takeover case: another party committing a history under this publisher's id.
	if err := ledger.Record(anchor, signature, 1); !errors.Is(err, ErrAnchorUnauthorised) {
		t.Fatalf("expected ErrAnchorUnauthorised for a foreign signature, got %v", err)
	}

	if err := ledger.Record(anchor, nil, 1); !errors.Is(err, ErrAnchorUnauthorised) {
		t.Fatalf("expected ErrAnchorUnauthorised for an unsigned anchor, got %v", err)
	}

	signature, err = SignAnchor(owner, anchor)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}
	if err := ledger.Record(anchor, signature, 1); err != nil {
		t.Fatalf("the owner's own anchor was rejected: %v", err)
	}
}

func TestAnchorSignatureIsBoundToItsRange(t *testing.T) {
	ledger := NewAnchorLedger(NewPublisherRegistry())
	identity, id := registerPublisher(t, ledger)
	chain := buildChain(t, id, 3)

	anchor := anchorOver(t, chain, 0, 2)
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
		"header root":   func(a *SidechainAnchor) { a.HeaderRoot[0] ^= 0xFF },
		"payload count": func(a *SidechainAnchor) { a.PayloadCount = 99 },
	}

	for name, mutate := range mutations {
		t.Run(name, func(t *testing.T) {
			altered := anchor
			altered.TipHash = append([]byte{}, anchor.TipHash...)
			altered.HeaderRoot = append([]byte{}, anchor.HeaderRoot...)
			mutate(&altered)

			if err := ledger.Verify(altered, signature); err == nil {
				t.Fatalf("changing the %s left the signature valid", name)
			}
		})
	}
}

func TestPublisherRegistryAllocatesAndRefusesKeyReuse(t *testing.T) {
	registry := NewPublisherRegistry()

	owner, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("NewPeerIdentity: %v", err)
	}

	first, err := registry.RegisterPublisher(owner.PublicPEM, "owner", MinPublisherBondUnits, 10)
	if err != nil {
		t.Fatalf("RegisterPublisher: %v", err)
	}
	// Ids are allocated, not chosen, so the first is 1 -- and zero is never
	// handed out, because zero is what an omitted protobuf field decodes to.
	if first != 1 {
		t.Fatalf("first publisher got id %d, want 1", first)
	}

	// One key must not hold two identities: compromising it would compromise
	// both, and "which publisher signed this" would stop having one answer.
	if _, err := registry.RegisterPublisher(owner.PublicPEM, "again", MinPublisherBondUnits, 11); !errors.Is(err, ErrKeyAlreadyRegistered) {
		t.Fatalf("expected ErrKeyAlreadyRegistered, got %v", err)
	}

	other, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("NewPeerIdentity: %v", err)
	}
	second, err := registry.RegisterPublisher(other.PublicPEM, "other", MinPublisherBondUnits, 12)
	if err != nil {
		t.Fatalf("RegisterPublisher: %v", err)
	}
	if second != 2 {
		t.Fatalf("second publisher got id %d, want 2", second)
	}

	// A bond below the minimum is refused: the availability rules have nothing
	// behind them otherwise.
	third, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("NewPeerIdentity: %v", err)
	}
	if _, err := registry.RegisterPublisher(third.PublicPEM, "cheap", MinPublisherBondUnits-1, 13); !errors.Is(err, ErrRegistrationInvalid) {
		t.Fatalf("expected ErrRegistrationInvalid for an insufficient bond, got %v", err)
	}
}

func TestAnchoringRequiresARegisteredGame(t *testing.T) {
	ledger := NewAnchorLedger(NewPublisherRegistry())
	identity, id := registerPublisher(t, ledger)

	// A registered publisher, but a game id they never registered. Without the
	// game check a publisher could anchor any game id at all -- including one
	// another publisher's SDK is already writing.
	unregistered, err := NewSidechainID(id.PublisherID, id.GameID+1)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}
	chain := buildChain(t, unregistered, 2)
	anchor := anchorOver(t, chain, 0, 1)

	signature, err := SignAnchor(identity, anchor)
	if err != nil {
		t.Fatalf("SignAnchor: %v", err)
	}
	if err := ledger.Record(anchor, signature, 1); !errors.Is(err, ErrGameNotRegistered) {
		t.Fatalf("expected ErrGameNotRegistered, got %v", err)
	}

	// Registering it makes the same anchor acceptable.
	gameID, err := ledger.Registry().RegisterGame(id.PublisherID, "second game", 1)
	if err != nil {
		t.Fatalf("RegisterGame: %v", err)
	}
	if gameID != unregistered.GameID {
		t.Fatalf("allocated game %d, expected %d", gameID, unregistered.GameID)
	}
	if err := ledger.Record(anchor, signature, 1); err != nil {
		t.Fatalf("anchoring a registered game failed: %v", err)
	}
}

func TestSidechainGoesDelinquentAfterALongGap(t *testing.T) {
	ledger := NewAnchorLedger(NewPublisherRegistry())
	identity, id := registerPublisher(t, ledger)
	chain := buildChain(t, id, 2)

	if _, err := AnchorSidechain(chain, identity, ledger, 100); err != nil {
		t.Fatalf("AnchorSidechain: %v", err)
	}
	if ledger.IsDelinquent(id) {
		t.Fatal("a sidechain was delinquent on its first anchor")
	}

	block, err := NewSidechainBlock(id, chain.Tip(), [][]byte{[]byte("late")}, 1)
	if err != nil {
		t.Fatalf("NewSidechainBlock: %v", err)
	}
	if err := chain.Append(block); err != nil {
		t.Fatalf("Append: %v", err)
	}

	// The anchor still lands -- refusing it would leave a publisher who had an
	// outage permanently unable to commit anything ever again -- but the lapse
	// is on the record.
	late := 100 + MaxAnchorGap + 1
	if _, err := AnchorSidechain(chain, identity, ledger, late); err != nil {
		t.Fatalf("a late anchor was refused: %v", err)
	}
	if !ledger.IsDelinquent(id) {
		t.Fatalf("a gap of %d main-chain blocks did not mark the sidechain delinquent",
			MaxAnchorGap+1)
	}
}

func TestAnchorSpanIsBounded(t *testing.T) {
	id, err := NewSidechainID(1, 1)
	if err != nil {
		t.Fatalf("NewSidechainID: %v", err)
	}

	// An unbounded span would let one anchor commit history that had been
	// rewritable the whole time, which is a commitment in name only.
	oversized := SidechainAnchor{
		PublisherID: id.PublisherID, GameID: id.GameID,
		FromHeight: 0, ToHeight: MaxAnchorSpan,
		TipHash:    make([]byte, 32),
		HeaderRoot: make([]byte, 32),
	}
	if err := oversized.Validate(); !errors.Is(err, ErrAnchorSpanTooLarge) {
		t.Fatalf("expected ErrAnchorSpanTooLarge, got %v", err)
	}

	atLimit := oversized
	atLimit.ToHeight = MaxAnchorSpan - 1
	if err := atLimit.Validate(); err != nil {
		t.Fatalf("an anchor exactly at the limit was refused: %v", err)
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

	anchor := anchorOver(t, chain, 0, 1)

	if err := VerifyAgainstChain(anchor, other); !errors.Is(err, ErrSidechainMismatch) {
		t.Fatalf("expected ErrSidechainMismatch, got %v", err)
	}
}
