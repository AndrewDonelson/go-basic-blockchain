// Package sdk is a software development kit for building blockchain applications.
// File sdk/sidechain_filestore_test.go - Tests for sidechain persistence.
package sdk

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

// chainBlocks returns a sidechain's blocks in height order.
func chainBlocks(t *testing.T, chain *Sidechain) []*SidechainBlock {
	t.Helper()
	blocks := make([]*SidechainBlock, 0, chain.Len())
	for height := 0; height < chain.Len(); height++ {
		block, ok := chain.BlockAt(uint64(height))
		if !ok {
			t.Fatalf("no block at height %d", height)
		}
		blocks = append(blocks, block)
	}
	return blocks
}

// jsonRoundTrip encodes and decodes a block, as a store would.
func jsonRoundTrip(t *testing.T, block *SidechainBlock) *SidechainBlock {
	t.Helper()
	encoded, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded SidechainBlock
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	return &decoded
}

func TestSidechainBlockJSONRoundTrip(t *testing.T) {
	id := SidechainID{PublisherID: 1, GameID: 2}
	chain := buildRichChain(t, id, 3, 3)

	for _, block := range chainBlocks(t, chain) {
		decoded := jsonRoundTrip(t, block)
		if err := decoded.Validate(); err != nil {
			t.Fatalf("height %d no longer validates after a round trip: %v",
				block.Header.Height, err)
		}
		if !bytes.Equal(decoded.Hash, block.Hash) {
			t.Fatalf("height %d changed hash in a round trip", block.Header.Height)
		}
		for i := range block.Payloads {
			if !bytes.Equal(decoded.Payloads[i], block.Payloads[i]) {
				t.Fatalf("height %d payload %d changed in a round trip", block.Header.Height, i)
			}
		}
	}

	// The names are a contract with whatever else reads these files.
	encoded, _ := json.Marshal(chain.Tip())
	for _, key := range []string{`"header"`, `"hash"`, `"payloads"`, `"previous_hash"`,
		`"payload_root"`, `"timestamp_unix_nano":"`, `"payload_count"`} {
		if !bytes.Contains(encoded, []byte(key)) {
			t.Fatalf("the wire form lost %s: %s", key, encoded)
		}
	}
}

func TestRestoreSidechainRebuildsPersistedHistory(t *testing.T) {
	id := SidechainID{PublisherID: 1, GameID: 2}
	original := buildRichChain(t, id, 5, 2)

	var persisted []*SidechainBlock
	for _, block := range chainBlocks(t, original) {
		persisted = append(persisted, jsonRoundTrip(t, block))
	}

	anchored := uint64(2)
	restored, err := RestoreSidechain(id, persisted, &anchored)
	if err != nil {
		t.Fatalf("RestoreSidechain: %v", err)
	}
	if !bytes.Equal(restored.Tip().Hash, original.Tip().Hash) {
		t.Fatal("the restored tip is not the original tip")
	}
	from, to, ok := restored.PendingAnchor()
	if !ok || from != 3 || to != 4 {
		t.Fatalf("pending range is %d..%d (%v), want 3..4", from, to, ok)
	}

	// The restored chain is the anchored one, so it can still prove records.
	anchor := anchorOver(t, original, 0, 2)
	if _, err := ProveRecord(restored, anchor, 1, 0); err != nil {
		t.Fatalf("the restored chain cannot prove a record: %v", err)
	}
}

func TestRestoreSidechainRefusesAnythingThatDoesNotCheckOut(t *testing.T) {
	id := SidechainID{PublisherID: 1, GameID: 2}

	fresh := func() []*SidechainBlock {
		var out []*SidechainBlock
		for _, block := range chainBlocks(t, buildChain(t, id, 4)) {
			out = append(out, jsonRoundTrip(t, block))
		}
		return out
	}
	foreign := chainBlocks(t, buildChain(t, SidechainID{PublisherID: 7, GameID: 2}, 1))

	beyondTip := uint64(9)

	cases := []struct {
		name     string
		id       SidechainID
		blocks   func() []*SidechainBlock
		anchored *uint64
		want     error
	}{
		{"zero id", SidechainID{}, fresh, nil, ErrInvalidSidechainID},
		{"another publisher's genesis", id, func() []*SidechainBlock { return foreign }, nil,
			ErrSidechainMismatch},
		{"missing genesis", id, func() []*SidechainBlock { return fresh()[1:] }, nil,
			ErrSidechainNotContiguous},
		{"gap", id, func() []*SidechainBlock {
			b := fresh()
			return append(b[:2], b[3:]...)
		}, nil, ErrSidechainNotContiguous},
		{"reordered", id, func() []*SidechainBlock {
			b := fresh()
			b[1], b[2] = b[2], b[1]
			return b
		}, nil, ErrSidechainNotContiguous},
		{"payload edited", id, func() []*SidechainBlock {
			b := fresh()
			b[2].Payloads[0] = []byte("rewritten")
			return b
		}, nil, ErrInvalidSidechainBlock},
		{"header edited", id, func() []*SidechainBlock {
			b := fresh()
			b[2].Header.TimestampUnixNano++
			return b
		}, nil, ErrInvalidSidechainBlock},
		{"broken hash link", id, func() []*SidechainBlock {
			b := fresh()
			// A self-consistent block whose predecessor is not the one stored.
			relinked, err := NewSidechainBlock(id, b[1], [][]byte{[]byte("x")}, 1)
			if err != nil {
				t.Fatalf("NewSidechainBlock: %v", err)
			}
			relinked.Header.PreviousHash = bytes.Repeat([]byte{1}, 32)
			relinked.Hash = relinked.ComputeHash()
			b[2] = relinked
			return b[:3]
		}, nil, ErrSidechainNotContiguous},
		{"watermark beyond the tip", id, fresh, &beyondTip, ErrAnchorNotContiguous},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := RestoreSidechain(tc.id, tc.blocks(), tc.anchored); !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}
}

