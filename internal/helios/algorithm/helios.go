package algorithm

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"math/big"
	"sync"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/helios/vdf"
	"golang.org/x/crypto/argon2"
)

const (
	// gcmNonceSize is the AES-GCM nonce length in bytes.
	gcmNonceSize = 12
	// defaultMiningTimeout bounds a single Mine call.
	defaultMiningTimeout = 18 * time.Second
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

	// VDFProof is the encoded Wesolowski proof for stage 2. Stage2Result is the
	// VDF's output; this is the witness that lets a verifier accept that output
	// without repeating the sequential work behind it.
	VDFProof []byte `json:"vdf_proof,omitempty"`
}

// HeliosConfig holds configuration for the Helios algorithm
type HeliosConfig struct {
	// Stage weights (must sum to 100)
	MemoryWeight   int `json:"memory_weight"`   // 40%
	TimeLockWeight int `json:"timelock_weight"` // 30%
	CryptoWeight   int `json:"crypto_weight"`   // 30%

	// Memory phase parameters (Argon2id, RFC 9106).
	//
	// All four are consensus-critical: Argon2's output depends on every one of
	// them, so nodes configured differently compute different stage-1 results and
	// reject each other's blocks.
	Argon2MemoryKiB   uint32 `json:"argon2_memory_kib"`
	Argon2TimeCost    uint32 `json:"argon2_time_cost"`
	Argon2Parallelism uint8  `json:"argon2_parallelism"`
	Argon2KeyLength   uint32 `json:"argon2_key_length"`

	// Retained so existing configuration files still parse. They no longer
	// affect anything: the hand-rolled buffer they sized has been replaced.
	MemoryBaseSize    int     `json:"memory_base_size"`
	MemoryScaleFactor float64 `json:"memory_scale_factor"`
	MemoryIterations  int     `json:"memory_iterations"`

	// Time-lock parameters
	TimeLockBaseDuration time.Duration `json:"timelock_base_duration"` // 50ms
	TimeLockScaleFactor  float64       `json:"timelock_scale_factor"`  // 1.0
	TimeLockIterations   int           `json:"timelock_iterations"`    // 1000

	// VDF parameters. Stage 2 is a Wesolowski verifiable delay function over a
	// class group; see internal/helios/vdf.
	//
	// VDFDiscriminantSeed derives the group's discriminant deterministically, so
	// every node can re-derive the same parameters and confirm nobody chose them
	// with a trapdoor in hand. It is a public constant, not a secret.
	VDFDiscriminantSeed []byte `json:"vdf_discriminant_seed"`
	VDFDiscriminantBits int    `json:"vdf_discriminant_bits"`
	VDFIterations       uint64 `json:"vdf_iterations"`

	// Cryptographic parameters
	CryptoKeySize    int `json:"crypto_key_size"`   // 32 bytes
	CryptoBlockSize  int `json:"crypto_block_size"` // 16 bytes
	CryptoIterations int `json:"crypto_iterations"` // 10000

	// Energy tracking
	EnableEnergyTracking bool `json:"enable_energy_tracking"` // false initially

	// MiningTimeout bounds a single Mine call. Zero means defaultMiningTimeout.
	MiningTimeout time.Duration `json:"mining_timeout"`
}

// Argon2id defaults.
//
// 64 MiB with a time cost of 1 is the RFC 9106 second recommendation, and it is
// chosen here for the reason that matters to a proof of work rather than to a
// password hash: memory is what denies an attacker the GPU and ASIC advantage.
// A device can put thousands of hash cores on a die; it cannot put thousands of
// 64 MiB memories next to them.
//
// Parallelism is 1 deliberately. It is a consensus parameter, not a performance
// knob -- raising it changes the digest, so it cannot be tuned per machine, and 1
// is the value that makes a single attempt cost one core rather than several.
const (
	defaultArgon2MemoryKiB   = 64 * 1024 // 64 MiB
	defaultArgon2TimeCost    = 1
	defaultArgon2Parallelism = 1
	defaultArgon2KeyLength   = 32
)

