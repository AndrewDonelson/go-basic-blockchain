package vdf

import (
	"errors"
	"math/big"
	"testing"
	"time"
)

// -----------------------------------------------------------------------------
// Correctness
// -----------------------------------------------------------------------------

func TestEvaluateAndVerify(t *testing.T) {
	d := testDiscriminant(t)
	input := testForm(t, d, "vdf-input")

	for _, iterations := range []uint64{1, 2, 10, 100, 1000} {
		proof, err := Evaluate(input, iterations, d)
		if err != nil {
			t.Fatalf("evaluate %d: %v", iterations, err)
		}
		if err := Verify(input, proof, d); err != nil {
			t.Fatalf("a valid proof for %d iterations did not verify: %v",
				iterations, err)
		}
	}
}

// TestOutputMatchesRepeatedSquaring: the output must genuinely be x^(2^T), not
// merely something the witness happens to satisfy.
func TestOutputMatchesRepeatedSquaring(t *testing.T) {
	d := testDiscriminant(t)
	input := testForm(t, d, "squaring-check")
	const iterations = 64

	proof, err := Evaluate(input, iterations, d)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	// Independently: x^(2^64) via exponentiation rather than the squaring loop.
	exponent := new(big.Int).Lsh(big.NewInt(1), iterations)
	expected, err := Exp(input, exponent, d)
	if err != nil {
		t.Fatalf("exp: %v", err)
	}

	if !proof.Output.Equal(expected) {
		t.Fatal("the VDF output is not x^(2^T)")
	}
}

// TestEvaluationIsDeterministic: every node must derive the same output, or they
// cannot agree on whether a block is valid.
func TestEvaluationIsDeterministic(t *testing.T) {
	d := testDiscriminant(t)
	input := testForm(t, d, "deterministic")

	first, err := Evaluate(input, 128, d)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	for i := 0; i < 3; i++ {
		again, err := Evaluate(input, 128, d)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		if !again.Output.Equal(first.Output) {
			t.Fatal("evaluation is not deterministic")
		}
		if !again.Witness.Equal(first.Witness) {
			t.Fatal("the witness is not deterministic")
		}
	}
}

// -----------------------------------------------------------------------------
// Soundness -- the part that matters
// -----------------------------------------------------------------------------

// TestForgedOutputIsRejected. Without this the VDF proves nothing: a miner would
// simply assert an output and skip the delay.
func TestForgedOutputIsRejected(t *testing.T) {
	d := testDiscriminant(t)
	input := testForm(t, d, "forge")

	proof, err := Evaluate(input, 200, d)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	// Any other group element in place of the real output.
	forged := proof
	forged.Output = testForm(t, d, "not-the-answer")

	if err := Verify(input, forged, d); !errors.Is(err, ErrInvalidProof) {
		t.Fatalf("a forged output verified (err=%v)", err)
	}
}

func TestForgedWitnessIsRejected(t *testing.T) {
	d := testDiscriminant(t)
	input := testForm(t, d, "forge-witness")

	proof, err := Evaluate(input, 200, d)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	forged := proof
	forged.Witness = testForm(t, d, "wrong-witness")

	if err := Verify(input, forged, d); !errors.Is(err, ErrInvalidProof) {
		t.Fatalf("a forged witness verified (err=%v)", err)
	}

	// The identity is the cheapest possible forgery, so check it explicitly.
	forged.Witness = Identity(d)
	if err := Verify(input, forged, d); !errors.Is(err, ErrInvalidProof) {
		t.Fatalf("an identity witness verified (err=%v)", err)
	}
}

// TestProofIsBoundToItsIterationCount: a proof of a short delay must not pass as
// a proof of a long one, which is what a miner would want.
func TestProofIsBoundToItsIterationCount(t *testing.T) {
	d := testDiscriminant(t)
	input := testForm(t, d, "iteration-binding")

	proof, err := Evaluate(input, 100, d)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	replayed := proof
	replayed.Iterations = 1000000

	if err := Verify(input, replayed, d); err == nil {
		t.Fatal("a 100-iteration proof passed as a 1,000,000-iteration proof; the " +
			"claimed delay is not bound to the proof")
	}
}

// TestProofIsBoundToItsInput: a proof must not be transferable to another input.
func TestProofIsBoundToItsInput(t *testing.T) {
	d := testDiscriminant(t)
	input := testForm(t, d, "input-a")
	other := testForm(t, d, "input-b")

	proof, err := Evaluate(input, 100, d)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	if err := Verify(other, proof, d); err == nil {
		t.Fatal("a proof for one input verified against another")
	}
}

// TestChallengeDependsOnTheWholeStatement. If any field were left out of the
// Fiat-Shamir hash, a witness could be reused across statements.
func TestChallengeDependsOnTheWholeStatement(t *testing.T) {
	d := testDiscriminant(t)
	x := testForm(t, d, "challenge-x")
	y := testForm(t, d, "challenge-y")
	z := testForm(t, d, "challenge-z")

	base, err := challengePrime(x, y, 100, d)
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}

	byInput, err := challengePrime(z, y, 100, d)
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}
	byOutput, err := challengePrime(x, z, 100, d)
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}
	byIterations, err := challengePrime(x, y, 101, d)
	if err != nil {
		t.Fatalf("challenge: %v", err)
	}

	for name, other := range map[string]*big.Int{
		"input":      byInput,
		"output":     byOutput,
		"iterations": byIterations,
	} {
		if base.Cmp(other) == 0 {
			t.Fatalf("changing the %s did not change the challenge", name)
		}
	}

	if !base.ProbablyPrime(32) {
		t.Fatal("the challenge is not prime")
	}
	if base.BitLen() != challengePrimeBits {
		t.Fatalf("the challenge is %d bits, want %d", base.BitLen(), challengePrimeBits)
	}
}

