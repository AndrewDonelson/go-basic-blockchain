// Package sdk is a software development kit for building blockchain applications.
// File sdk/merkle.go - Merkle trees with inclusion proofs (see internal/merkle).
package sdk

import "github.com/AndrewDonelson/go-basic-blockchain/internal/merkle"

// ErrInvalidMerkleProof is returned when an inclusion proof does not check out.
var ErrInvalidMerkleProof = merkle.ErrInvalidProof

// The trees live in internal/merkle so the dependency-free sidechain package
// computes exactly the roots the node does. These keep the node's call sites.

func merkleRoot(domain string, leaves [][]byte) []byte { return merkle.Root(domain, leaves) }

func merkleProof(domain string, leaves [][]byte, index int) ([][]byte, error) {
	return merkle.Proof(domain, leaves, index)
}

func merkleVerify(domain string, root, leaf []byte, index, count int, path [][]byte) bool {
	return merkle.Verify(domain, root, leaf, index, count, path)
}