func TestSidechainMarkAnchoredOnlyMovesForward(t *testing.T) {
	chain := buildChain(t, SidechainID{PublisherID: 1, GameID: 1}, 4)

	if err := chain.MarkAnchored(2); err != nil {
		t.Fatalf("MarkAnchored(2): %v", err)
	}
	if err := chain.MarkAnchored(2); err != nil {
		t.Fatalf("repeating the watermark should be a no-op, got %v", err)
	}
	if err := chain.MarkAnchored(1); !errors.Is(err, ErrAnchorNotContiguous) {
		t.Fatalf("moving the watermark back should be refused, got %v", err)
	}
	if err := chain.MarkAnchored(4); !errors.Is(err, ErrAnchorNotContiguous) {
		t.Fatalf("marking past the tip should be refused, got %v", err)
	}
	if height, ok := chain.AnchoredHeight(); !ok || height != 2 {
		t.Fatalf("watermark is %d (%v), want 2", height, ok)
	}
}

func TestFileSidechainStoreSurvivesReopening(t *testing.T) {
	dir := t.TempDir()
	id := SidechainID{PublisherID: 4, GameID: 1}
	other := SidechainID{PublisherID: 4, GameID: 2}

	store, err := OpenFileSidechainStore(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}

	source := buildRichChain(t, id, 4, 2)
	for _, block := range chainBlocks(t, source) {
		if err := store.AppendBlock(block); err != nil {
			t.Fatalf("AppendBlock: %v", err)
		}
	}
	if err := store.MarkAnchored(id, 1); err != nil {
		t.Fatalf("MarkAnchored: %v", err)
	}
	if err := store.AppendBlock(chainBlocks(t, buildChain(t, other, 1))[0]); err != nil {
		t.Fatalf("AppendBlock other: %v", err)
	}

	// A fresh store over the same directory: what a restarted process sees.
	reopened, err := OpenFileSidechainStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	ids, err := reopened.IDs()
	if err != nil {
		t.Fatalf("IDs: %v", err)
	}
	if len(ids) != 2 || !ids[0].Equal(id) || !ids[1].Equal(other) {
		t.Fatalf("IDs = %v, want [%s %s]", ids, id, other)
	}

	loaded, err := reopened.Load(id)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if !bytes.Equal(loaded.Tip().Hash, source.Tip().Hash) {
		t.Fatal("the loaded tip is not the stored tip")
	}
	if height, ok := loaded.AnchoredHeight(); !ok || height != 1 {
		t.Fatalf("watermark is %d (%v), want 1", height, ok)
	}

	// And the reopened store carries on from the tip it read back.
	next, err := NewSidechainBlock(id, loaded.Tip(), [][]byte{[]byte("after restart")}, 9)
	if err != nil {
		t.Fatalf("NewSidechainBlock: %v", err)
	}
	if err := reopened.AppendBlock(next); err != nil {
		t.Fatalf("AppendBlock after reopen: %v", err)
	}

	empty, err := reopened.Load(SidechainID{PublisherID: 99, GameID: 1})
	if err != nil || empty.Len() != 0 {
		t.Fatalf("an unknown sidechain should load empty, got %v, %v", empty, err)
	}
}

