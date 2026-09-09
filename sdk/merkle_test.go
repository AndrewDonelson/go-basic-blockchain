// Package sdk is a software development kit for building blockchain applications.
// File sdk/merkle_test.go - Tests for the Merkle tree and its proofs.
package sdk

import (
	"bytes"
	"fmt"
	"testing"
)

const testDomain = "gbb/test"

func testLeaves(n int) [][]byte {
	leaves := make([][]byte, n)
	for i := range leaves {
		leaves[i] = []byte(fmt.Sprintf("leaf-%d", i))
	}
	return leaves
}

func TestMerkleProofsVerifyForEveryLeaf(t *testing.T) {
	// Odd sizes are the interesting ones: they are where a node gets paired with
	// itself and where a naive implementation goes wrong.
	for _, count := range []int{1, 2, 3, 4, 5, 7, 8, 9, 16, 17, 100} {
		t.Run(fmt.Sprintf("%d leaves", count), func(t *testing.T) {
			leaves := testLeaves(count)
			root := merkleRoot(testDomain, leaves)

			for i := range leaves {
				path, err := merkleProof(testDomain, leaves, i)
				if err != nil {
					t.Fatalf("merkleProof(%d): %v", i, err)
				}
				if !merkleVerify(testDomain, root, leaves[i], i, count, path) {
					t.Fatalf("the proof for leaf %d of %d did not verify", i, count)
				}
			}
		})
	}
}

func TestMerkleProofFailsForTheWrongLeaf(t *testing.T) {
	leaves := testLeaves(8)
	root := merkleRoot(testDomain, leaves)

	path, err := merkleProof(testDomain, leaves, 3)
	if err != nil {
		t.Fatalf("merkleProof: %v", err)
	}

	if merkleVerify(testDomain, root, []byte("not a leaf"), 3, 8, path) {
		t.Fatal("a proof verified for data that is not in the tree")
	}
	// The same path at a different index proves nothing: position is part of the
	// statement, not a hint.
	if merkleVerify(testDomain, root, leaves[3], 4, 8, path) {
		t.Fatal("a proof verified at the wrong index")
	}
	if merkleVerify(testDomain, root, leaves[3], 3, 8, path[:len(path)-1]) {
		t.Fatal("a truncated path verified")
	}
}

// TestMerkleRootBindsTheLeafCount is CVE-2012-2459 in miniature.
//
// A tree whose last leaf is duplicated has the same internal shape as one that
// genuinely contains the duplicate, so without the count in the root the two are
// indistinguishable -- which is how a Bitcoin block could be given a second,
// invalid encoding with an identical Merkle root.
func TestMerkleRootBindsTheLeafCount(t *testing.T) {
	leaves := testLeaves(3)

	duplicated := make([][]byte, 0, 4)
	duplicated = append(duplicated, leaves...)
	duplicated = append(duplicated, leaves[2])

	if bytes.Equal(merkleRoot(testDomain, leaves), merkleRoot(testDomain, duplicated)) {
		t.Fatal("three leaves and three-plus-a-duplicate produced the same root")
	}

	// A proof built over three leaves must not verify against the four-leaf root
	// while still claiming three. Note the narrowness of the claim: leaf 2 really
	// is at index 2 in both trees, so a proof presented with the correct count of
	// four verifies legitimately. What the count prevents is passing one tree off
	// as the other.
	path, err := merkleProof(testDomain, leaves, 2)
	if err != nil {
		t.Fatalf("merkleProof: %v", err)
	}
	if merkleVerify(testDomain, merkleRoot(testDomain, duplicated), leaves[2], 2, 3, path) {
		t.Fatal("a three-leaf proof verified against a four-leaf root")
	}

	// And the duplicate's own index does not exist in the smaller tree.
	if merkleVerify(testDomain, merkleRoot(testDomain, leaves), leaves[2], 3, 3, path) {
		t.Fatal("an index outside the tree verified")
	}
}

func TestMerkleDomainsDoNotCollide(t *testing.T) {
	leaves := testLeaves(5)

	// Two trees over identical data in different domains must not share a root,
	// or a proof about a sidechain's payloads could be replayed as a proof about
	// its headers.
	if bytes.Equal(merkleRoot("gbb/a", leaves), merkleRoot("gbb/b", leaves)) {
		t.Fatal("two domains produced the same root over the same leaves")
	}

	path, err := merkleProof("gbb/a", leaves, 1)
	if err != nil {
		t.Fatalf("merkleProof: %v", err)
	}
	if merkleVerify("gbb/b", merkleRoot("gbb/b", leaves), leaves[1], 1, 5, path) {
		t.Fatal("a proof verified across domains")
	}
}

func TestMerkleEmptyAndOutOfRange(t *testing.T) {
	// "Committed to nothing" needs its own value, distinct from any real tree.
	empty := merkleRoot(testDomain, nil)
	if bytes.Equal(empty, merkleRoot(testDomain, testLeaves(1))) {
		t.Fatal("the empty root collided with a one-leaf root")
	}
	if merkleVerify(testDomain, empty, []byte("x"), 0, 0, nil) {
		t.Fatal("something verified against the empty root")
	}

	if _, err := merkleProof(testDomain, testLeaves(4), 4); err == nil {
		t.Fatal("a proof was produced for an index outside the tree")
	}
	if _, err := merkleProof(testDomain, testLeaves(4), -1); err == nil {
		t.Fatal("a proof was produced for a negative index")
	}
}
