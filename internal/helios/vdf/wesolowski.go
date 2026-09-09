// File internal/helios/vdf/wesolowski.go - The Wesolowski verifiable delay
// function.
package vdf

import (
	"crypto/sha512"
	"encoding/binary"
	"errors"
	"fmt"
	"math/big"
)

const (
	// challengePrimeBits is the size of the Fiat-Shamir prime l.
	//
	// Soundness is roughly 1/l: a cheating prover must guess l before committing
	// to y, and 128 bits puts that far out of reach. Verification cost is
	// proportional to this, not to T, which is the entire point -- raising the
	// delay does not make checking it more expensive.
	challengePrimeBits = 128

	// maxChallengeAttempts bounds the search for that prime.
	maxChallengeAttempts = 1 << 20
)

var (
	// ErrInvalidProof is returned when a proof does not verify.
	ErrInvalidProof = errors.New("vdf proof is invalid")

	// ErrBadParameters is returned for a malformed evaluation request.
	ErrBadParameters = errors.New("vdf parameters are invalid")
)

// Proof is the output of a VDF evaluation.
//
// Output is x^(2^T); Witness is the Wesolowski proof that lets a verifier check
// it in roughly log(l) group operations instead of redoing T squarings.
type Proof struct {
	Output  Form
	Witness Form
	// Iterations is T, the number of sequential squarings claimed. It is part of
	// the Fiat-Shamir challenge, so a proof cannot be replayed at a different
	// difficulty.
	Iterations uint64
}

// Evaluate computes x^(2^T) and a proof of it.
//
// The T squarings are inherently sequential: each needs the previous result, so
// no amount of parallelism shortens the wall-clock time. That is the delay. What
// Wesolowski adds is the witness, which makes checking the delay cheap.
//
// Cost is about 2T squarings -- T for the output, T more for the witness. The
// witness loop is the long-division trick: it accumulates x^floor(2^T/l) without
// ever materialising that astronomically large exponent.
func Evaluate(input Form, iterations uint64, d *big.Int) (Proof, error) {
	if err := ValidateDiscriminant(d); err != nil {
		return Proof{}, err
	}
	if iterations == 0 {
		return Proof{}, fmt.Errorf("%w: iterations must be positive", ErrBadParameters)
	}
	if input.Discriminant().Cmp(d) != 0 {
		return Proof{}, fmt.Errorf("%w: input is not of the given discriminant", ErrBadParameters)
	}

	// The delay itself.
	output := Reduce(input.Clone())
	for i := uint64(0); i < iterations; i++ {
		var err error
		output, err = Square(output)
		if err != nil {
			return Proof{}, fmt.Errorf("squaring at step %d: %w", i, err)
		}
	}

	witness, err := proveWesolowski(input, output, iterations, d)
	if err != nil {
		return Proof{}, err
	}

	return Proof{Output: output, Witness: witness, Iterations: iterations}, nil
}

// proveWesolowski builds the witness pi = x^floor(2^T / l).
//
// floor(2^T / l) has about T bits, so it cannot be computed and then used as an
// exponent for any T worth having. Instead the quotient is accumulated one bit at
// a time alongside a running remainder, which is the standard long division:
//
//	pi = 1, r = 1
//	repeat T times:  b = floor(2r/l), r = 2r mod l, pi = pi^2 * x^b
//
// Each step costs one squaring and at most one multiplication.
func proveWesolowski(input, output Form, iterations uint64, d *big.Int) (Form, error) {
	l, err := challengePrime(input, output, iterations, d)
	if err != nil {
		return Form{}, err
	}

	pi := Identity(d)
	r := big.NewInt(1)
	quotientBit := new(big.Int)

	for i := uint64(0); i < iterations; i++ {
		// b = floor(2r / l), r = 2r mod l
		r.Lsh(r, 1)
		quotientBit.DivMod(r, l, r)

		pi, err = Square(pi)
		if err != nil {
			return Form{}, fmt.Errorf("witness squaring at step %d: %w", i, err)
		}
		if quotientBit.Sign() != 0 {
			pi, err = Compose(pi, input)
			if err != nil {
				return Form{}, fmt.Errorf("witness multiply at step %d: %w", i, err)
			}
		}
	}

	return pi, nil
}

