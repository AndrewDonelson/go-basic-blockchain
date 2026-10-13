// File sidechain/proof_test.go - Tests for portable record proofs.
package sidechain

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

// buildRichChain returns a sidechain whose blocks carry several payloads each,
// so payload proofs have real paths (a one-payload block has an empty one).
func buildRichChain(t *testing.T, id ID, blocks, payloadsPerBlock int) *Chain {
	t.Helper()

	chain, err := NewChain(id)
	if err != nil {
		t.Fatalf("NewChain: %v", err)
	}
	for height := 0; height < blocks; height++ {
		payloads := make([][]byte, 0, payloadsPerBlock)
		for i := 0; i < payloadsPerBlock; i++ {
			payloads = append(payloads, []byte{byte(id.GameID), byte(height), byte(i), 0xAB})
		}
		block, err := NewBlock(id, chain.Tip(), payloads, int64(1_700_000_000+height))
		if err != nil {
			t.Fatalf("NewBlock at %d: %v", height, err)
		}
		if err := chain.Append(block); err != nil {
			t.Fatalf("Append at %d: %v", height, err)
		}
	}
	return chain
}

func TestPayloadProofRoundTrip(t *testing.T) {
	id := ID{PublisherID: 3, GameID: 4}
	chain := buildRichChain(t, id, 1, 5)
	block, _ := chain.BlockAt(0)

	for index := range block.Payloads {
		proof, err := block.ProvePayload(index)
		if err != nil {
			t.Fatalf("ProvePayload(%d): %v", index, err)
		}
		if err := VerifyPayloadProof(block.Payloads[index], proof, block.Header.PayloadRoot); err != nil {
			t.Fatalf("payload %d did not verify: %v", index, err)
		}
	}

	if _, err := block.ProvePayload(5); err == nil {
		t.Fatal("a proof was produced for a payload the block does not carry")
	}
}

func TestPayloadProofRejectsTampering(t *testing.T) {
	id := ID{PublisherID: 3, GameID: 4}
	chain := buildRichChain(t, id, 1, 5)
	block, _ := chain.BlockAt(0)

	proof, err := block.ProvePayload(2)
	if err != nil {
		t.Fatalf("ProvePayload: %v", err)
	}
	payload := block.Payloads[2]
	root := block.Header.PayloadRoot

	cases := []struct {
		name    string
		payload []byte
		proof   func() PayloadProof
		root    []byte
	}{
		{"changed payload", []byte("not the payload"), func() PayloadProof { return proof }, root},
		{"wrong index", payload, func() PayloadProof { p := proof; p.Index = 3; return p }, root},
		{"wrong count", payload, func() PayloadProof { p := proof; p.Count = 6; return p }, root},
		{"swapped sibling", payload, func() PayloadProof {
			p := proof
			p.Path = append([][]byte{}, proof.Path...)
			p.Path[0], p.Path[1] = p.Path[1], p.Path[0]
			return p
		}, root},
		{"truncated path", payload, func() PayloadProof {
			p := proof
			p.Path = proof.Path[:len(proof.Path)-1]
			return p
		}, root},
		{"another block's root", payload, func() PayloadProof { return proof },
			mustBlockPayloadRoot(t, buildRichChain(t, ID{PublisherID: 3, GameID: 5}, 1, 5))},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyPayloadProof(tc.payload, tc.proof(), tc.root)
			if !errors.Is(err, ErrInvalidSidechainProof) {
				t.Fatalf("expected ErrInvalidSidechainProof, got %v", err)
			}
		})
	}
}

func mustBlockPayloadRoot(t *testing.T, chain *Chain) []byte {
	t.Helper()
	block, ok := chain.BlockAt(0)
	if !ok {
		t.Fatal("no block at height 0")
	}
	return block.Header.PayloadRoot
}

