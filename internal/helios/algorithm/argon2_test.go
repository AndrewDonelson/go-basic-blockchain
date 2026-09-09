package algorithm

import (
	"bytes"
	"runtime"
	"testing"

	"golang.org/x/crypto/argon2"
)

// Stage 1 is Argon2id (RFC 9106). What it replaced was a hand-rolled buffer fill
// that was not memory-hard at all: the fill was a sequential SHA-256 chain and
// the mix only touched adjacent blocks, so the whole thing could be reproduced by
// streaming, in 64 bytes, whatever size buffer it nominally allocated.

func argonTestAlgorithm() *HeliosAlgorithm {
	return NewHeliosAlgorithm(TestHeliosConfig())
}

// TestMemoryPhaseIsArgon2id checks the stage really is the standard function,
// against a direct call rather than against itself.
func TestMemoryPhaseIsArgon2id(t *testing.T) {
	cfg := TestHeliosConfig()
	h := NewHeliosAlgorithm(cfg)

	header := []byte("argon2 identity check")
	const nonce = 7

	got, err := h.executeMemoryPhase(header, nonce)
	if err != nil {
		t.Fatalf("memory phase: %v", err)
	}

	want := argon2.IDKey(
		memoryPhaseInput("helios/stage1/argon2/password", header, nonce),
		memoryPhaseInput("helios/stage1/argon2/salt", header, nonce),
		cfg.Argon2TimeCost, cfg.Argon2MemoryKiB, cfg.Argon2Parallelism, cfg.Argon2KeyLength,
	)

	if !bytes.Equal(got, want) {
		t.Fatal("the memory phase is not Argon2id over the documented inputs")
	}
	if len(got) != int(cfg.Argon2KeyLength) {
		t.Fatalf("output is %d bytes, want %d", len(got), cfg.Argon2KeyLength)
	}
}

// TestMemoryPhaseActuallyAllocatesMemory is the property the old phase lacked.
//
// The previous implementation could be reproduced with 64 bytes of state
// regardless of the buffer it claimed to need, so it gave an attacker with custom
// hardware exactly the advantage the stage existed to remove. Argon2id's data
// dependencies are designed so that computing it with less memory costs
// disproportionately more time.
func TestMemoryPhaseActuallyAllocatesMemory(t *testing.T) {
	cfg := TestHeliosConfig()
	cfg.Argon2MemoryKiB = 32 * 1024 // 32 MiB, large enough to see
	h := NewHeliosAlgorithm(cfg)

	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)

	if _, err := h.executeMemoryPhase([]byte("allocation"), 1); err != nil {
		t.Fatalf("memory phase: %v", err)
	}

	runtime.ReadMemStats(&after)
	allocated := after.TotalAlloc - before.TotalAlloc

	expected := uint64(cfg.Argon2MemoryKiB) * 1024
	t.Logf("configured %d KiB, allocated %d bytes during the phase",
		cfg.Argon2MemoryKiB, allocated)

	// Allow generous slack for the runtime, but a streaming implementation would
	// come in orders of magnitude under this.
	if allocated < expected/2 {
		t.Fatalf("the phase allocated %d bytes for a %d KiB parameter; it is not "+
			"using the memory it claims to need", allocated, cfg.Argon2MemoryKiB)
	}
}

// TestMemoryPhaseIsDeterministic: a verifier recomputes this, so it must be
// reproducible. A random salt here would make the stage unverifiable, which is
// the defect the Helios rewrite began from.
func TestMemoryPhaseIsDeterministic(t *testing.T) {
	h := argonTestAlgorithm()
	header := []byte("determinism")

	first, err := h.executeMemoryPhase(header, 3)
	if err != nil {
		t.Fatalf("memory phase: %v", err)
	}
	for i := 0; i < 3; i++ {
		again, err := h.executeMemoryPhase(header, 3)
		if err != nil {
			t.Fatalf("memory phase: %v", err)
		}
		if !bytes.Equal(first, again) {
			t.Fatal("the memory phase is not deterministic; a verifier could not " +
				"reproduce it")
		}
	}
}

// TestMemoryPhaseSeparatesCandidates: every nonce must get its own work, or the
// search could be amortised across attempts.
func TestMemoryPhaseSeparatesCandidates(t *testing.T) {
	h := argonTestAlgorithm()
	header := []byte("separation")

	seen := map[string]bool{}
	for nonce := uint64(0); nonce < 8; nonce++ {
		out, err := h.executeMemoryPhase(header, nonce)
		if err != nil {
			t.Fatalf("memory phase: %v", err)
		}
		if seen[string(out)] {
			t.Fatalf("nonce %d reproduced an earlier result", nonce)
		}
		seen[string(out)] = true
	}

	// And a different header gives different work for the same nonce.
	a, _ := h.executeMemoryPhase([]byte("header A"), 1)
	b, _ := h.executeMemoryPhase([]byte("header B"), 1)
	if bytes.Equal(a, b) {
		t.Fatal("two different headers produced the same stage-1 result")
	}
}