// Verify checks a proof without repeating the delay.
//
// The identity is pi^l * x^r == y, where r = 2^T mod l. Both exponents are at
// most l, so verification is a fixed few hundred group operations regardless of
// how large T was -- a verifier confirms a delay of a billion squarings at the
// same cost as one of a thousand.
func Verify(input Form, proof Proof, d *big.Int) error {
	if err := ValidateDiscriminant(d); err != nil {
		return err
	}
	if proof.Iterations == 0 {
		return fmt.Errorf("%w: iterations must be positive", ErrBadParameters)
	}
	if input.Discriminant().Cmp(d) != 0 {
		return fmt.Errorf("%w: input is not of the given discriminant", ErrInvalidProof)
	}
	if proof.Output.Discriminant().Cmp(d) != 0 {
		return fmt.Errorf("%w: output is not of the given discriminant", ErrInvalidProof)
	}
	if proof.Witness.Discriminant().Cmp(d) != 0 {
		return fmt.Errorf("%w: witness is not of the given discriminant", ErrInvalidProof)
	}
	// Unreduced elements would break the equality check below, which compares
	// canonical representatives.
	if !proof.Output.IsReduced() || !proof.Witness.IsReduced() {
		return fmt.Errorf("%w: proof elements are not reduced", ErrInvalidProof)
	}

	// The challenge is bound to the input, the output and T. Recomputing it here
	// rather than trusting a supplied value is what stops a prover choosing an l
	// that makes an arbitrary y check out.
	l, err := challengePrime(input, proof.Output, proof.Iterations, d)
	if err != nil {
		return err
	}

	// r = 2^T mod l
	exponent := new(big.Int).SetUint64(proof.Iterations)
	r := new(big.Int).Exp(two, exponent, l)

	piL, err := Exp(proof.Witness, l, d)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}
	xR, err := Exp(input, r, d)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	candidate, err := Compose(piL, xR)
	if err != nil {
		return fmt.Errorf("verify: %w", err)
	}

	if !candidate.Equal(proof.Output) {
		return ErrInvalidProof
	}
	return nil
}

// challengePrime derives the Fiat-Shamir prime l from the whole statement.
//
// Every field the proof asserts goes into the hash. Leaving any of them out
// would let a prover reuse a witness across statements -- most obviously, if T
// were excluded, one proof would serve for every delay length.
func challengePrime(input, output Form, iterations uint64, d *big.Int) (*big.Int, error) {
	base := sha512.New()
	base.Write([]byte("gbb/vdf/wesolowski/challenge/v1"))
	base.Write(lengthPrefixed(d.Bytes()))
	base.Write(lengthPrefixed(input.Bytes()))
	base.Write(lengthPrefixed(output.Bytes()))

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], iterations)
	base.Write(buf[:])
	digest := base.Sum(nil)

	for counter := uint64(0); counter < maxChallengeAttempts; counter++ {
		h := sha512.New()
		h.Write(digest)
		binary.BigEndian.PutUint64(buf[:], counter)
		h.Write(buf[:])

		candidate := new(big.Int).SetBytes(h.Sum(nil)[:challengePrimeBits/8])
		candidate.SetBit(candidate, challengePrimeBits-1, 1) // full size
		candidate.SetBit(candidate, 0, 1)                    // odd

		if candidate.ProbablyPrime(32) {
			return candidate, nil
		}
	}

	return nil, errors.New("could not derive a challenge prime")
}

// lengthPrefixed guards against concatenation ambiguity: without the prefix, two
// different statements could produce identical hash input and so share a
// challenge.
func lengthPrefixed(b []byte) []byte {
	assertEncodableLength(len(b))
	out := make([]byte, 4+len(b))
	binary.BigEndian.PutUint32(out[:4], uint32(len(b))) //nolint:gosec // bounded by assertEncodableLength above
	copy(out[4:], b)
	return out
}

// Bytes encodes a proof for transport.
func (p Proof) Bytes() []byte {
	output := p.Output.Bytes()
	witness := p.Witness.Bytes()

	out := make([]byte, 0, 16+len(output)+len(witness))
	var buf [8]byte

	binary.BigEndian.PutUint64(buf[:], p.Iterations)
	out = append(out, buf[:]...)
	out = append(out, lengthPrefixed(output)...)
	out = append(out, lengthPrefixed(witness)...)
	return out
}

// ProofFromBytes decodes a proof.
func ProofFromBytes(data []byte, d *big.Int) (Proof, error) {
	if len(data) < 16 {
		return Proof{}, errors.New("encoded proof is too short")
	}

	iterations := binary.BigEndian.Uint64(data[:8])
	offset := 8

	readField := func() ([]byte, error) {
		if offset+4 > len(data) {
			return nil, errors.New("encoded proof is truncated")
		}
		length := int(binary.BigEndian.Uint32(data[offset:]))
		offset += 4
		if length < 0 || offset+length > len(data) {
			return nil, errors.New("encoded proof has a bad field length")
		}
		field := data[offset : offset+length]
		offset += length
		return field, nil
	}

	outputBytes, err := readField()
	if err != nil {
		return Proof{}, err
	}
	witnessBytes, err := readField()
	if err != nil {
		return Proof{}, err
	}

	output, err := FormFromBytes(outputBytes, d)
	if err != nil {
		return Proof{}, fmt.Errorf("decode output: %w", err)
	}
	witness, err := FormFromBytes(witnessBytes, d)
	if err != nil {
		return Proof{}, fmt.Errorf("decode witness: %w", err)
	}

	return Proof{Output: output, Witness: witness, Iterations: iterations}, nil
}
