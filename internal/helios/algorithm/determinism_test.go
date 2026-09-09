package algorithm

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"testing"
	"time"
)

// fastConfig is a Helios profile small enough to mine repeatedly in a test.
func fastConfig() *HeliosConfig {
	cfg := TestHeliosConfig()
	cfg.MiningTimeout = 20 * time.Second
	return cfg
}

// easyTarget accepts almost any hash, so tests exercise verification rather than
// spending their time on the search.
func easyTarget() *big.Int {
	return new(big.Int).Lsh(big.NewInt(1), 255)
}

// TestStagesAreDeterministic is the core guard for the defect that made Helios
// not a proof of work: executeCryptographicPhase drew its AES key and nonce from
// crypto/rand, so stage 3 could not be recomputed by a verifier. ValidateProof
// had to trust the stage outputs the miner supplied, which meant a miner could
// invent all three stage results and brute-force only the cheap final SHA-256 --
// skipping the memory-hard phase, the sequential phase and the cipher phase
// entirely, at zero cost.
func TestStagesAreDeterministic(t *testing.T) {
	h := NewHeliosAlgorithm(fastConfig())
	header := []byte("block-header-under-test")

	stage1a, err := h.executeMemoryPhase(header, 42)
	if err != nil {
		t.Fatalf("memory phase: %v", err)
	}
	stage1b, err := h.executeMemoryPhase(header, 42)
	if err != nil {
		t.Fatalf("memory phase: %v", err)
	}
	if !bytes.Equal(stage1a, stage1b) {
		t.Fatal("stage 1 is not deterministic")
	}

	// Stage 2 takes the block header, not the stage-1 result: the delay is a
	// property of the block, not of a nonce attempt.
	stage2a, proofA, err := h.executeTimeLockPhase(header)
	if err != nil {
		t.Fatalf("time-lock phase: %v", err)
	}
	stage2b, proofB, err := h.executeTimeLockPhase(header)
	if err != nil {
		t.Fatalf("time-lock phase: %v", err)
	}
	if !bytes.Equal(stage2a, stage2b) {
		t.Fatal("stage 2 is not deterministic")
	}
	if !bytes.Equal(proofA, proofB) {
		t.Fatal("the delay proof is not deterministic")
	}

	stage3a, err := h.executeCryptographicPhase(stage2a)
	if err != nil {
		t.Fatalf("crypto phase: %v", err)
	}
	stage3b, err := h.executeCryptographicPhase(stage2b)
	if err != nil {
		t.Fatalf("crypto phase: %v", err)
	}
	if !bytes.Equal(stage3a, stage3b) {
		t.Fatal("stage 3 is not deterministic -- the proof cannot be verified")
	}

	// A different nonce must produce a different chain of results.
	other, err := h.executeMemoryPhase(header, 43)
	if err != nil {
		t.Fatalf("memory phase: %v", err)
	}
	if bytes.Equal(stage1a, other) {
		t.Fatal("stage 1 ignores the nonce")
	}
}

// TestMinedProofValidates is the round trip: what Mine produces, ValidateProof
// must accept.
func TestMinedProofValidates(t *testing.T) {
	h := NewHeliosAlgorithm(fastConfig())
	header := []byte("mine-and-verify")
	target := easyTarget()

	proof, err := h.Mine(header, target)
	if err != nil {
		t.Fatalf("mine: %v", err)
	}

	if err := h.ValidateProof(proof, header, target); err != nil {
		t.Fatalf("a freshly mined proof must validate: %v", err)
	}
}