// DefaultHeliosConfig returns the default configuration for Helios
// Optimized for 20-second block time
func DefaultHeliosConfig() *HeliosConfig {
	return &HeliosConfig{
		MemoryWeight:         40,
		TimeLockWeight:       30,
		CryptoWeight:         30,
		Argon2MemoryKiB:      defaultArgon2MemoryKiB,
		Argon2TimeCost:       defaultArgon2TimeCost,
		Argon2Parallelism:    defaultArgon2Parallelism,
		Argon2KeyLength:      defaultArgon2KeyLength,
		TimeLockBaseDuration: 2 * time.Millisecond, // Reduced from 50ms
		TimeLockScaleFactor:  1.0,
		TimeLockIterations:   50, // Reduced from 1000
		CryptoKeySize:        32,
		CryptoBlockSize:      16,
		CryptoIterations:     100, // Reduced from 10000
		EnableEnergyTracking: false,
		VDFDiscriminantSeed:  []byte(vdf.DefaultDiscriminantSeed),
		VDFDiscriminantBits:  vdf.DefaultDiscriminantBits,
		VDFIterations:        2000,
	}
}

// TestHeliosConfig returns a configuration for fast mining in tests
func TestHeliosConfig() *HeliosConfig {
	return &HeliosConfig{
		MemoryWeight:   40,
		TimeLockWeight: 30,
		CryptoWeight:   30,
		// Small but real Argon2id: the suite mines blocks, and 64 MiB per nonce
		// attempt would dominate its runtime. 8 MiB still exercises the actual
		// KDF rather than a stub.
		Argon2MemoryKiB:      8 * 1024,
		Argon2TimeCost:       1,
		Argon2Parallelism:    1,
		Argon2KeyLength:      32,
		TimeLockBaseDuration: 1 * time.Millisecond,
		TimeLockScaleFactor:  1.0,
		TimeLockIterations:   10,
		CryptoKeySize:        32,
		CryptoBlockSize:      16,
		CryptoIterations:     10,
		EnableEnergyTracking: false,
		// Small but real: the VDF still runs, so tests exercise the actual
		// evaluate-and-verify path rather than a stub. A 256-bit discriminant is
		// far below anything a chain should use -- it is here so the suite stays
		// fast, and the size is exactly what DefaultHeliosConfig does not skimp on.
		VDFDiscriminantSeed: []byte(vdf.DefaultDiscriminantSeed),
		VDFDiscriminantBits: 256,
		VDFIterations:       64,
	}
}

// vdfParameters resolves the group and delay length, filling in defaults.
func (h *HeliosAlgorithm) vdfParameters() (*big.Int, uint64, error) {
	seed := h.config.VDFDiscriminantSeed
	if len(seed) == 0 {
		seed = []byte(vdf.DefaultDiscriminantSeed)
	}
	bits := h.config.VDFDiscriminantBits
	if bits <= 0 {
		bits = vdf.DefaultDiscriminantBits
	}
	iterations := h.config.VDFIterations
	if iterations == 0 {
		iterations = 2000
	}

	h.vdfOnce.Do(func() {
		// The published group is embedded, because deriving a 2048-bit
		// discriminant means searching for a prime -- about half a second, far
		// too long to repeat at startup for a value that can never change.
		// Anything else is derived on demand, which is what the small test
		// parameters use.
		if bits == vdf.DefaultDiscriminantBits && string(seed) == vdf.DefaultDiscriminantSeed {
			h.vdfDiscriminant, h.vdfErr = vdf.DefaultDiscriminant()
			return
		}
		h.vdfDiscriminant, h.vdfErr = vdf.NewDiscriminant(seed, bits)
	})
	if h.vdfErr != nil {
		return nil, 0, fmt.Errorf("derive VDF discriminant: %w", h.vdfErr)
	}
	return h.vdfDiscriminant, iterations, nil
}

// HeliosAlgorithm implements the three-stage Helios proof of work algorithm
type HeliosAlgorithm struct {
	config *HeliosConfig

	// The discriminant is derived by searching for a prime, which is far too
	// expensive to redo per block. It depends only on the seed and size, so it is
	// computed once and shared.
	vdfOnce         sync.Once
	vdfDiscriminant *big.Int
	vdfErr          error
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
	return h.MineOnParent(blockHeader, nil, targetDifficulty)
}