// TestPasswordAndSaltAreDistinct. Argon2 takes both; deriving them from the same
// bytes would waste the domain separation and hand the KDF a password equal to
// its salt.
func TestPasswordAndSaltAreDistinct(t *testing.T) {
	header := []byte("domain separation")
	password := memoryPhaseInput("helios/stage1/argon2/password", header, 5)
	salt := memoryPhaseInput("helios/stage1/argon2/salt", header, 5)

	if bytes.Equal(password, salt) {
		t.Fatal("the password and salt are identical")
	}
}

// TestMemoryPhaseInputIsUnambiguous.
//
// The old code appended fmt.Sprintf("%d", nonce) to the header, so a header
// ending in digits and a nonce could collide with a different header and a
// different nonce -- two distinct candidates sharing stage-1 work.
func TestMemoryPhaseInputIsUnambiguous(t *testing.T) {
	// "abc" + 12  vs  "abc1" + 2  concatenate identically in the old scheme.
	first := memoryPhaseInput("d", []byte("abc"), 12)
	second := memoryPhaseInput("d", []byte("abc1"), 2)

	if bytes.Equal(first, second) {
		t.Fatal("two different (header, nonce) pairs produced the same input")
	}
}

// TestArgon2ParametersAreValidated: a misconfiguration presents as blocks being
// mysteriously refused by the network, so it has to fail loudly here.
func TestArgon2ParametersAreValidated(t *testing.T) {
	t.Run("memory below the RFC minimum", func(t *testing.T) {
		cfg := TestHeliosConfig()
		cfg.Argon2Parallelism = 4
		cfg.Argon2MemoryKiB = 8 // needs 8*4 = 32
		h := NewHeliosAlgorithm(cfg)

		if _, err := h.executeMemoryPhase([]byte("x"), 1); err == nil {
			t.Fatal("memory below 8*parallelism was accepted")
		}
	})

	t.Run("key length too short", func(t *testing.T) {
		cfg := TestHeliosConfig()
		cfg.Argon2KeyLength = 4
		h := NewHeliosAlgorithm(cfg)

		if _, err := h.executeMemoryPhase([]byte("x"), 1); err == nil {
			t.Fatal("a 4-byte output length was accepted")
		}
	})

	t.Run("zero values fall back to the defaults", func(t *testing.T) {
		cfg := TestHeliosConfig()
		cfg.Argon2MemoryKiB = 0
		cfg.Argon2TimeCost = 0
		cfg.Argon2Parallelism = 0
		cfg.Argon2KeyLength = 0
		h := NewHeliosAlgorithm(cfg)

		memory, time, parallelism, keyLen, err := h.argon2Parameters()
		if err != nil {
			t.Fatalf("defaults did not resolve: %v", err)
		}
		if memory != defaultArgon2MemoryKiB || time != defaultArgon2TimeCost ||
			parallelism != defaultArgon2Parallelism || keyLen != defaultArgon2KeyLength {
			t.Fatalf("unexpected defaults: %d %d %d %d", memory, time, parallelism, keyLen)
		}
	})
}

// TestParametersAreConsensusCritical. Every one changes the digest, so two nodes
// configured differently reject each other's blocks -- which is why they are
// validated rather than treated as tuning.
func TestParametersAreConsensusCritical(t *testing.T) {
	header := []byte("consensus parameters")

	baseline := TestHeliosConfig()
	base, err := NewHeliosAlgorithm(baseline).executeMemoryPhase(header, 1)
	if err != nil {
		t.Fatalf("memory phase: %v", err)
	}

	variants := map[string]func(*HeliosConfig){
		"memory":      func(c *HeliosConfig) { c.Argon2MemoryKiB *= 2 },
		"time cost":   func(c *HeliosConfig) { c.Argon2TimeCost++ },
		"parallelism": func(c *HeliosConfig) { c.Argon2Parallelism = 2 },
		"key length":  func(c *HeliosConfig) { c.Argon2KeyLength = 64 },
	}

	for name, mutate := range variants {
		cfg := TestHeliosConfig()
		mutate(cfg)

		other, err := NewHeliosAlgorithm(cfg).executeMemoryPhase(header, 1)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if bytes.Equal(base, other) {
			t.Fatalf("changing the %s did not change the result; it would not be "+
				"consensus-critical and nodes could silently disagree", name)
		}
	}
}

// TestMinedProofValidatesWithArgon2 closes the loop through Helios.
func TestMinedProofValidatesWithArgon2(t *testing.T) {
	h := argonTestAlgorithm()
	header := []byte("end to end")

	proof, err := h.Mine(header, easyTarget())
	if err != nil {
		t.Fatalf("mine: %v", err)
	}
	if err := h.ValidateProof(proof, header, easyTarget()); err != nil {
		t.Fatalf("a mined proof did not validate: %v", err)
	}

	// A fabricated stage-1 result must be refused: verification recomputes it.
	forged := *proof
	forged.Stage1Result = append([]byte{}, proof.Stage1Result...)
	forged.Stage1Result[0] ^= 0xff
	if err := h.ValidateProof(&forged, header, easyTarget()); err == nil {
		t.Fatal("a fabricated stage-1 result validated")
	}
}