// TestValidateProofRejectsFabricatedStages is the attack the old design allowed:
// a miner supplies arbitrary stage results and grinds only the final hash.
// Validation now recomputes every stage from the header, so fabricated stages are
// rejected regardless of whether the final hash is consistent with them.
func TestValidateProofRejectsFabricatedStages(t *testing.T) {
	h := NewHeliosAlgorithm(fastConfig())
	header := []byte("fabrication-test")
	target := easyTarget()

	proof, err := h.Mine(header, target)
	if err != nil {
		t.Fatalf("mine: %v", err)
	}

	t.Run("stage1 replaced", func(t *testing.T) {
		bad := *proof
		bad.Stage1Result = bytes.Repeat([]byte{0xAA}, len(proof.Stage1Result))
		if err := h.ValidateProof(&bad, header, target); err == nil {
			t.Fatal("expected a fabricated stage 1 to be rejected")
		}
	})

	t.Run("stage2 replaced", func(t *testing.T) {
		bad := *proof
		bad.Stage2Result = bytes.Repeat([]byte{0xBB}, len(proof.Stage2Result))
		if err := h.ValidateProof(&bad, header, target); err == nil {
			t.Fatal("expected a fabricated stage 2 to be rejected")
		}
	})

	t.Run("stage3 replaced", func(t *testing.T) {
		bad := *proof
		bad.Stage3Result = bytes.Repeat([]byte{0xCC}, len(proof.Stage3Result))
		if err := h.ValidateProof(&bad, header, target); err == nil {
			t.Fatal("expected a fabricated stage 3 to be rejected")
		}
	})

	t.Run("self-consistent fabrication", func(t *testing.T) {
		// The strongest form of the attack: all three stages invented, and the
		// final hash recomputed to match them so the proof is internally
		// consistent. Only recomputation from the header catches this.
		bad := *proof
		bad.Stage1Result = []byte("invented-1")
		bad.Stage2Result = []byte("invented-2")
		bad.Stage3Result = []byte("invented-3")
		bad.FinalHash = hashHex(heliosFinalPreimage(header, bad.Nonce,
			bad.Stage1Result, bad.Stage2Result, bad.Stage3Result))

		if err := h.ValidateProof(&bad, header, target); err == nil {
			t.Fatal("expected an internally consistent but fabricated proof to be rejected")
		}
	})
}

// TestValidateProofRejectsWrongHeaderOrNonce guards proof reuse across blocks.
func TestValidateProofRejectsWrongHeaderOrNonce(t *testing.T) {
	h := NewHeliosAlgorithm(fastConfig())
	header := []byte("original-header")
	target := easyTarget()

	proof, err := h.Mine(header, target)
	if err != nil {
		t.Fatalf("mine: %v", err)
	}

	if err := h.ValidateProof(proof, []byte("different-header"), target); err == nil {
		t.Fatal("a proof must not validate against a different block header")
	}

	bad := *proof
	bad.Nonce = proof.Nonce + 1
	if err := h.ValidateProof(&bad, header, target); err == nil {
		t.Fatal("a proof must not validate with an altered nonce")
	}
}

// TestValidateProofRejectsHashAboveTarget guards the difficulty comparison.
func TestValidateProofRejectsHashAboveTarget(t *testing.T) {
	h := NewHeliosAlgorithm(fastConfig())
	header := []byte("target-test")

	proof, err := h.Mine(header, easyTarget())
	if err != nil {
		t.Fatalf("mine: %v", err)
	}

	// An impossibly strict target must reject the proof.
	if err := h.ValidateProof(proof, header, big.NewInt(1)); err == nil {
		t.Fatal("expected a proof to be rejected against an unreachable target")
	}
}

// TestValidateProofRejectsNilInputs covers the defensive paths.
func TestValidateProofRejectsNilInputs(t *testing.T) {
	h := NewHeliosAlgorithm(fastConfig())

	if err := h.ValidateProof(nil, []byte("h"), easyTarget()); err == nil {
		t.Fatal("expected an error for a nil proof")
	}
	if err := h.ValidateProof(&HeliosProof{}, []byte("h"), nil); err == nil {
		t.Fatal("expected an error for a nil target")
	}
}

