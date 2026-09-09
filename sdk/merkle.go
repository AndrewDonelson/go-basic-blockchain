// Package sdk is a software development kit for building blockchain applications.
// File sdk/merkle.go - Merkle trees with inclusion proofs.
package sdk

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

// ErrInvalidMerkleProof is returned when an inclusion proof does not check out.
var ErrInvalidMerkleProof = errors.New("invalid merkle proof")

// A Merkle tree here does three things that a naive one does not.
//
// LEAVES AND NODES ARE DOMAIN-SEPARATED. Hashing a leaf and hashing an internal
// node with the same function lets an attacker present an internal node as if it
// were a leaf, which is a second-preimage attack on the tree rather than on the
// hash.
//
// THE LEAF COUNT IS BOUND INTO THE ROOT. An odd node has to be paired with
// something, and pairing it with itself means a tree of n leaves and a tree of
// n+1 leaves whose last is duplicated produce the same root -- the flaw that
// CVE-2012-2459 turned into a Bitcoin denial-of-service. Folding the count into
// the root makes two trees of different sizes structurally unable to collide,
// which is cheaper and more complete than special-casing the duplication.
//
// PROOFS ARE PRODUCED, not just roots. A commitment nobody can prove membership
// against forces a verifier to hold the whole set, and an availability challenge
// that expensive is one nobody will ever issue.

// merkleLeafHash hashes one leaf under a domain.
func merkleLeafHash(domain string, leaf []byte) []byte {
	h := sha256.New()
	h.Write([]byte(domain))
	h.Write([]byte("/leaf"))
	h.Write(leaf)
	return h.Sum(nil)
}

// merkleNodeHash hashes a pair of children under a domain.
func merkleNodeHash(domain string, left, right []byte) []byte {
	h := sha256.New()
	h.Write([]byte(domain))
	h.Write([]byte("/node"))
	h.Write(left)
	h.Write(right)
	return h.Sum(nil)
}

// merkleFinalise binds the leaf count into the root.
func merkleFinalise(domain string, count int, apex []byte) []byte {
	var tmp [8]byte
	// count is a slice length, so it is never negative.
	binary.BigEndian.PutUint64(tmp[:], uint64(count)) //nolint:gosec // non-negative length

	h := sha256.New()
	h.Write([]byte(domain))
	h.Write([]byte("/root"))
	h.Write(tmp[:])
	h.Write(apex)
	return h.Sum(nil)
}

// merkleLevels builds every level of the tree, leaves first.
func merkleLevels(domain string, leaves [][]byte) [][][]byte {
	level := make([][]byte, 0, len(leaves))
	for _, leaf := range leaves {
		level = append(level, merkleLeafHash(domain, leaf))
	}

	levels := [][][]byte{level}
	for len(level) > 1 {
		next := make([][]byte, 0, (len(level)+1)/2)
		for i := 0; i < len(level); i += 2 {
			left := level[i]
			right := left
			if i+1 < len(level) {
				right = level[i+1]
			}
			next = append(next, merkleNodeHash(domain, left, right))
		}
		level = next
		levels = append(levels, level)
	}
	return levels
}

// merkleRoot returns the root over a set of leaves.
//
// An empty set has its own root rather than a zero value, so "committed to
// nothing" and "committed to a leaf that happens to hash to zero" stay distinct.
func merkleRoot(domain string, leaves [][]byte) []byte {
	if len(leaves) == 0 {
		return merkleFinalise(domain, 0, []byte{})
	}
	levels := merkleLevels(domain, leaves)
	return merkleFinalise(domain, len(leaves), levels[len(levels)-1][0])
}

// merkleProof returns the sibling path proving the leaf at index is in the tree.
func merkleProof(domain string, leaves [][]byte, index int) ([][]byte, error) {
	if index < 0 || index >= len(leaves) {
		return nil, fmt.Errorf("%w: index %d is outside a tree of %d leaves",
			ErrInvalidMerkleProof, index, len(leaves))
	}

	levels := merkleLevels(domain, leaves)
	path := make([][]byte, 0, len(levels))

	position := index
	for depth := 0; depth < len(levels)-1; depth++ {
		level := levels[depth]

		sibling := position ^ 1
		if sibling >= len(level) {
			// The odd node out is paired with itself.
			sibling = position
		}

		path = append(path, append([]byte{}, level[sibling]...))
		position /= 2
	}

	return path, nil
}

// merkleVerify checks an inclusion proof against a root.
//
// count is part of the statement being proved, not a hint: it is what the root
// commits to, so a proof cannot be replayed against a tree of a different size.
func merkleVerify(domain string, root, leaf []byte, index, count int, path [][]byte) bool {
	if index < 0 || count <= 0 || index >= count {
		return false
	}

	// The path must be exactly as deep as a tree of this size, or a truncated
	// path could stop at an internal node and pass it off as the apex.
	depth := 0
	for width := count; width > 1; width = (width + 1) / 2 {
		depth++
	}
	if len(path) != depth {
		return false
	}

	current := merkleLeafHash(domain, leaf)
	position := index

	for _, sibling := range path {
		if position%2 == 0 {
			current = merkleNodeHash(domain, current, sibling)
		} else {
			current = merkleNodeHash(domain, sibling, current)
		}
		position /= 2
	}

	// Roots and proofs are public commitments, so an ordinary comparison is
	// correct here; there is no secret whose compare time could leak.
	return bytes.Equal(merkleFinalise(domain, count, current), root)
}