// MineOnParent is Mine with the parent block's delay output, which chains this
// block's delay onto it. A nil parentOutput is the genesis case: there is nothing
// to chain onto.
func (h *HeliosAlgorithm) MineOnParent(blockHeader, parentOutput []byte, targetDifficulty *big.Int) (*HeliosProof, error) {
	startTime := time.Now()
	var energyUsed int64

	// Validate weights sum to 100
	if h.config.MemoryWeight+h.config.TimeLockWeight+h.config.CryptoWeight != 100 {
		return nil, fmt.Errorf("stage weights must sum to 100, got %d",
			h.config.MemoryWeight+h.config.TimeLockWeight+h.config.CryptoWeight)
	}

	// Stage 2 runs ONCE, before the search, because its input is the block header
	// and nothing else.
	//
	// Putting a delay function inside the nonce loop would be self-defeating: each
	// attempt would begin its own independent chain, so a miner with n cores runs
	// n chains concurrently and the block still costs one chain of wall-clock
	// time. Hoisting it out is what makes the delay a property of the block
	// rather than of an attempt -- and it also stops the search paying for T
	// sequential squarings per candidate.
	stage2Start := time.Now()
	stage2Result, vdfProof, err := h.executeTimeLockPhase(blockHeader, parentOutput)
	if err != nil {
		return nil, fmt.Errorf("time-lock phase failed: %w", err)
	}
	energyUsed += time.Since(stage2Start).Nanoseconds()

	// Stage 3 depends only on stage 2, so it is fixed for the block too.
	stage3Start := time.Now()
	stage3Result, err := h.executeCryptographicPhase(stage2Result)
	if err != nil {
		return nil, fmt.Errorf("cryptographic phase failed: %w", err)
	}
	energyUsed += time.Since(stage3Start).Nanoseconds()

	nonce := uint64(0)
	for {
		// Create proof attempt
		proof := &HeliosProof{
			Nonce:        nonce,
			Timestamp:    time.Now(),
			Difficulty:   targetDifficulty,
			Stage2Result: stage2Result,
			Stage3Result: stage3Result,
			VDFProof:     vdfProof,
		}

		// Stage 1: Memory Phase (40% weight)
		stage1Start := time.Now()
		stage1Result, err := h.executeMemoryPhase(blockHeader, nonce)
		if err != nil {
			return nil, fmt.Errorf("memory phase failed: %w", err)
		}
		proof.Stage1Result = stage1Result
		energyUsed += time.Since(stage1Start).Nanoseconds()

		// Combine all stage results for the final hash, in a fresh buffer so we do
		// not append into the caller's blockHeader backing array.
		finalHash := sha256.Sum256(heliosFinalPreimage(blockHeader, nonce,
			proof.Stage1Result, proof.Stage2Result, proof.Stage3Result))
		proof.FinalHash = hex.EncodeToString(finalHash[:])
		proof.EnergyUsed = energyUsed

		// Check if proof meets target difficulty
		hashInt := new(big.Int).SetBytes(finalHash[:])
		if hashInt.Cmp(targetDifficulty) <= 0 {
			return proof, nil
		}

		nonce++

		// Bound the search. The caller treats this as a hard failure and abandons
		// the block; it used to log and return the unmined block, which was then
		// appended to the chain and persisted anyway.
		if time.Since(startTime) > h.miningTimeout() {
			return nil, fmt.Errorf("mining timeout reached after %s", h.miningTimeout())
		}
	}
}

// executeMemoryPhase implements Stage 1: Memory Phase (40% weight)
// Uses Argon2-inspired memory-hard function
func (h *HeliosAlgorithm) executeMemoryPhase(blockHeader []byte, nonce uint64) ([]byte, error) {
	memoryKiB, timeCost, parallelism, keyLength, err := h.argon2Parameters()
	if err != nil {
		return nil, err
	}

	// The password commits to the header and the nonce; the salt is derived from
	// the same pair under a different domain string.
	//
	// The salt must be deterministic, because a verifier has to reproduce this
	// exactly -- a random salt would make the stage unverifiable, which is the
	// defect the whole Helios rewrite started from. It must also differ per
	// candidate, or every nonce would share one salt and the work could be
	// amortised across attempts.
	password := memoryPhaseInput("helios/stage1/argon2/password", blockHeader, nonce)
	salt := memoryPhaseInput("helios/stage1/argon2/salt", blockHeader, nonce)

	return argon2.IDKey(password, salt, timeCost, memoryKiB, parallelism, keyLength), nil
}