// TestMemoryPhaseDoesNotMutateTheHeader guards the append-aliasing defect:
// `append(blockHeader, ...)` writes into the caller's backing array whenever it
// has spare capacity, corrupting the header between mining iterations.
func TestMemoryPhaseDoesNotMutateTheHeader(t *testing.T) {
	h := NewHeliosAlgorithm(fastConfig())

	// A slice with spare capacity is exactly the case that used to break.
	header := make([]byte, 8, 128)
	copy(header, "hdr-data")
	original := string(header)

	if _, err := h.executeMemoryPhase(header, 1); err != nil {
		t.Fatalf("memory phase: %v", err)
	}
	if string(header) != original {
		t.Fatalf("memory phase mutated the caller's header: %q -> %q", original, string(header))
	}

	if _, err := h.Mine(header, easyTarget()); err != nil {
		t.Fatalf("mine: %v", err)
	}
	if string(header) != original {
		t.Fatalf("Mine mutated the caller's header: %q -> %q", original, string(header))
	}
}

// TestMemoryPhaseHandlesUnalignedSizes guards the slice bounds in the mixing
// loop, which read past the end when the buffer was not a multiple of 32.
func TestMemoryPhaseHandlesUnalignedSizes(t *testing.T) {
	for _, size := range []int{1, 31, 32, 33, 63, 64, 65, 1000} {
		cfg := fastConfig()
		cfg.MemoryBaseSize = size

		h := NewHeliosAlgorithm(cfg)
		if _, err := h.executeMemoryPhase([]byte("header"), 1); err != nil {
			t.Fatalf("memory size %d: %v", size, err)
		}
	}
}

// TestMiningTimeoutIsReported guards the silent acceptance of unmined blocks:
// Mine's timeout must surface as an error, not as a block the caller stores.
func TestMiningTimeoutIsReported(t *testing.T) {
	cfg := fastConfig()
	cfg.MiningTimeout = 50 * time.Millisecond

	h := NewHeliosAlgorithm(cfg)

	// A target of 1 is effectively unreachable.
	proof, err := h.Mine([]byte("timeout-test"), big.NewInt(1))
	if err == nil {
		t.Fatal("expected a timeout error")
	}
	if proof != nil {
		t.Fatal("no proof may be returned when mining fails")
	}
}

// TestMineRejectsBadWeights covers the configuration guard.
func TestMineRejectsBadWeights(t *testing.T) {
	cfg := fastConfig()
	cfg.MemoryWeight = 50 // total no longer 100

	h := NewHeliosAlgorithm(cfg)
	if _, err := h.Mine([]byte("x"), easyTarget()); err == nil {
		t.Fatal("expected an error when the stage weights do not sum to 100")
	}
}

// TestCryptoPhaseRejectsBadKeySize covers a configuration that would otherwise
// slice out of range.
func TestCryptoPhaseRejectsBadKeySize(t *testing.T) {
	cfg := fastConfig()
	cfg.CryptoKeySize = 0

	h := NewHeliosAlgorithm(cfg)
	if _, err := h.executeCryptographicPhase([]byte("stage2")); err == nil {
		t.Fatal("expected an error for a zero key size")
	}

	// An oversized key size must not panic.
	cfg.CryptoKeySize = 4096
	h = NewHeliosAlgorithm(cfg)
	if _, err := h.executeCryptographicPhase([]byte("stage2")); err == nil {
		t.Fatal("expected an error for an invalid AES key size")
	}
}

// TestTimeLockPhaseDoesNotSleep guards the replacement of wall-clock sleeping
// with actual sequential work. Sleeping is free to skip and free to parallelise,
// so it is not proof of anything.
func TestTimeLockPhaseDoesNotSleep(t *testing.T) {
	cfg := fastConfig()
	cfg.TimeLockIterations = 1000
	cfg.TimeLockBaseDuration = 10 * time.Second // would have dominated the runtime

	h := NewHeliosAlgorithm(cfg)

	start := time.Now()
	if _, _, err := h.executeTimeLockPhase([]byte("stage1")); err != nil {
		t.Fatalf("time-lock phase: %v", err)
	}
	elapsed := time.Since(start)

	// 1000 SHA-256 rounds take microseconds; the old implementation slept for
	// TimeLockBaseDuration in total.
	if elapsed > time.Second {
		t.Fatalf("time-lock phase appears to be sleeping: took %s", elapsed)
	}
}

// hashHex mirrors the final-hash computation for test fixtures.
func hashHex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
