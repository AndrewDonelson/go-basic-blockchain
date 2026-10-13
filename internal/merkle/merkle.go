// Package merkle builds domain-separated, count-bound Merkle trees with inclusion
// proofs. It is shared by the node (sdk) and the dependency-free sidechain
// package, so a root computed by an application and one computed by a node can
// never disagree.
package merkle

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
)

// ErrInvalidProof is returned when an inclusion proof does not check out.
var ErrInvalidProof = errors.New("invalid merkle proof")

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

// leafHash hashes one leaf under a domain.
func leafHash(domain string, leaf []byte) []byte {
	h := sha256.New()
	h.Write([]byte(domain))
	h.Write([]byte("/leaf"))
	h.Write(leaf)
	return h.Sum(nil)
}

// nodeHash hashes a pair of children under a domain.
func nodeHash(domain string, left, right []byte) []byte {
	h := sha256.New()
	h.Write([]byte(domain))
	h.Write([]byte("/node"))
	h.Write(left)
	h.Write(right)
	return h.Sum(nil)
}

// finalise binds the leaf count into the root.
func finalise(domain string, count int, apex []byte) []byte {
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

// buildLevels builds every level of the tree, leaves first.
func buildLevels(domain string, leaves [][]byte) [][][]byte {
	level := make([][]byte, 0, len(leaves))
	for _, leaf := range leaves {
		level = append(level, leafHash(domain, leaf))
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
			next = append(next, nodeHash(domain, left, right))
		}
		level = next
		levels = append(levels, level)
	}
	return levels
}

// Root returns the root over a set of leaves.
//
// An empty set has its own root rather than a zero value, so "committed to
// nothing" and "committed to a leaf that happens to hash to zero" stay distinct.
func Root(domain string, leaves [][]byte) []byte {
	if len(leaves) == 0 {
		return finalise(domain, 0, []byte{})
	}
	levels := buildLevels(domain, leaves)
	return finalise(domain, len(leaves), levels[len(levels)-1][0])
}

// Proof returns the sibling path proving the leaf at index is in the tree.
func Proof(domain string, leaves [][]byte, index int) ([][]byte, error) {
	if index < 0 || index >= len(leaves) {
		return nil, fmt.Errorf("%w: index %d is outside a tree of %d leaves",
			ErrInvalidProof, index, len(leaves))
	}

	levels := buildLevels(domain, leaves)
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

// Verify checks an inclusion proof against a root.
//
// count is part of the statement being proved, not a hint: it is what the root
// commits to, so a proof cannot be replayed against a tree of a different size.
func Verify(domain string, root, leaf []byte, index, count int, path [][]byte) bool {
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

	current := leafHash(domain, leaf)
	position := index

	for _, sibling := range path {
		if position%2 == 0 {
			current = nodeHash(domain, current, sibling)
		} else {
			current = nodeHash(domain, sibling, current)
		}
		position /= 2
	}

	// Roots and proofs are public commitments, so an ordinary comparison is
	// correct here; there is no secret whose compare time could leak.
	return bytes.Equal(finalise(domain, count, current), root)
}