func TestBlockProofRequiresTheAnchoredHistory(t *testing.T) {
	id := ID{PublisherID: 3, GameID: 4}
	chain := buildRichChain(t, id, 6, 2)
	anchor := anchorOver(t, chain, 0, 5)

	for height := uint64(0); height <= 5; height++ {
		proof, err := ProveBlock(chain, anchor, height)
		if err != nil {
			t.Fatalf("ProveBlock(%d): %v", height, err)
		}
		if err := VerifyBlockProof(mustBlockHash(t, chain, height), proof, anchor); err != nil {
			t.Fatalf("block %d did not verify: %v", height, err)
		}
	}

	// A chain whose blocks are not the anchored ones cannot produce a proof at
	// all, rather than producing one that fails for whoever receives it.
	other := buildRichChain(t, id, 6, 3)
	if _, err := ProveBlock(other, anchor, 2); !errors.Is(err, ErrInvalidSidechainProof) {
		t.Fatalf("expected a different history to be refused, got %v", err)
	}

	if _, err := ProveBlock(chain, anchor, 6); !errors.Is(err, ErrInvalidSidechainProof) {
		t.Fatalf("expected a height outside the anchor to be refused, got %v", err)
	}

	// A hash that is in the anchor, presented at a height where it is not.
	proof, err := ProveBlock(chain, anchor, 2)
	if err != nil {
		t.Fatalf("ProveBlock: %v", err)
	}
	if err := VerifyBlockProof(mustBlockHash(t, chain, 3), proof, anchor); err == nil {
		t.Fatal("block 3's hash verified against block 2's proof")
	}
}

// anchoredRecordFixture is a chain, an anchor over part of it, and a proof for
// one payload inside that anchor.
type anchoredRecordFixture struct {
	chain   *Chain
	anchor  Anchor
	proof   *RecordProof
	payload []byte
}

func newAnchoredRecordFixture(t *testing.T) *anchoredRecordFixture {
	t.Helper()

	id := ID{PublisherID: 9, GameID: 2}
	chain := buildRichChain(t, id, 8, 4)
	// Anchor a range that does not start at zero, so the block's index inside
	// the anchor differs from its height.
	anchor := anchorOver(t, chain, 3, 7)

	proof, err := ProveRecord(chain, anchor, 5, 2)
	if err != nil {
		t.Fatalf("ProveRecord: %v", err)
	}
	block, _ := chain.BlockAt(5)

	return &anchoredRecordFixture{
		chain:   chain,
		anchor:  anchor,
		proof:   proof,
		payload: append([]byte{}, block.Payloads[2]...),
	}
}

func TestRecordProofVerifiesWithOnlyThePayloadAndTheAnchor(t *testing.T) {
	f := newAnchoredRecordFixture(t)

	// A third party receives the proof as JSON, and holds nothing else but the
	// payload and the anchor it read from the main chain.
	encoded, err := json.Marshal(f.proof)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var received RecordProof
	if err := json.Unmarshal(encoded, &received); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if err := VerifyRecord(f.payload, &received, f.anchor); err != nil {
		t.Fatalf("a genuine record did not verify: %v", err)
	}
}

