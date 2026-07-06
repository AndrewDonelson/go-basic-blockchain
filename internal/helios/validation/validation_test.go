package validation

import (
	"math/big"
	"testing"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/helios/algorithm"
)

func validProof() *algorithm.HeliosProof {
	return &algorithm.HeliosProof{
		Nonce:        1,
		Timestamp:    time.Now(),
		Stage1Result: []byte{1},
		Stage2Result: []byte{2},
		Stage3Result: []byte{3},
		FinalHash:    "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
		Difficulty:   big.NewInt(10),
		EnergyUsed:   1000,
	}
}

func TestNewProofValidator(t *testing.T) {
	pv := NewProofValidator(nil)
	if pv == nil || pv.algorithm == nil {
		t.Fatal("expected validator with algorithm")
	}
}

func TestValidateProof(t *testing.T) {
	pv := NewProofValidator(nil)
	proof := validProof()

	if err := pv.ValidateProof(proof, big.NewInt(10)); err != nil {
		t.Fatalf("expected valid proof, got err: %v", err)
	}

	if err := pv.ValidateProof(nil, big.NewInt(1)); err == nil {
		t.Fatal("expected nil proof error")
	}
	if err := pv.ValidateProof(proof, nil); err == nil {
		t.Fatal("expected nil target error")
	}

	bad := *proof
	bad.Stage1Result = nil
	if err := pv.ValidateProof(&bad, big.NewInt(10)); err == nil {
		t.Fatal("expected stage1 error")
	}

	bad = *proof
	bad.Stage2Result = nil
	if err := pv.ValidateProof(&bad, big.NewInt(10)); err == nil {
		t.Fatal("expected stage2 error")
	}

	bad = *proof
	bad.Stage3Result = nil
	if err := pv.ValidateProof(&bad, big.NewInt(10)); err == nil {
		t.Fatal("expected stage3 error")
	}

	bad = *proof
	bad.FinalHash = ""
	if err := pv.ValidateProof(&bad, big.NewInt(10)); err == nil {
		t.Fatal("expected final hash error")
	}

	bad = *proof
	bad.Difficulty = nil
	if err := pv.ValidateProof(&bad, big.NewInt(10)); err == nil {
		t.Fatal("expected nil difficulty error")
	}

	bad = *proof
	bad.Difficulty = big.NewInt(11)
	if err := pv.ValidateProof(&bad, big.NewInt(10)); err == nil {
		t.Fatal("expected difficulty mismatch error")
	}
}

func TestValidateProofHash(t *testing.T) {
	pv := NewProofValidator(nil)
	proof := validProof()
	if err := pv.ValidateProofHash(proof); err != nil {
		t.Fatalf("expected valid hash, got err: %v", err)
	}

	if err := pv.ValidateProofHash(nil); err == nil {
		t.Fatal("expected nil proof error")
	}

	bad := *proof
	bad.FinalHash = ""
	if err := pv.ValidateProofHash(&bad); err == nil {
		t.Fatal("expected empty hash error")
	}

	bad = *proof
	bad.FinalHash = "abcd"
	if err := pv.ValidateProofHash(&bad); err == nil {
		t.Fatal("expected invalid length error")
	}

	bad = *proof
	bad.FinalHash = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdeg"
	if err := pv.ValidateProofHash(&bad); err == nil {
		t.Fatal("expected non-hex char error")
	}
}

func TestValidateProofStagesAndSequence(t *testing.T) {
	pv := NewProofValidator(nil)
	proof := validProof()
	if err := pv.ValidateProofStages(proof); err != nil {
		t.Fatalf("expected valid stages, got err: %v", err)
	}
	if err := pv.VerifyProofSequence(proof); err != nil {
		t.Fatalf("expected valid sequence, got err: %v", err)
	}

	if err := pv.ValidateProofStages(nil); err == nil {
		t.Fatal("expected nil proof error")
	}
	if err := pv.VerifyProofSequence(nil); err == nil {
		t.Fatal("expected nil proof sequence error")
	}

	bad := *proof
	bad.Stage1Result = nil
	if err := pv.VerifyProofSequence(&bad); err == nil {
		t.Fatal("expected invalid stage sequence error")
	}

	bad = *proof
	bad.Nonce = ^uint64(0)
	if err := pv.VerifyProofSequence(&bad); err == nil {
		t.Fatal("expected unreasonable nonce error")
	}
}

func TestValidateFullProofAndStats(t *testing.T) {
	pv := NewProofValidator(nil)
	proof := validProof()
	if err := pv.ValidateFullProof(proof, big.NewInt(10)); err != nil {
		t.Fatalf("expected valid full proof, got err: %v", err)
	}

	bad := *proof
	bad.FinalHash = "bad"
	if err := pv.ValidateFullProof(&bad, big.NewInt(10)); err == nil {
		t.Fatal("expected full validation hash error")
	}

	stats := pv.GetProofStatistics(proof)
	if stats["final_hash"] != proof.FinalHash {
		t.Fatalf("unexpected stats final hash: %v", stats["final_hash"])
	}

	nilStats := pv.GetProofStatistics(nil)
	if nilStats["error"] == nil {
		t.Fatal("expected error entry for nil proof stats")
	}
}

func TestIsHexChar(t *testing.T) {
	valid := []rune{'0', '9', 'a', 'f', 'A', 'F'}
	for _, r := range valid {
		if !isHexChar(r) {
			t.Fatalf("expected %c to be hex", r)
		}
	}
	invalid := []rune{'g', 'G', '-', 'x'}
	for _, r := range invalid {
		if isHexChar(r) {
			t.Fatalf("expected %c to be non-hex", r)
		}
	}
}
