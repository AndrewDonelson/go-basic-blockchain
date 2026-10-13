// File sidechain/helpers_test.go - Shared test fixtures.
package sidechain

import (
	"testing"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/merkle"
)

// buildChain returns a sidechain carrying `count` blocks of trivial payloads.
func buildChain(t *testing.T, id ID, count int) *Chain {
	t.Helper()

	chain, err := NewChain(id)
	if err != nil {
		t.Fatalf("NewChain: %v", err)
	}

	for i := 0; i < count; i++ {
		block, err := NewBlock(id, chain.Tip(),
			[][]byte{[]byte{byte(i)}}, int64(1_700_000_000+i))
		if err != nil {
			t.Fatalf("NewBlock at %d: %v", i, err)
		}
		if err := chain.Append(block); err != nil {
			t.Fatalf("Append at %d: %v", i, err)
		}
	}
	return chain
}

// anchorOver builds a well-formed anchor for an explicit range, so a test can
// choose a range BuildAnchor would never produce.
func anchorOver(t *testing.T, chain *Chain, from, to uint64) Anchor {
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

	return Anchor{
		PublisherID:  id.PublisherID,
		GameID:       id.GameID,
		FromHeight:   from,
		ToHeight:     to,
		TipHash:      append([]byte{}, hashes[len(hashes)-1]...),
		HeaderRoot:   merkle.Root(HeaderDomain, hashes),
		PayloadCount: payloads,
	}
}

func mustBlockHash(t *testing.T, chain *Chain, height uint64) []byte {
	t.Helper()
	block, ok := chain.BlockAt(height)
	if !ok {
		t.Fatalf("no block at height %d", height)
	}
	return append([]byte{}, block.Hash...)
}