// memoryPhaseInput builds a domain-separated, unambiguous input.
//
// The nonce is encoded as eight fixed bytes rather than decimal text. The old
// code appended fmt.Sprintf("%d", nonce), so a header ending in digits and a
// nonce could collide with a different header and a different nonce -- two
// distinct candidates sharing a stage-1 input.
func memoryPhaseInput(domain string, blockHeader []byte, nonce uint64) []byte {
	h := sha256.New()
	h.Write([]byte(domain))

	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], uint64(len(blockHeader)))
	h.Write(buf[:])
	h.Write(blockHeader)

	binary.BigEndian.PutUint64(buf[:], nonce)
	h.Write(buf[:])

	return h.Sum(nil)
}

// argon2Parameters resolves the Argon2id cost parameters, filling in defaults.
//
// EVERY ONE OF THESE IS CONSENSUS-CRITICAL. Argon2's output depends on the memory
// size, the time cost, the degree of parallelism and the output length, so two
// nodes configured differently compute different stage-1 results and reject each
// other's blocks. They are validated here rather than trusted, because a
// misconfiguration presents as blocks being mysteriously refused by the network
// rather than as an error.
func (h *HeliosAlgorithm) argon2Parameters() (memoryKiB uint32, timeCost uint32, parallelism uint8, keyLength uint32, err error) {
	memoryKiB = h.config.Argon2MemoryKiB
	if memoryKiB == 0 {
		memoryKiB = defaultArgon2MemoryKiB
	}
	timeCost = h.config.Argon2TimeCost
	if timeCost == 0 {
		timeCost = defaultArgon2TimeCost
	}
	parallelism = h.config.Argon2Parallelism
	if parallelism == 0 {
		parallelism = defaultArgon2Parallelism
	}
	keyLength = h.config.Argon2KeyLength
	if keyLength == 0 {
		keyLength = defaultArgon2KeyLength
	}

	// RFC 9106: memory must be at least 8 * parallelism blocks of 1 KiB.
	if minimum := 8 * uint32(parallelism); memoryKiB < minimum {
		return 0, 0, 0, 0, fmt.Errorf(
			"argon2 memory of %d KiB is below the %d KiB minimum for parallelism %d",
			memoryKiB, minimum, parallelism)
	}
	if keyLength < 16 {
		return 0, 0, 0, 0, fmt.Errorf("argon2 key length %d is too short", keyLength)
	}

	return memoryKiB, timeCost, parallelism, keyLength, nil
}

// executeTimeLockPhase implements Stage 2: a Wesolowski verifiable delay
// function over a class group.
//
// TWO THINGS CHANGED HERE, AND THE SECOND MATTERS MORE.
//
// First, verifiability. Stage 2 was a sequential SHA-256 chain. Sequential it
// was, but checking it cost exactly as much as producing it -- every node had to
// walk all T links -- so the delay could never be set high enough to mean
// anything without making validation just as expensive. A Wesolowski proof is
// checked in a few hundred group operations whatever T is, so the delay can be
// raised to whatever the chain actually wants.
//
// Second, and less obvious: the input is derived from the block header ALONE,
// never from the nonce. A delay function inside a nonce search provides no delay
// at all. Each attempt would start its own independent chain, so a miner with n
// cores runs n of them at once and the wall-clock time for the block is one
// chain, not n -- the sequentiality is real per attempt and worthless per block.
// With the input fixed by the header there is exactly one chain per block, and
// it has to be walked end to end before any nonce can be tried.
func (h *HeliosAlgorithm) executeTimeLockPhase(blockHeader, parentOutput []byte) ([]byte, []byte, error) {
	discriminant, iterations, err := h.vdfParameters()
	if err != nil {
		return nil, nil, err
	}

	// The VDF input is a group element derived from the parent's delay output and
	// this block's header. Hashing into the group rather than using a fixed
	// generator matters: an element whose discrete logarithm to some published
	// base were known would let a prover shortcut the delay.
	seed := stage2Seed(blockHeader, parentOutput)
	input, err := vdf.HashToForm(seed, discriminant)
	if err != nil {
		return nil, nil, fmt.Errorf("map the header into the group: %w", err)
	}

	proof, err := vdf.Evaluate(input, iterations, discriminant)
	if err != nil {
		return nil, nil, fmt.Errorf("evaluate the delay function: %w", err)
	}

	return proof.Output.Bytes(), proof.Bytes(), nil
}

