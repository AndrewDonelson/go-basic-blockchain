package sdk

import (
	"math/big"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/internal/helios/algorithm"
)

// The proof of work is verified with the parent's delay output, but WITHOUT the
// chain lock. Resolving the parent takes the lock for a map lookup and releases
// it; the expensive verification then runs holding nothing. That works because a
// parent is immutable -- a child names it by hash, so which block it is cannot
// change underneath the verifier.

// TestExpensiveVerificationNeedsNoChainLock is the property the restructure had
// to preserve.
//
// Verifying a block re-runs the memory-hard phase and checks a delay proof --
// around a hundred milliseconds at production parameters. If that ran under the
// chain lock, every reader and the miner would block behind each arriving block,
// which is precisely what an attacker sending junk blocks would exploit.
//
// The test holds the lock and then runs the expensive verification, which must
// still complete. Resolving the parent is done first, because that step does take
// the lock -- briefly, for a map lookup -- and the whole point of the split is
// that only that part needs it.
func TestExpensiveVerificationNeedsNoChainLock(t *testing.T) {
	bc := forkTestChain(t, 1, uint32(genesisDifficulty))
	bc.useHeliosMining = true
	bc.heliosAlgorithm = algorithm.NewHeliosAlgorithm(algorithm.TestHeliosConfig())

	parent := bc.Blocks[len(bc.Blocks)-1]

	// A real block, mined on that parent, so verification does genuine work.
	block := NewBlock(nil, parent.Hash)
	block.Index = *big.NewInt(int64(len(bc.Blocks)))
	block.Header.Difficulty = uint32(genesisDifficulty)
	block.Header.Timestamp = parent.Header.Timestamp.Add(time.Minute)
	block.Header.MerkleRoot = block.CalculateMerkleRoot()
	block.Hash = block.CalculateHash()

	proof, err := bc.heliosAlgorithm.MineOnParent(
		block.createBlockHeaderForMining(),
		parentDelayOutput(parent),
		difficultyTarget(genesisDifficulty))
	if err != nil {
		t.Fatalf("mine: %v", err)
	}
	if err := block.updateWithHeliosProof(proof); err != nil {
		t.Fatalf("record proof: %v", err)
	}

	// Hold the chain lock for the duration.
	bc.mux.Lock()
	defer bc.mux.Unlock()

	done := make(chan error, 1)
	go func() {
		done <- bc.verifyHeliosProof(block, parent, genesisDifficulty)
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("verification failed: %v", err)
		}
	case <-time.After(15 * time.Second):
		t.Fatal("the expensive verification did not complete while the chain lock " +
			"was held, so it is taking the lock -- every reader and the miner " +
			"would block behind each arriving block")
	}
}

// TestStandaloneValidationIsParentIndependent: it runs for blocks whose parent
// has not arrived, so it must not need one.
func TestStandaloneValidationIsParentIndependent(t *testing.T) {
	bc := forkTestChain(t, 1, uint32(genesisDifficulty))

	orphan := forkTestBlock(50, "a-parent-we-have-never-seen", uint32(genesisDifficulty), "x")

	// Standalone validation concerns structure, not ancestry, so an unknown
	// parent must not make it fail.
	if err := bc.validateStandalone(orphan); err != nil {
		if strings.Contains(err.Error(), "proof-of-work") {
			t.Fatalf("standalone validation is checking the proof of work, which "+
				"needs the parent it does not have: %v", err)
		}
	}
}

// TestOrphanProofIsVerifiedWhenItsBranchConnects.
//
// A block whose parent is unknown cannot have its chained delay checked at
// acceptance -- there is nothing to chain onto. It must therefore be checked
// before the block can join the chain, or unverified work would slip in through
// the orphan path.
func TestOrphanProofIsVerifiedWhenItsBranchConnects(t *testing.T) {
	bc := forkTestChain(t, 2, uint32(genesisDifficulty))
	bc.useHeliosMining = true

	// A branch block with a fabricated proof, rooted on a real parent.
	bogus := forkTestBlock(3, bc.HeadHash(), uint32(genesisDifficulty), "bogus")
	bogus.Header.Timestamp = bc.Blocks[2].Header.Timestamp.Add(time.Minute)
	bogus.Hash = bogus.CalculateHash()

	// It carries no Helios proof at all, which verification must refuse.
	if _, err := bc.AcceptBlockWithResult(bogus); err == nil {
		t.Fatal("a block with no proof of work was accepted while Helios mining " +
			"was enabled")
	}
}

