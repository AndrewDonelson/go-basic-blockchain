package sdk

import (
	"bytes"

	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/helios/algorithm"
)

// The delay must be sequential ACROSS blocks, not merely within one.
//
// Stage 2's input is the mining header, and that header contains PreviousHash --
// so block N's delay cannot begin until block N-1 is final. These tests pin that
// dependency, because it is a property of what goes into the header rather than
// of the VDF itself, and nothing in the VDF would notice if it disappeared.
//
// It is deliberately NOT arranged by feeding the parent's VDF output into the
// child's input. That would give the same guarantee, but stage 2 is verified in
// validateStandalone, which runs outside the chain lock precisely so an expensive
// proof check does not stall mining and every reader. Making the check depend on
// the parent would force it back inside the lock for no additional security.

// TestVDFInputDependsOnTheParentBlock is the load-bearing test.
//
// If the mining header ever stops committing to PreviousHash, the per-block delay
// survives but the chain-level ordering silently vanishes: a miner could
// precompute every block's delay in parallel, before its parent exists.
func TestVDFInputDependsOnTheParentBlock(t *testing.T) {
	base := NewBlock(nil, "parent-hash-aaaa")
	base.Index = *big.NewInt(1)
	base.Header.Timestamp = time.Unix(1700000000, 0)
	base.Header.Difficulty = 4
	base.Header.MerkleRoot = []byte("merkle")

	changedParent := NewBlock(nil, "parent-hash-bbbb")
	changedParent.Index = *big.NewInt(1)
	changedParent.Header.Timestamp = base.Header.Timestamp
	changedParent.Header.Difficulty = base.Header.Difficulty
	changedParent.Header.MerkleRoot = base.Header.MerkleRoot

	headerA := base.createBlockHeaderForMining()
	headerB := changedParent.createBlockHeaderForMining()

	if bytes.Equal(headerA, headerB) {
		t.Fatal("two blocks with different parents produce the same mining header; " +
			"the delay would no longer be ordered across blocks and every block's " +
			"VDF could be computed in parallel before its parent existed")
	}
	if !strings.Contains(string(headerA), "parent-hash-aaaa") {
		t.Fatalf("the mining header does not commit to the parent hash: %q", headerA)
	}
}

// TestVDFOutputChangesWithTheParent follows that through to the delay itself.
func TestVDFOutputChangesWithTheParent(t *testing.T) {
	// A real algorithm, built here rather than taken from a fixture: a skip would
	// leave this test proving nothing.
	helios := algorithm.NewHeliosAlgorithm(algorithm.TestHeliosConfig())

	build := func(previousHash string) []byte {
		b := NewBlock(nil, previousHash)
		b.Index = *big.NewInt(1)
		b.Header.Timestamp = time.Unix(1700000000, 0)
		b.Header.Difficulty = 4
		b.Header.MerkleRoot = []byte("merkle")

		proof, err := helios.Mine(b.createBlockHeaderForMining(), difficultyTarget(1))
		if err != nil {
			t.Fatalf("mine: %v", err)
		}
		return proof.Stage2Result
	}

	first := build("parent-one")
	second := build("parent-two")

	if bytes.Equal(first, second) {
		t.Fatal("the delay output is the same for two different parents; the " +
			"chain-level ordering is gone")
	}

	// And it is stable for the same parent, or nodes could not agree.
	if !bytes.Equal(first, build("parent-one")) {
		t.Fatal("the delay output is not reproducible for the same parent")
	}
}

// TestChainOfBlocksSerialisesTheDelay demonstrates the property end to end: each
// block's delay input is determined by its predecessor, so the delays form one
// chain rather than a set that can be computed at once.
func TestChainOfBlocksSerialisesTheDelay(t *testing.T) {
	helios := algorithm.NewHeliosAlgorithm(algorithm.TestHeliosConfig())

	previousHash := "genesis-hash"
	seen := map[string]bool{}

	for height := 1; height <= 3; height++ {
		b := NewBlock(nil, previousHash)
		b.Index = *big.NewInt(int64(height))
		b.Header.Timestamp = time.Unix(1700000000, 0).Add(time.Duration(height) * time.Minute)
		b.Header.Difficulty = 1
		b.Header.MerkleRoot = []byte("merkle")

		proof, err := helios.Mine(b.createBlockHeaderForMining(), difficultyTarget(1))
		if err != nil {
			t.Fatalf("mine block %d: %v", height, err)
		}

		key := string(proof.Stage2Result)
		if seen[key] {
			t.Fatalf("block %d reproduced an earlier block's delay output", height)
		}
		seen[key] = true

		if err := b.updateWithHeliosProof(proof); err != nil {
			t.Fatalf("record proof: %v", err)
		}
		previousHash = b.Hash
	}

	if len(seen) != 3 {
		t.Fatalf("expected 3 distinct delay outputs, got %d", len(seen))
	}
}