// stage2Seed derives the VDF input from the parent's delay output and this
// block's header.
//
// THE PARENT'S OUTPUT IS WHAT CHAINS THE DELAY.
//
// A per-block delay guarantees little on its own: if every block's delay could be
// computed independently, a miner with enough cores would compute a hundred of
// them at once and the chain would gain no elapsed-time property at all. Feeding
// block N-1's output into block N's input makes the delays one continuous
// sequential computation -- block N's cannot begin until block N-1's has
// finished, so a chain of k blocks costs at least k delays of wall-clock time no
// matter how much hardware is aimed at it.
//
// The header alone would very nearly do this, since it commits to PreviousHash.
// But that is a property of what happens to be in the header rather than of this
// function, and it would vanish silently if the header changed. The dependency is
// explicit here so it cannot be removed by accident.
//
// The length prefix is not decoration: without it a parent output ending in some
// bytes and a header beginning with them would be indistinguishable from a
// different split of the same concatenation, and two distinct blocks could share
// a delay input.
func stage2Seed(blockHeader, parentOutput []byte) []byte {
	h := sha256.New()
	h.Write([]byte("helios/stage2/vdf/input/v2"))

	var length [8]byte
	binary.BigEndian.PutUint64(length[:], uint64(len(parentOutput)))
	h.Write(length[:])
	h.Write(parentOutput)

	binary.BigEndian.PutUint64(length[:], uint64(len(blockHeader)))
	h.Write(length[:])
	h.Write(blockHeader)

	return h.Sum(nil)
}

// verifyTimeLockPhase checks a stage-2 result against its proof.
//
// This is the asymmetry the whole change exists for: the miner spent T
// sequential squarings, and this confirms it without repeating one of them.
func (h *HeliosAlgorithm) verifyTimeLockPhase(blockHeader, parentOutput, stage2Result, encodedProof []byte) error {
	discriminant, _, err := h.vdfParameters()
	if err != nil {
		return err
	}

	seed := stage2Seed(blockHeader, parentOutput)
	input, err := vdf.HashToForm(seed, discriminant)
	if err != nil {
		return fmt.Errorf("map the header into the group: %w", err)
	}

	proof, err := vdf.ProofFromBytes(encodedProof, discriminant)
	if err != nil {
		return fmt.Errorf("decode the delay proof: %w", err)
	}

	// The stage-2 bytes the rest of the proof is built on must be the output the
	// witness attests to; otherwise a miner could prove one delay and commit to
	// something else.
	if !bytes.Equal(proof.Output.Bytes(), stage2Result) {
		return fmt.Errorf("stage 2 result does not match the proven delay output")
	}

	if _, expected, err := h.vdfParameters(); err == nil && proof.Iterations != expected {
		return fmt.Errorf("delay proof claims %d iterations, the chain requires %d",
			proof.Iterations, expected)
	}

	return vdf.Verify(input, proof, discriminant)
}

// executeCryptographicPhase implements Stage 3: Cryptographic Phase (30% weight)
// Uses AES-NI optimized encryption.
//
// The key and nonce are DERIVED from the stage-2 result, not drawn from
// crypto/rand. This is the difference between a proof of work and a decoration:
// with random inputs the stage was non-deterministic, so a verifier could not
// recompute it and ValidateProof had to trust the stage outputs the miner
// supplied. That in turn meant a miner could invent arbitrary bytes for all three
// stages and brute-force only the cheap final SHA-256 -- skipping the memory-hard
// phase, the sequential phase and this one entirely, at zero cost.
func (h *HeliosAlgorithm) executeCryptographicPhase(stage2Result []byte) ([]byte, error) {
	if h.config.CryptoKeySize <= 0 {
		return nil, fmt.Errorf("crypto key size must be positive, got %d", h.config.CryptoKeySize)
	}

	// Derive the key and nonce deterministically, with domain separation so the
	// two are never equal.
	keyDigest := sha256.Sum256(append([]byte("helios/stage3/key"), stage2Result...))
	nonceDigest := sha256.Sum256(append([]byte("helios/stage3/nonce"), stage2Result...))

	key := make([]byte, h.config.CryptoKeySize)
	copy(key, keyDigest[:])
	nonce := make([]byte, gcmNonceSize)
	copy(nonce, nonceDigest[:])

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
		encrypted := gcm.Seal(nil, nonce, result, nil)

		// Truncate to key size. Guard the bound: a CryptoKeySize larger than the
		// ciphertext would slice out of range.
		n := h.config.CryptoKeySize
		if n > len(encrypted) {
			n = len(encrypted)
		}
		result = encrypted[:n]
	}

	return result, nil
}