func TestVerifyRejectsMalformedProofs(t *testing.T) {
	d := testDiscriminant(t)
	input := testForm(t, d, "malformed")

	proof, err := Evaluate(input, 50, d)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	t.Run("zero iterations", func(t *testing.T) {
		bad := proof
		bad.Iterations = 0
		if err := Verify(input, bad, d); err == nil {
			t.Fatal("a zero-iteration proof verified")
		}
	})

	t.Run("wrong discriminant", func(t *testing.T) {
		other, err := NewDiscriminant([]byte("a different group"), 256)
		if err != nil {
			t.Fatalf("discriminant: %v", err)
		}
		if err := Verify(input, proof, other); err == nil {
			t.Fatal("a proof verified against a different group")
		}
	})

	t.Run("unreduced elements", func(t *testing.T) {
		bad := proof
		bad.Output = Form{
			A: new(big.Int).Set(proof.Output.A),
			B: new(big.Int).Add(proof.Output.B, new(big.Int).Lsh(proof.Output.A, 1)),
			C: new(big.Int).Add(new(big.Int).Add(proof.Output.C, proof.Output.A), proof.Output.B),
		}
		if bad.Output.IsReduced() {
			t.Skip("the shifted form stayed reduced")
		}
		if err := Verify(input, bad, d); err == nil {
			t.Fatal("an unreduced output verified")
		}
	})
}

func TestEvaluateRejectsBadParameters(t *testing.T) {
	d := testDiscriminant(t)
	input := testForm(t, d, "params")

	if _, err := Evaluate(input, 0, d); !errors.Is(err, ErrBadParameters) {
		t.Fatalf("zero iterations were accepted: %v", err)
	}
	if _, err := Evaluate(input, 10, big.NewInt(-4)); err == nil {
		t.Fatal("a bad discriminant was accepted")
	}
}

// -----------------------------------------------------------------------------
// The property the whole exercise is for
// -----------------------------------------------------------------------------

// TestVerificationIsCheaperThanEvaluation is the reason for replacing the hash
// chain. The old stage 2 made every verifier redo all T sequential steps;
// Wesolowski's cost is fixed by the challenge size, not by T.
func TestVerificationIsCheaperThanEvaluation(t *testing.T) {
	d := testDiscriminant(t)
	input := testForm(t, d, "cost")

	const iterations = 2000

	start := time.Now()
	proof, err := Evaluate(input, iterations, d)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	evalTime := time.Since(start)

	start = time.Now()
	if err := Verify(input, proof, d); err != nil {
		t.Fatalf("verify: %v", err)
	}
	verifyTime := time.Since(start)

	t.Logf("evaluate(%d) = %s, verify = %s, ratio = %.1fx",
		iterations, evalTime, verifyTime,
		float64(evalTime)/float64(verifyTime))

	if verifyTime >= evalTime {
		t.Fatalf("verification (%s) is not cheaper than evaluation (%s); the "+
			"proof is buying nothing", verifyTime, evalTime)
	}
}

// TestVerificationCostDoesNotGrowWithDelay is the stronger claim: doubling the
// delay must not make checking it more expensive, or the delay cannot be raised.
func TestVerificationCostDoesNotGrowWithDelay(t *testing.T) {
	d := testDiscriminant(t)
	input := testForm(t, d, "scaling")

	measure := func(iterations uint64) time.Duration {
		proof, err := Evaluate(input, iterations, d)
		if err != nil {
			t.Fatalf("evaluate: %v", err)
		}
		// Several passes: a single measurement at this scale is mostly noise.
		start := time.Now()
		for i := 0; i < 10; i++ {
			if err := Verify(input, proof, d); err != nil {
				t.Fatalf("verify: %v", err)
			}
		}
		return time.Since(start) / 10
	}

	small := measure(500)
	large := measure(4000)

	t.Logf("verify after 500 iterations: %s; after 4000: %s", small, large)

	// An eightfold increase in delay must not produce a large increase in
	// verification cost. The bound is loose because these are microsecond-scale
	// timings, but a cost proportional to T would blow straight through it.
	if large > small*3 {
		t.Fatalf("verification cost grew from %s to %s for an 8x longer delay; "+
			"it should be independent of T", small, large)
	}
}

// -----------------------------------------------------------------------------
// Encoding
// -----------------------------------------------------------------------------

func TestProofEncodingRoundTrips(t *testing.T) {
	d := testDiscriminant(t)
	input := testForm(t, d, "encode")

	proof, err := Evaluate(input, 128, d)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}

	decoded, err := ProofFromBytes(proof.Bytes(), d)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}

	if decoded.Iterations != proof.Iterations {
		t.Fatalf("iterations changed: %d vs %d", decoded.Iterations, proof.Iterations)
	}
	if !decoded.Output.Equal(proof.Output) || !decoded.Witness.Equal(proof.Witness) {
		t.Fatal("the round trip changed the proof")
	}

	// And the decoded proof still verifies, which is what actually matters.
	if err := Verify(input, decoded, d); err != nil {
		t.Fatalf("a decoded proof did not verify: %v", err)
	}
}

func TestProofDecodingRejectsGarbage(t *testing.T) {
	d := testDiscriminant(t)

	for _, bad := range [][]byte{
		nil,
		{},
		make([]byte, 8),
		make([]byte, 20),
		append(make([]byte, 8), 0, 0, 0, 255),
	} {
		if _, err := ProofFromBytes(bad, d); err == nil {
			t.Fatalf("decoded garbage of length %d", len(bad))
		}
	}
}
