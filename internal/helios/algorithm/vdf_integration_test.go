package algorithm

import (
	"bytes"
	"testing"
	"time"
)

// These cover stage 2 as it sits inside Helios, rather than the VDF in isolation.

func vdfTestAlgorithm() *HeliosAlgorithm {
	return NewHeliosAlgorithm(TestHeliosConfig())
}

// TestMinedProofCarriesAVerifiableDelay is the end-to-end contract.
func TestMinedProofCarriesAVerifiableDelay(t *testing.T) {
	h := vdfTestAlgorithm()
	header := []byte("a block header")

	proof, err := h.Mine(header, easyTarget())
	if err != nil {
		t.Fatalf("mine: %v", err)
	}

	if len(proof.VDFProof) == 0 {
		t.Fatal("the mined proof carries no delay proof")
	}
	if err := h.ValidateProof(proof, header, easyTarget()); err != nil {
		t.Fatalf("a freshly mined proof did not validate: %v", err)
	}
}

// TestForgedDelayIsRejected. Without this the stage proves nothing: a miner
// would assert a stage-2 output and skip the sequential work.
func TestForgedDelayIsRejected(t *testing.T) {
	h := vdfTestAlgorithm()
	header := []byte("forgery test header")

	proof, err := h.Mine(header, easyTarget())
	if err != nil {
		t.Fatalf("mine: %v", err)
	}

	t.Run("substituted output", func(t *testing.T) {
		forged := *proof
		forged.Stage2Result = append([]byte{}, proof.Stage2Result...)
		forged.Stage2Result[0] ^= 0xff

		if err := h.ValidateProof(&forged, header, easyTarget()); err == nil {
			t.Fatal("a fabricated stage-2 output validated")
		}
	})

	t.Run("missing witness", func(t *testing.T) {
		forged := *proof
		forged.VDFProof = nil

		if err := h.ValidateProof(&forged, header, easyTarget()); err == nil {
			t.Fatal("a proof with no delay witness validated")
		}
	})

	t.Run("corrupted witness", func(t *testing.T) {
		forged := *proof
		forged.VDFProof = append([]byte{}, proof.VDFProof...)
		forged.VDFProof[len(forged.VDFProof)-1] ^= 0xff

		if err := h.ValidateProof(&forged, header, easyTarget()); err == nil {
			t.Fatal("a corrupted delay witness validated")
		}
	})

	t.Run("witness from another header", func(t *testing.T) {
		other, err := h.Mine([]byte("a different header"), easyTarget())
		if err != nil {
			t.Fatalf("mine: %v", err)
		}

		forged := *proof
		forged.Stage2Result = other.Stage2Result
		forged.VDFProof = other.VDFProof

		if err := h.ValidateProof(&forged, header, easyTarget()); err == nil {
			t.Fatal("a delay proof computed for another block validated here")
		}
	})
}

// TestDelayIsBoundToTheBlockNotTheNonce.
//
// The input is derived from the header alone. If it depended on the nonce, each
// attempt would start an independent chain -- so a miner with n cores would run n
// of them at once and the block would still cost one chain of wall-clock time,
// which is no delay at all at the level that matters.
func TestDelayIsBoundToTheBlockNotTheNonce(t *testing.T) {
	h := vdfTestAlgorithm()
	header := []byte("nonce independence")

	first, _, err := h.executeTimeLockPhase(header)
	if err != nil {
		t.Fatalf("time-lock: %v", err)
	}
	second, _, err := h.executeTimeLockPhase(header)
	if err != nil {
		t.Fatalf("time-lock: %v", err)
	}

	if !bytes.Equal(first, second) {
		t.Fatal("the same header produced two different delay outputs")
	}

	other, _, err := h.executeTimeLockPhase([]byte("a different header"))
	if err != nil {
		t.Fatalf("time-lock: %v", err)
	}
	if bytes.Equal(first, other) {
		t.Fatal("two different headers produced the same delay output")
	}
}

// TestValidationDoesNotRepeatTheDelay is the reason for the whole change.
//
// The old stage 2 was a sequential hash chain that every verifier had to walk in
// full, so the delay could never be set higher than validators were willing to
// pay. Checking a Wesolowski witness costs a few hundred group operations
// whatever the delay was.
func TestValidationDoesNotRepeatTheDelay(t *testing.T) {
	cfg := TestHeliosConfig()
	cfg.VDFIterations = 3000 // long enough for the difference to be unambiguous
	h := NewHeliosAlgorithm(cfg)
	header := []byte("cost comparison")

	start := time.Now()
	proof, err := h.Mine(header, easyTarget())
	if err != nil {
		t.Fatalf("mine: %v", err)
	}
	mineTime := time.Since(start)

	start = time.Now()
	if err := h.ValidateProof(proof, header, easyTarget()); err != nil {
		t.Fatalf("validate: %v", err)
	}
	validateTime := time.Since(start)

	t.Logf("mine = %s, validate = %s (%.1fx cheaper)",
		mineTime, validateTime, float64(mineTime)/float64(validateTime))

	if validateTime >= mineTime {
		t.Fatalf("validation (%s) costs as much as mining (%s); the proof is "+
			"buying nothing", validateTime, mineTime)
	}
}

// TestDiscriminantIsStableAcrossInstances: two nodes must derive the same group,
// or they cannot check each other's proofs.
func TestDiscriminantIsStableAcrossInstances(t *testing.T) {
	first := vdfTestAlgorithm()
	second := vdfTestAlgorithm()

	d1, iterations1, err := first.vdfParameters()
	if err != nil {
		t.Fatalf("parameters: %v", err)
	}
	d2, iterations2, err := second.vdfParameters()
	if err != nil {
		t.Fatalf("parameters: %v", err)
	}

	if d1.Cmp(d2) != 0 {
		t.Fatal("two instances derived different discriminants; they could not " +
			"verify each other's delay proofs")
	}
	if iterations1 != iterations2 {
		t.Fatal("two instances disagree on the delay length")
	}

	// And a proof from one validates under the other.
	header := []byte("cross-instance")
	proof, err := first.Mine(header, easyTarget())
	if err != nil {
		t.Fatalf("mine: %v", err)
	}
	if err := second.ValidateProof(proof, header, easyTarget()); err != nil {
		t.Fatalf("a proof from one instance failed on another: %v", err)
	}
}

// TestProofRejectsTheWrongIterationCount: a short delay must not pass for a long
// one, which is exactly what a miner would want.
func TestProofRejectsTheWrongIterationCount(t *testing.T) {
	cheap := TestHeliosConfig()
	cheap.VDFIterations = 16
	miner := NewHeliosAlgorithm(cheap)

	header := []byte("iteration mismatch")
	proof, err := miner.Mine(header, easyTarget())
	if err != nil {
		t.Fatalf("mine: %v", err)
	}

	// A validator that requires a longer delay must refuse it.
	strict := TestHeliosConfig()
	strict.VDFIterations = 256
	validator := NewHeliosAlgorithm(strict)

	if err := validator.ValidateProof(proof, header, easyTarget()); err == nil {
		t.Fatal("a 16-iteration delay was accepted where 256 were required")
	}
}
