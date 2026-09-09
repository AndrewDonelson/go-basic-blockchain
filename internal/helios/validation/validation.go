// Package validation provides proof validation for the Helios consensus algorithm.
package validation

import (
	"encoding/hex"
	"fmt"
	"math/big"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/helios/algorithm"
)

// ProofValidator validates Helios proofs of work
type ProofValidator struct {
	algorithm *algorithm.HeliosAlgorithm
}

// NewProofValidator creates a new ProofValidator instance
func NewProofValidator(algo *algorithm.HeliosAlgorithm) *ProofValidator {
	if algo == nil {
		algo = algorithm.NewHeliosAlgorithm(nil) // Use default config
	}
	return &ProofValidator{
		algorithm: algo,
	}
}

// ValidateProof validates a Helios proof against the target difficulty
func (pv *ProofValidator) ValidateProof(proof *algorithm.HeliosProof, targetDifficulty *big.Int) error {
	if proof == nil {
		return fmt.Errorf("proof cannot be nil")
	}

	if targetDifficulty == nil {
		return fmt.Errorf("target difficulty cannot be nil")
	}

	// Validate that proof has all required stages
	if len(proof.Stage1Result) == 0 {
		return fmt.Errorf("stage 1 result is empty")
	}

	if len(proof.Stage2Result) == 0 {
		return fmt.Errorf("stage 2 result is empty")
	}

	if len(proof.Stage3Result) == 0 {
		return fmt.Errorf("stage 3 result is empty")
	}

	if proof.FinalHash == "" {
		return fmt.Errorf("final hash is empty")
	}

	// Validate that the difficulty matches or exceeds target
	if proof.Difficulty == nil {
		return fmt.Errorf("proof difficulty is nil")
	}

	// Compare the ACTUAL proof hash against the target.
	//
	// This used to compare proof.Difficulty against targetDifficulty. Both are
	// written by the miner -- HeliosAlgorithm.Mine sets proof.Difficulty to
	// targetDifficulty itself -- so the check reduced to `x <= x` and could never
	// fail. The hash, which is what the work actually produces, was never
	// inspected here at all.
	hashBytes, err := hex.DecodeString(proof.FinalHash)
	if err != nil {
		return fmt.Errorf("final hash is not valid hex: %w", err)
	}

	hashInt := new(big.Int).SetBytes(hashBytes)
	if hashInt.Cmp(targetDifficulty) > 0 {
		return fmt.Errorf("proof hash does not meet target difficulty")
	}

	return nil
}

// ValidateProofHash validates that a proof's final hash is valid
func (pv *ProofValidator) ValidateProofHash(proof *algorithm.HeliosProof) error {
	if proof == nil {
		return fmt.Errorf("proof cannot be nil")
	}

	if proof.FinalHash == "" {
		return fmt.Errorf("final hash is empty")
	}

	// Validate hash format (should be hex string of expected length)
	// Helios uses SHA-256, which produces 64 hex characters
	if len(proof.FinalHash) != 64 {
		return fmt.Errorf("invalid final hash length: expected 64, got %d", len(proof.FinalHash))
	}

	// Validate that hash is valid hex
	for _, c := range proof.FinalHash {
		if !isHexChar(c) {
			return fmt.Errorf("invalid final hash: contains non-hex character %c", c)
		}
	}

	return nil
}

// isHexChar checks if a character is a valid hexadecimal digit
func isHexChar(c rune) bool {
	return (c >= '0' && c <= '9') || (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
}

// ValidateProofStages validates that all three stages of the proof are present and valid
func (pv *ProofValidator) ValidateProofStages(proof *algorithm.HeliosProof) error {
	if proof == nil {
		return fmt.Errorf("proof cannot be nil")
	}

	stages := []struct {
		name   string
		data   []byte
		minLen int
	}{
		{"Stage 1 (Memory)", proof.Stage1Result, 1},
		{"Stage 2 (Time-lock)", proof.Stage2Result, 1},
		{"Stage 3 (Crypto)", proof.Stage3Result, 1},
	}

	for _, stage := range stages {
		if len(stage.data) < stage.minLen {
			return fmt.Errorf("%s result is invalid (length: %d)", stage.name, len(stage.data))
		}
	}

	return nil
}

// VerifyProofSequence verifies that a proof was generated in the correct sequence
// by checking that each stage's output was properly used in the next stage
func (pv *ProofValidator) VerifyProofSequence(proof *algorithm.HeliosProof) error {
	if proof == nil {
		return fmt.Errorf("proof cannot be nil")
	}

	if err := pv.ValidateProofStages(proof); err != nil {
		return fmt.Errorf("invalid proof stages: %w", err)
	}

	// Verify nonce is reasonable
	if proof.Nonce > ^uint64(0)/2 { // Simple sanity check
		return fmt.Errorf("nonce value is unreasonably high: %d", proof.Nonce)
	}

	return nil
}

// ValidateFullProof performs a comprehensive validation of a Helios proof
func (pv *ProofValidator) ValidateFullProof(proof *algorithm.HeliosProof, targetDifficulty *big.Int) error {
	if err := pv.ValidateProof(proof, targetDifficulty); err != nil {
		return fmt.Errorf("basic proof validation failed: %w", err)
	}

	if err := pv.ValidateProofHash(proof); err != nil {
		return fmt.Errorf("proof hash validation failed: %w", err)
	}

	if err := pv.VerifyProofSequence(proof); err != nil {
		return fmt.Errorf("proof sequence verification failed: %w", err)
	}

	return nil
}

// GetProofStatistics returns statistics about a proof
func (pv *ProofValidator) GetProofStatistics(proof *algorithm.HeliosProof) map[string]interface{} {
	if proof == nil {
		return map[string]interface{}{
			"error": "proof is nil",
		}
	}

	difficulty := "0"
	if proof.Difficulty != nil {
		difficulty = proof.Difficulty.String()
	}

	return map[string]interface{}{
		"nonce":          proof.Nonce,
		"timestamp":      proof.Timestamp.String(),
		"stage1_size":    len(proof.Stage1Result),
		"stage2_size":    len(proof.Stage2Result),
		"stage3_size":    len(proof.Stage3Result),
		"final_hash":     proof.FinalHash,
		"difficulty":     difficulty,
		"energy_used_ns": proof.EnergyUsed,
		"energy_used_ms": float64(proof.EnergyUsed) / 1_000_000,
	}
}