// heliosFinalPreimage builds the preimage of the final proof hash.
func heliosFinalPreimage(blockHeader []byte, nonce uint64, stage1, stage2, stage3 []byte) []byte {
	nonceBytes := []byte(fmt.Sprintf("%d", nonce))
	buf := make([]byte, 0, len(blockHeader)+len(nonceBytes)+len(stage1)+len(stage2)+len(stage3))
	buf = append(buf, blockHeader...)
	buf = append(buf, nonceBytes...)
	buf = append(buf, stage1...)
	buf = append(buf, stage2...)
	buf = append(buf, stage3...)
	return buf
}

// ValidateProof validates a Helios proof against a block header and target.
//
// Every stage is RECOMPUTED from the header and nonce rather than taken on trust.
// The old implementation re-hashed the stage outputs stored *in the proof*, so the
// three stages constrained nothing: a miner could supply any bytes and grind only
// the final SHA-256. Recomputation is what makes the work binding.
func (h *HeliosAlgorithm) ValidateProof(proof *HeliosProof, blockHeader []byte, targetDifficulty *big.Int) error {
	return h.ValidateProofOnParent(proof, blockHeader, nil, targetDifficulty)
}

// ValidateProofOnParent is ValidateProof with the parent block's delay output.
//
// The verifier must supply the same parent output the miner used, or the derived
// VDF input differs and the proof fails -- which is exactly the binding that
// makes the delay sequential across blocks rather than merely within one.
func (h *HeliosAlgorithm) ValidateProofOnParent(proof *HeliosProof, blockHeader, parentOutput []byte, targetDifficulty *big.Int) error {
	if proof == nil {
		return fmt.Errorf("proof cannot be nil")
	}
	if targetDifficulty == nil {
		return fmt.Errorf("target difficulty cannot be nil")
	}

	stage1, err := h.executeMemoryPhase(blockHeader, proof.Nonce)
	if err != nil {
		return fmt.Errorf("failed to recompute memory phase: %w", err)
	}
	if !bytes.Equal(stage1, proof.Stage1Result) {
		return fmt.Errorf("stage 1 result does not match the recomputed value")
	}

	// Stage 2 is CHECKED, not recomputed.
	//
	// This is the whole point of the change. Recomputing meant every verifier
	// redid the entire sequential delay, so the delay could never exceed what a
	// validator was willing to spend. The Wesolowski witness is confirmed in a
	// few hundred group operations no matter how long the delay was.
	stage2 := proof.Stage2Result
	if err := h.verifyTimeLockPhase(blockHeader, parentOutput, stage2, proof.VDFProof); err != nil {
		return fmt.Errorf("time-lock proof is invalid: %w", err)
	}

	stage3, err := h.executeCryptographicPhase(stage2)
	if err != nil {
		return fmt.Errorf("failed to recompute cryptographic phase: %w", err)
	}
	if !bytes.Equal(stage3, proof.Stage3Result) {
		return fmt.Errorf("stage 3 result does not match the recomputed value")
	}

	finalHash := sha256.Sum256(heliosFinalPreimage(blockHeader, proof.Nonce, stage1, stage2, stage3))
	reconstructedHash := hex.EncodeToString(finalHash[:])

	if reconstructedHash != proof.FinalHash {
		return fmt.Errorf("proof hash mismatch: expected %s, got %s",
			proof.FinalHash, reconstructedHash)
	}

	// Check difficulty against the recomputed hash.
	hashInt := new(big.Int).SetBytes(finalHash[:])
	if hashInt.Cmp(targetDifficulty) > 0 {
		return fmt.Errorf("proof does not meet target difficulty")
	}

	return nil
}

// miningTimeout returns the configured mining timeout, or the default.
func (h *HeliosAlgorithm) miningTimeout() time.Duration {
	if h.config.MiningTimeout > 0 {
		return h.config.MiningTimeout
	}
	return defaultMiningTimeout
}
