package algorithm

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"
)

// HeliosProof represents a complete proof of work for the Helios algorithm
type HeliosProof struct {
	Nonce        uint64    `json:"nonce"`
	Timestamp    time.Time `json:"timestamp"`
	Stage1Result []byte    `json:"stage1_result"` // Memory phase result
	Stage2Result []byte    `json:"stage2_result"` // Time-lock phase result
	Stage3Result []byte    `json:"stage3_result"` // Cryptographic phase result
	FinalHash    string    `json:"final_hash"`
	Difficulty   *big.Int  `json:"difficulty"`
	EnergyUsed   int64     `json:"energy_used"` // CPU cycles used
}

// HeliosConfig holds configuration for the Helios algorithm
type HeliosConfig struct {
	// Stage weights (must sum to 100)
	MemoryWeight   int `json:"memory_weight"`   // 40%
	TimeLockWeight int `json:"timelock_weight"` // 30%
	CryptoWeight   int `json:"crypto_weight"`   // 30%

	// Memory phase parameters
	MemoryBaseSize    int     `json:"memory_base_size"`    // 64MB
	MemoryScaleFactor float64 `json:"memory_scale_factor"` // 1.0
	MemoryIterations  int     `json:"memory_iterations"`   // 3

	// Time-lock parameters
	TimeLockBaseDuration time.Duration `json:"timelock_base_duration"` // 50ms
	TimeLockScaleFactor  float64       `json:"timelock_scale_factor"`  // 1.0
	TimeLockIterations   int           `json:"timelock_iterations"`    // 1000

	// Cryptographic parameters
	CryptoKeySize    int `json:"crypto_key_size"`   // 32 bytes
	CryptoBlockSize  int `json:"crypto_block_size"` // 16 bytes
	CryptoIterations int `json:"crypto_iterations"` // 10000

	// Energy tracking
	EnableEnergyTracking bool `json:"enable_energy_tracking"` // false initially
}

// DefaultHeliosConfig returns the default configuration for Helios
// Optimized for 20-second block time
func DefaultHeliosConfig() *HeliosConfig {
	return &HeliosConfig{
		MemoryWeight:         40,
		TimeLockWeight:       30,
		CryptoWeight:         30,
		MemoryBaseSize:       1 * 1024 * 1024, // 1MB (reduced from 64MB)
		MemoryScaleFactor:    1.0,
		MemoryIterations:     1,                    // Reduced from 3
		TimeLockBaseDuration: 2 * time.Millisecond, // Reduced from 50ms
		TimeLockScaleFactor:  1.0,
		TimeLockIterations:   50, // Reduced from 1000
		CryptoKeySize:        32,
		CryptoBlockSize:      16,
		CryptoIterations:     100, // Reduced from 10000
		EnableEnergyTracking: false,
	}
}

// TestHeliosConfig returns a configuration for fast mining in tests
func TestHeliosConfig() *HeliosConfig {
	return &HeliosConfig{
		MemoryWeight:         40,
		TimeLockWeight:       30,
		CryptoWeight:         30,
		MemoryBaseSize:       128 * 1024, // 128KB (much smaller)
		MemoryScaleFactor:    1.0,
		MemoryIterations:     1,
		TimeLockBaseDuration: 1 * time.Millisecond,
		TimeLockScaleFactor:  1.0,
		TimeLockIterations:   10,
		CryptoKeySize:        32,
		CryptoBlockSize:      16,
		CryptoIterations:     10,
		EnableEnergyTracking: false,
	}
}

// HeliosAlgorithm implements the three-stage Helios proof of work algorithm
type HeliosAlgorithm struct {
	config *HeliosConfig
}

// NewHeliosAlgorithm creates a new Helios algorithm instance
func NewHeliosAlgorithm(config *HeliosConfig) *HeliosAlgorithm {
	if config == nil {
		config = DefaultHeliosConfig()
	}
	return &HeliosAlgorithm{config: config}
}

// Mine attempts to find a valid proof of work using the Helios algorithm
func (h *HeliosAlgorithm) Mine(blockHeader []byte, targetDifficulty *big.Int) (*HeliosProof, error) {
	startTime := time.Now()
	var energyUsed int64

	// Validate weights sum to 100
	if h.config.MemoryWeight+h.config.TimeLockWeight+h.config.CryptoWeight != 100 {
		return nil, fmt.Errorf("stage weights must sum to 100, got %d",
			h.config.MemoryWeight+h.config.TimeLockWeight+h.config.CryptoWeight)
	}

	nonce := uint64(0)
	for {
		// Create proof attempt
		proof := &HeliosProof{
			Nonce:      nonce,
			Timestamp:  time.Now(),
			Difficulty: targetDifficulty,
		}

		// Stage 1: Memory Phase (40% weight)
		stage1Start := time.Now()
		stage1Result, err := h.executeMemoryPhase(blockHeader, nonce)
		if err != nil {
			return nil, fmt.Errorf("memory phase failed: %w", err)
		}
		proof.Stage1Result = stage1Result
		energyUsed += time.Since(stage1Start).Nanoseconds()

		// Stage 2: Time-Lock Phase (30% weight)
		stage2Start := time.Now()
		stage2Result, err := h.executeTimeLockPhase(stage1Result)
		if err != nil {
			return nil, fmt.Errorf("time-lock phase failed: %w", err)
		}
		proof.Stage2Result = stage2Result
		energyUsed += time.Since(stage2Start).Nanoseconds()

		// Stage 3: Cryptographic Phase (30% weight)
		stage3Start := time.Now()
		stage3Result, err := h.executeCryptographicPhase(stage2Result)
		if err != nil {
			return nil, fmt.Errorf("cryptographic phase failed: %w", err)
		}
		proof.Stage3Result = stage3Result
		energyUsed += time.Since(stage3Start).Nanoseconds()

		// Combine all stage results for final hash
		finalInput := append(blockHeader, []byte(fmt.Sprintf("%d", nonce))...)
		finalInput = append(finalInput, proof.Stage1Result...)
		finalInput = append(finalInput, proof.Stage2Result...)
		finalInput = append(finalInput, proof.Stage3Result...)

		finalHash := sha256.Sum256(finalInput)
		proof.FinalHash = hex.EncodeToString(finalHash[:])
		proof.EnergyUsed = energyUsed

		// Check if proof meets target difficulty
		hashInt := new(big.Int).SetBytes(finalHash[:])
		if hashInt.Cmp(targetDifficulty) <= 0 {
			return proof, nil
		}

		nonce++

		// Optional: Add timeout to prevent infinite mining
		if time.Since(startTime) > 4*time.Second {
			return nil, fmt.Errorf("mining timeout reached")
		}
	}
}