func TestFileSidechainStoreRefusesBlocksThatDoNotFollow(t *testing.T) {
	store, err := OpenFileSidechainStore(t.TempDir())
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	id := SidechainID{PublisherID: 4, GameID: 1}
	blocks := chainBlocks(t, buildChain(t, id, 3))

	if err := store.AppendBlock(blocks[1]); !errors.Is(err, ErrSidechainNotContiguous) {
		t.Fatalf("a first block above height 0 should be refused, got %v", err)
	}
	if err := store.AppendBlock(blocks[0]); err != nil {
		t.Fatalf("AppendBlock(0): %v", err)
	}
	if err := store.AppendBlock(blocks[0]); !errors.Is(err, ErrSidechainNotContiguous) {
		t.Fatalf("re-appending the tip should be refused, got %v", err)
	}
	if err := store.AppendBlock(blocks[2]); !errors.Is(err, ErrSidechainNotContiguous) {
		t.Fatalf("a gap should be refused, got %v", err)
	}

	// Same height, different history.
	fork, err := NewSidechainBlock(id, nil, [][]byte{[]byte("fork")}, 1)
	if err != nil {
		t.Fatalf("NewSidechainBlock: %v", err)
	}
	forkChild, err := NewSidechainBlock(id, fork, nil, 2)
	if err != nil {
		t.Fatalf("NewSidechainBlock: %v", err)
	}
	if err := store.AppendBlock(forkChild); !errors.Is(err, ErrSidechainNotContiguous) {
		t.Fatalf("a block from another history should be refused, got %v", err)
	}

	tampered := jsonRoundTrip(t, blocks[1])
	tampered.Payloads[0] = []byte("rewritten")
	if err := store.AppendBlock(tampered); !errors.Is(err, ErrInvalidSidechainBlock) {
		t.Fatalf("an invalid block should be refused, got %v", err)
	}

	if err := store.MarkAnchored(id, 1); !errors.Is(err, ErrAnchorNotContiguous) {
		t.Fatalf("a watermark past the stored tip should be refused, got %v", err)
	}
}

func TestFileSidechainStoreCutsOffATornWrite(t *testing.T) {
	dir := t.TempDir()
	id := SidechainID{PublisherID: 4, GameID: 1}
	blocks := chainBlocks(t, buildChain(t, id, 3))

	store, err := OpenFileSidechainStore(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, block := range blocks[:2] {
		if err := store.AppendBlock(block); err != nil {
			t.Fatalf("AppendBlock: %v", err)
		}
	}

	// Simulate a crash part-way through writing the third block.
	path := filepath.Join(dir, id.Hex()+sidechainFileExtension)
	whole, _ := json.Marshal(sidechainLogRecord{Block: blocks[2]})
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	if _, err := file.Write(whole[:len(whole)/2]); err != nil {
		t.Fatalf("write torn record: %v", err)
	}
	_ = file.Close()

	reopened, err := OpenFileSidechainStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	loaded, err := reopened.Load(id)
	if err != nil {
		t.Fatalf("a torn final record should be cut off, not fatal: %v", err)
	}
	if loaded.Len() != 2 {
		t.Fatalf("loaded %d blocks, want the 2 that were completely written", loaded.Len())
	}

	// The torn bytes are gone, so the next append lands on a clean line.
	if err := reopened.AppendBlock(blocks[2]); err != nil {
		t.Fatalf("AppendBlock after repair: %v", err)
	}
	again, err := OpenFileSidechainStore(dir)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	if loaded, err := again.Load(id); err != nil || loaded.Len() != 3 {
		t.Fatalf("after repair and append: %v blocks, err %v", loaded, err)
	}
}

func TestFileSidechainStoreRefusesCorruption(t *testing.T) {
	dir := t.TempDir()
	id := SidechainID{PublisherID: 4, GameID: 1}
	blocks := chainBlocks(t, buildChain(t, id, 3))

	store, err := OpenFileSidechainStore(dir)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	for _, block := range blocks {
		if err := store.AppendBlock(block); err != nil {
			t.Fatalf("AppendBlock: %v", err)
		}
	}

	path := filepath.Join(dir, id.Hex()+sidechainFileExtension)
	original, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	lines := bytes.SplitAfter(original, []byte("\n"))

	cases := []struct {
		name   string
		mutate func() []byte
	}{
		{"unparseable middle line", func() []byte {
			return bytes.Join([][]byte{lines[0], []byte("{not json\n"), lines[2]}, nil)
		}},
		{"edited payload", func() []byte {
			var record sidechainLogRecord
			if err := json.Unmarshal(lines[1], &record); err != nil {
				t.Fatalf("decode: %v", err)
			}
			record.Block.Payloads[0] = []byte("rewritten")
			edited, _ := json.Marshal(record)
			return bytes.Join([][]byte{lines[0], append(edited, '\n'), lines[2]}, nil)
		}},
		{"dropped block", func() []byte {
			return bytes.Join([][]byte{lines[0], lines[2]}, nil)
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, tc.mutate(), 0o600); err != nil {
				t.Fatalf("write: %v", err)
			}
			reopened, err := OpenFileSidechainStore(dir)
			if err != nil {
				t.Fatalf("reopen: %v", err)
			}
			if _, err := reopened.Load(id); !errors.Is(err, ErrSidechainStoreCorrupt) {
				t.Fatalf("expected ErrSidechainStoreCorrupt, got %v", err)
			}
			// And it will not append on top of a history it cannot vouch for.
			if err := reopened.AppendBlock(blocks[0]); !errors.Is(err, ErrSidechainStoreCorrupt) {
				t.Fatalf("expected appends to be refused, got %v", err)
			}
		})
	}
}
