// File internal/helios/vdf/parameters.go - The published class group parameters.
package vdf

import (
	"fmt"
	"math/big"
	"sync"
)

// DefaultDiscriminantSeed is the public string the group is derived from.
//
// There is no secret behind it, which is the entire point: an RSA-based VDF needs
// a modulus whose factorisation somebody knows, and that somebody could compute
// the group order and skip the delay. Here the seed, the derivation and the
// result are all public, and none of them reveals the class number.
const DefaultDiscriminantSeed = "gbb/helios/vdf/discriminant/v1"

// DefaultDiscriminantBits is the size of the published discriminant.
//
// 2048 bits. The security of a class group rests on the class number being hard
// to compute, and the best known algorithms for that are subexponential in the
// size of the discriminant -- the same shape as factoring, so the sizing
// intuition carries over from RSA. 1024 would be defensible (it is what Chia
// runs); 2048 costs about 4.7x per group operation and buys a margin that does
// not have to be revisited.
const DefaultDiscriminantBits = 2048

// defaultDiscriminantHex is |D| for the seed and size above, precomputed.
//
// Deriving it means searching for a 2048-bit prime, which takes on the order of
// half a second -- far too long to repeat at every startup for a value that can
// never change. It is embedded rather than cached because it is a *constant of
// the protocol*: every node must use exactly this group or they cannot check each
// other's proofs.
//
// Nothing here has to be taken on trust. TestPublishedDiscriminantMatchesItsSeed
// re-derives it from DefaultDiscriminantSeed and fails if it differs, so the
// claim "this was produced by public derivation, not chosen" is checked on every
// test run rather than asserted in a comment.
const defaultDiscriminantHex = "da3ea25d0a7dfc1b3c94b0ad159c1f72a8c345113be57b6c79731b58226b136e" +
	"0425d024cdbe67b19a563db5837026043861806c5a6fe2797d1596601e858772" +
	"8c74950452dc22d90fa33e8d7785fb497fae742398c028f48fcf4b08639048a6" +
	"a18c918d54d60bb0a0f15a858ef3b324f5794761e6474f518fa68ffa752d39f1" +
	"2c2980c78b78ed782f8fef1b5e30e0f6c6877c9070931ee7f7a5bca14240c5cc" +
	"0307c9263cd2a1bee74c798607eb7d2fa9292d4eb87fecc107c83b254071092b" +
	"8b95155ac8dd7abd229b00e7090d9057917155378f1db65152b5d2012bf388b3" +
	"f75ceb209eb18eddaefd7e2b4c02678332749f423a8a9ed12e11f76585400097"

var (
	defaultDiscriminantOnce sync.Once
	defaultDiscriminant     *big.Int
	defaultDiscriminantErr  error
)

// DefaultDiscriminant returns the published group discriminant.
//
// Parsing a hex constant, not searching for a prime: this is cheap enough to call
// freely.
func DefaultDiscriminant() (*big.Int, error) {
	defaultDiscriminantOnce.Do(func() {
		magnitude, ok := new(big.Int).SetString(defaultDiscriminantHex, 16)
		if !ok {
			defaultDiscriminantErr = fmt.Errorf("the embedded discriminant is not valid hex")
			return
		}
		d := new(big.Int).Neg(magnitude)
		if err := ValidateDiscriminant(d); err != nil {
			defaultDiscriminantErr = fmt.Errorf("the embedded discriminant is unusable: %w", err)
			return
		}
		defaultDiscriminant = d
	})

	if defaultDiscriminantErr != nil {
		return nil, defaultDiscriminantErr
	}
	return defaultDiscriminant, nil
}