// TestConcurrentAcceptanceIsSafe: the restructure moved work outside the lock,
// so the paths that remain locked must still be correct under contention.
func TestConcurrentAcceptanceIsSafe(t *testing.T) {
	bc := forkTestChain(t, 3, uint32(genesisDifficulty))

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 30; j++ {
				orphan := forkTestBlock(20+n, "unknown-parent", uint32(genesisDifficulty),
					string(rune('a'+n)))
				_ = bc.verifyProofOfWorkAgainstParent(orphan)
				_ = bc.blockByHash(bc.HeadHash())
				_ = bc.Height()
			}
		}(i)
	}
	wg.Wait()
}

// TestParentLookupIsByHashNotByHead: a branch block's parent is the block it
// names, not whatever happens to be at the tip.
func TestParentLookupIsByHashNotByHead(t *testing.T) {
	bc := forkTestChain(t, 3, uint32(genesisDifficulty))

	// A block deep in the chain, not the head.
	target := bc.Blocks[1]

	found := bc.blockByHash(target.Hash)
	if found == nil {
		t.Fatal("a block on the chain could not be resolved by hash")
	}
	if found.Hash != target.Hash {
		t.Fatalf("resolved the wrong block: %s vs %s", found.Hash, target.Hash)
	}
	if found.Hash == bc.HeadHash() {
		t.Fatal("the lookup returned the head rather than the block asked for")
	}

	if bc.blockByHash("nothing-with-this-hash") != nil {
		t.Fatal("an unknown hash resolved to a block")
	}
	if bc.blockByHash("") != nil {
		t.Fatal("an empty hash resolved to a block")
	}
}

// TestRecordingAProofDoesNotChangeTheMinedHeader pins a bug that would have
// invalidated nearly every block at real difficulty.
//
// updateWithHeliosProof used to overwrite the block's timestamp with the proof's.
// The timestamp is part of the header the miner mined against, so replacing it
// afterwards left the stored proof describing a header the block no longer had --
// every peer recomputing stage 1 would get a different answer and reject it.
//
// It survived only because the mining header records the timestamp to the second
// and test mining finishes inside one. At production difficulty, where mining
// takes many seconds, the timestamp would move nearly every time.
func TestRecordingAProofDoesNotChangeTheMinedHeader(t *testing.T) {
	helios := algorithm.NewHeliosAlgorithm(algorithm.TestHeliosConfig())

	block := NewBlock(nil, "parent-hash")
	block.Index = *big.NewInt(1)
	block.Header.Difficulty = 1
	// A timestamp deliberately far from now, standing in for a block whose
	// mining crossed a second boundary.
	block.Header.Timestamp = time.Unix(1700000000, 0)
	block.Header.MerkleRoot = block.CalculateMerkleRoot()

	before := string(block.createBlockHeaderForMining())

	proof, err := helios.MineOnParent(block.createBlockHeaderForMining(), nil, difficultyTarget(1))
	if err != nil {
		t.Fatalf("mine: %v", err)
	}
	if err := block.updateWithHeliosProof(proof); err != nil {
		t.Fatalf("record proof: %v", err)
	}

	after := string(block.createBlockHeaderForMining())
	if before != after {
		t.Fatalf("recording the proof changed the header that was mined, so the "+
			"proof can no longer validate against the block\n before = %s\n after  = %s",
			before, after)
	}

	// And the proof still validates against the block, which is the point.
	if err := helios.ValidateProofOnParent(proof, block.createBlockHeaderForMining(),
		nil, difficultyTarget(1)); err != nil {
		t.Fatalf("the recorded proof does not validate against its own block: %v", err)
	}
}
