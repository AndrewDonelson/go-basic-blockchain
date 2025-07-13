package algorithm

import (
	"math/big"
	"testing"
	"time"
)

func TestHeliosAlgorithm(t *testing.T) {
	// Create Helios algorithm with test config for fast mining
	config := TestHeliosConfig()
	helios := NewHeliosAlgorithm(config)

	// Test block header
	blockHeader := []byte("test block header for mining")

	// Set a very easy target difficulty for testing (accepts any hash)
	targetDifficulty := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)) // 2^256-1

	t.Run("Test Mining", func(t *testing.T) {
		// Start mining
		proof, err := helios.Mine(blockHeader, targetDifficulty)
		if err != nil {
			t.Fatalf("Mining failed: %v", err)
		}

		// Verify proof structure
		if proof == nil {
			t.Fatal("Proof is nil")
		}

		if proof.Nonce == 0 && len(proof.FinalHash) == 0 {
			t.Fatal("Proof appears to be empty")
		}

		// Verify all stages have results
		if len(proof.Stage1Result) == 0 {
			t.Error("Stage 1 result is empty")
		}
		if len(proof.Stage2Result) == 0 {
			t.Error("Stage 2 result is empty")
		}
		if len(proof.Stage3Result) == 0 {
			t.Error("Stage 3 result is empty")
		}

		// Verify final hash is not empty
		if proof.FinalHash == "" {
			t.Error("Final hash is empty")
		}

		// Verify timestamp is reasonable
		if proof.Timestamp.IsZero() {
			t.Error("Timestamp is zero")
		}

		if proof.Timestamp.After(time.Now().Add(5 * time.Minute)) {
			t.Error("Timestamp is in the future")
		}

		t.Logf("Mining successful: nonce=%d, hash=%s", proof.Nonce, proof.FinalHash)
	})

	t.Run("Test Proof Validation", func(t *testing.T) {
		// Mine a proof first
		proof, err := helios.Mine(blockHeader, targetDifficulty)
		if err != nil {
			t.Fatalf("Mining failed: %v", err)
		}

		// Validate the proof
		err = helios.ValidateProof(proof, blockHeader, targetDifficulty)
		if err != nil {
			t.Fatalf("Proof validation failed: %v", err)
		}

		t.Logf("Proof validation successful")
	})

	t.Run("Test Configuration Validation", func(t *testing.T) {
		// Test invalid configuration (weights don't sum to 100)
		invalidConfig := &HeliosConfig{
			MemoryWeight:   50,
			TimeLockWeight: 30,
			CryptoWeight:   30, // Total = 110, should fail
		}

		invalidHelios := NewHeliosAlgorithm(invalidConfig)
		_, err := invalidHelios.Mine(blockHeader, targetDifficulty)
		if err == nil {
			t.Error("Expected error for invalid configuration, got none")
		}

		t.Logf("Configuration validation working: %v", err)
	})
}

func TestHeliosStages(t *testing.T) {
	config := TestHeliosConfig()
	helios := NewHeliosAlgorithm(config)

	blockHeader := []byte("test header")
	nonce := uint64(12345)

	t.Run("Test Memory Phase", func(t *testing.T) {
		result, err := helios.executeMemoryPhase(blockHeader, nonce)
		if err != nil {
			t.Fatalf("Memory phase failed: %v", err)
		}

		if len(result) != 32 {
			t.Errorf("Expected 32-byte result, got %d bytes", len(result))
		}

		t.Logf("Memory phase successful: %x", result)
	})

	t.Run("Test Time-Lock Phase", func(t *testing.T) {
		// First get memory phase result
		memoryResult, err := helios.executeMemoryPhase(blockHeader, nonce)
		if err != nil {
			t.Fatalf("Memory phase failed: %v", err)
		}

		// Then execute time-lock phase
		result, err := helios.executeTimeLockPhase(memoryResult)
		if err != nil {
			t.Fatalf("Time-lock phase failed: %v", err)
		}

		if len(result) != 32 {
			t.Errorf("Expected 32-byte result, got %d bytes", len(result))
		}

		t.Logf("Time-lock phase successful: %x", result)
	})

	t.Run("Test Cryptographic Phase", func(t *testing.T) {
		// First get memory phase result
		memoryResult, err := helios.executeMemoryPhase(blockHeader, nonce)
		if err != nil {
			t.Fatalf("Memory phase failed: %v", err)
		}

		// Then get time-lock phase result
		timelockResult, err := helios.executeTimeLockPhase(memoryResult)
		if err != nil {
			t.Fatalf("Time-lock phase failed: %v", err)
		}

		// Finally execute cryptographic phase
		result, err := helios.executeCryptographicPhase(timelockResult)
		if err != nil {
			t.Fatalf("Cryptographic phase failed: %v", err)
		}

		if len(result) != 32 {
			t.Errorf("Expected 32-byte result, got %d bytes", len(result))
		}

		t.Logf("Cryptographic phase successful: %x", result)
	})
}

func TestHeliosPerformance(t *testing.T) {
	config := TestHeliosConfig()
	helios := NewHeliosAlgorithm(config)

	blockHeader := []byte("performance test header")
	targetDifficulty := new(big.Int).Sub(new(big.Int).Lsh(big.NewInt(1), 256), big.NewInt(1)) // 2^256-1

	t.Run("Test Mining Performance", func(t *testing.T) {
		startTime := time.Now()

		proof, err := helios.Mine(blockHeader, targetDifficulty)
		if err != nil {
			t.Fatalf("Mining failed: %v", err)
		}

		duration := time.Since(startTime)
		t.Logf("Mining completed in %v", duration)

		// Verify the proof meets the target difficulty
		err = helios.ValidateProof(proof, blockHeader, targetDifficulty)
		if err != nil {
			t.Fatalf("Proof validation failed: %v", err)
		}

		t.Logf("Performance test successful: nonce=%d, energy=%d", proof.Nonce, proof.EnergyUsed)
	})
}