func TestRecordProofRejectsTampering(t *testing.T) {
	f := newAnchoredRecordFixture(t)

	// An anchor for the same sidechain that does not cover height 5.
	laterAnchor := anchorOver(t, f.chain, 6, 7)

	// The same height in another publisher's sidechain.
	foreignID := ID{PublisherID: 10, GameID: 2}
	foreign := buildRichChain(t, foreignID, 8, 4)
	foreignBlock, _ := foreign.BlockAt(5)

	clone := func() *RecordProof {
		p := *f.proof
		p.Payload.Path = append([][]byte{}, f.proof.Payload.Path...)
		p.Block.Path = append([][]byte{}, f.proof.Block.Path...)
		return &p
	}

	cases := []struct {
		name    string
		payload []byte
		proof   func() *RecordProof
		anchor  Anchor
		want    error
	}{
		{
			name:    "changed payload",
			payload: []byte("forged achievement"),
			proof:   clone,
			anchor:  f.anchor,
			want:    ErrInvalidSidechainProof,
		},
		{
			name:    "wrong payload index",
			payload: f.payload,
			proof:   func() *RecordProof { p := clone(); p.Payload.Index = 1; return p },
			anchor:  f.anchor,
			want:    ErrInvalidSidechainProof,
		},
		{
			name:    "swapped payload sibling",
			payload: f.payload,
			proof: func() *RecordProof {
				p := clone()
				p.Payload.Path[0], p.Payload.Path[1] = p.Payload.Path[1], p.Payload.Path[0]
				return p
			},
			anchor: f.anchor,
			want:   ErrInvalidSidechainProof,
		},
		{
			name:    "swapped block sibling",
			payload: f.payload,
			proof: func() *RecordProof {
				p := clone()
				p.Block.Path[0], p.Block.Path[1] = p.Block.Path[1], p.Block.Path[0]
				return p
			},
			anchor: f.anchor,
			want:   ErrInvalidSidechainProof,
		},
		{
			name:    "payload count disagrees with the header",
			payload: f.payload,
			proof:   func() *RecordProof { p := clone(); p.Payload.Count = 5; return p },
			anchor:  f.anchor,
			want:    ErrInvalidSidechainProof,
		},
		{
			name:    "header edited after proving",
			payload: f.payload,
			proof: func() *RecordProof {
				p := clone()
				p.Header.TimestampUnixNano++
				return p
			},
			anchor: f.anchor,
			want:   ErrInvalidSidechainProof,
		},
		{
			name:    "header from another sidechain",
			payload: foreignBlock.Payloads[2],
			proof: func() *RecordProof {
				p := clone()
				p.Header = foreignBlock.Header
				return p
			},
			anchor: f.anchor,
			want:   ErrSidechainMismatch,
		},
		{
			name:    "header relabelled as this sidechain",
			payload: foreignBlock.Payloads[2],
			proof: func() *RecordProof {
				p := clone()
				p.Header = foreignBlock.Header
				p.Header.PublisherID = f.anchor.PublisherID
				return p
			},
			anchor: f.anchor,
			want:   ErrInvalidSidechainProof,
		},
		{
			name:    "header for a different height",
			payload: f.payload,
			proof: func() *RecordProof {
				p := clone()
				block, _ := f.chain.BlockAt(6)
				p.Header = block.Header
				return p
			},
			anchor: f.anchor,
			want:   ErrInvalidSidechainProof,
		},
		{
			name:    "anchor that does not cover the height",
			payload: f.payload,
			proof:   clone,
			anchor:  laterAnchor,
			want:    ErrInvalidSidechainProof,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := VerifyRecord(tc.payload, tc.proof(), tc.anchor)
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
		})
	}

	if err := VerifyRecord(f.payload, nil, f.anchor); !errors.Is(err, ErrInvalidSidechainProof) {
		t.Fatalf("a nil proof should be refused, got %v", err)
	}
}

func TestRecordProofWireFormatIsStable(t *testing.T) {
	f := newAnchoredRecordFixture(t)

	encoded, err := json.Marshal(f.proof)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	// The field names are the contract with non-Go verifiers; renaming a Go
	// field must not move them.
	var shape map[string]json.RawMessage
	if err := json.Unmarshal(encoded, &shape); err != nil {
		t.Fatalf("decode shape: %v", err)
	}
	for _, key := range []string{"version", "header", "payload_proof", "block_proof"} {
		if _, ok := shape[key]; !ok {
			t.Fatalf("the wire form lost %q: %s", key, encoded)
		}
	}

	var header map[string]json.RawMessage
	if err := json.Unmarshal(shape["header"], &header); err != nil {
		t.Fatalf("decode header: %v", err)
	}
	// The timestamp is a string so a JavaScript reader cannot round it.
	if !bytes.HasPrefix(header["timestamp_unix_nano"], []byte(`"`)) {
		t.Fatalf("timestamp should be encoded as a string, got %s", header["timestamp_unix_nano"])
	}

	// An unknown version is refused rather than guessed at.
	shape["version"] = json.RawMessage("2")
	tampered, _ := json.Marshal(shape)
	var decoded RecordProof
	if err := json.Unmarshal(tampered, &decoded); !errors.Is(err, ErrInvalidSidechainProof) {
		t.Fatalf("expected an unknown version to be refused, got %v", err)
	}
}