// executeMemoryPhase implements Stage 1: Memory Phase (40% weight)
// Uses Argon2-inspired memory-hard function
func (h *HeliosAlgorithm) executeMemoryPhase(blockHeader []byte, nonce uint64) ([]byte, error) {
	// Calculate memory size based on difficulty
	memorySize := h.config.MemoryBaseSize
	if h.config.MemoryScaleFactor != 1.0 {
		memorySize = int(float64(memorySize) * h.config.MemoryScaleFactor)
	}

	// Create memory buffer
	memory := make([]byte, memorySize)

	// Initialize memory with block header and nonce
	seed := append(blockHeader, []byte(fmt.Sprintf("%d", nonce))...)
	hash := sha256.Sum256(seed)
	copy(memory[:32], hash[:])

	// Memory-hard computation (simplified Argon2-inspired)
	for i := 0; i < h.config.MemoryIterations; i++ {
		// Fill memory with pseudo-random data
		for j := 32; j < len(memory); j += 32 {
			chunk := memory[max(0, j-32):j]
			hash := sha256.Sum256(chunk)
			copy(memory[j:min(j+32, len(memory))], hash[:])
		}

		// Mix memory blocks
		for j := 0; j < len(memory)-32; j += 32 {
			block1 := memory[j : j+32]
			block2 := memory[(j+32)%len(memory) : (j+32)%len(memory)+32]

			// XOR blocks
			for k := 0; k < 32; k++ {
				block1[k] ^= block2[k]
			}
		}
	}

	// Return final memory state hash
	result := sha256.Sum256(memory)
	return result[:], nil
}

// executeTimeLockPhase implements Stage 2: Time-Lock Phase (30% weight)
// Uses VDF-inspired sequential computation
func (h *HeliosAlgorithm) executeTimeLockPhase(stage1Result []byte) ([]byte, error) {
	// Calculate time-lock duration based on difficulty
	duration := h.config.TimeLockBaseDuration
	if h.config.TimeLockScaleFactor != 1.0 {
		duration = time.Duration(float64(duration) * h.config.TimeLockScaleFactor)
	}

	// Sequential computation that cannot be parallelized
	result := stage1Result
	for i := 0; i < h.config.TimeLockIterations; i++ {
		// Sequential hash chain
		hash := sha256.Sum256(result)
		result = hash[:]

		// Add small delay to ensure sequential nature
		time.Sleep(duration / time.Duration(h.config.TimeLockIterations))
	}

	return result, nil
}

// executeCryptographicPhase implements Stage 3: Cryptographic Phase (30% weight)
// Uses AES-NI optimized encryption
func (h *HeliosAlgorithm) executeCryptographicPhase(stage2Result []byte) ([]byte, error) {
	// Generate random key and nonce
	key := make([]byte, h.config.CryptoKeySize)
	nonce := make([]byte, 12) // GCM requires 12-byte nonce

	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("failed to generate key: %w", err)
	}
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("failed to create AES cipher: %w", err)
	}

	// Create GCM mode for authenticated encryption
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM mode: %w", err)
	}

	// Encrypt stage2 result multiple times
	result := stage2Result
	for i := 0; i < h.config.CryptoIterations; i++ {
		// Encrypt with GCM
		encrypted := gcm.Seal(nil, nonce, result, nil)

		// Use encrypted result as input for next iteration
		result = encrypted[:h.config.CryptoKeySize] // Truncate to key size
	}

	return result, nil
}

// ValidateProof validates a Helios proof
func (h *HeliosAlgorithm) ValidateProof(proof *HeliosProof, blockHeader []byte, targetDifficulty *big.Int) error {
	// Reconstruct final hash
	finalInput := append(blockHeader, []byte(fmt.Sprintf("%d", proof.Nonce))...)
	finalInput = append(finalInput, proof.Stage1Result...)
	finalInput = append(finalInput, proof.Stage2Result...)
	finalInput = append(finalInput, proof.Stage3Result...)

	finalHash := sha256.Sum256(finalInput)
	reconstructedHash := hex.EncodeToString(finalHash[:])

	if reconstructedHash != proof.FinalHash {
		return fmt.Errorf("proof hash mismatch: expected %s, got %s",
			proof.FinalHash, reconstructedHash)
	}

	// Check difficulty
	hashInt := new(big.Int).SetBytes(finalHash[:])
	if hashInt.Cmp(targetDifficulty) > 0 {
		return fmt.Errorf("proof does not meet target difficulty")
	}

	return nil
}

// Helper functions
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
