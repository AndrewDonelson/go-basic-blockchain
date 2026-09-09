package sdk

import (
	"fmt"
	"testing"
	"time"
)

// Benchmarks for the two lookups on the path that peer traffic drives.
//
// Both used to be linear in the length of the chain and ran while holding the
// chain lock, so a peer sending transactions made every node re-scan its whole
// history per message -- blocking mining and every reader for the duration.

// benchChain builds a chain of the requested height with one transaction each.
func benchChain(tb testing.TB, height int) *Blockchain {
	tb.Helper()

	bc := forkTestChain(&testing.T{}, 0, uint32(genesisDifficulty))
	blocks := extendValidChain(bc, bc.Blocks, height, time.Minute, "")
	bc.Blocks = append(bc.Blocks, blocks...)
	bc.CurrentBlockIndex = len(bc.Blocks) - 1
	bc.NextBlockIndex = len(bc.Blocks)
	return bc
}

func BenchmarkHasTransactionID(b *testing.B) {
	for _, height := range []int{100, 1000, 5000} {
		b.Run(fmt.Sprintf("chain-%d", height), func(b *testing.B) {
			bc := benchChain(b, height)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				// A miss is the worst case: it cannot short-circuit.
				bc.HasTransactionID("no-such-transaction")
			}
		})
	}
}

func BenchmarkIndexMainChain(b *testing.B) {
	for _, height := range []int{100, 1000, 5000} {
		b.Run(fmt.Sprintf("chain-%d", height), func(b *testing.B) {
			bc := benchChain(b, height)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				bc.mux.Lock()
				bc.indexMainChainLocked()
				bc.mux.Unlock()
			}
		})
	}
}

// TestTransactionIndexIsInvalidatedByAReorg is the correctness half of the
// optimisation: a transaction rolled back must become unknown again, or it can
// never be mined onto the new branch.
func TestTransactionIndexIsInvalidatedByAReorg(t *testing.T) {
	alice, bob := utxoWallets(t)
	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	bc := forkTestChainWithGenesisTxs(t, 0, 4, mintTo(t, alice, alice, 500))

	spend, err := NewBankTransaction(alice, bob, 10)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	block := utxoBlock(t, 1, bc.HeadHash(), spend)
	if _, err := bc.AcceptBlockWithResult(block); err != nil {
		t.Fatalf("accept: %v", err)
	}

	if !bc.HasTransactionID(spend.GetID()) {
		t.Fatal("a mined transaction is not reported as known")
	}

	// Roll the chain back to genesis, as a reorganisation would.
	bc.mux.Lock()
	bc.Blocks = bc.Blocks[:1]
	bc.mux.Unlock()

	if bc.HasTransactionID(spend.GetID()) {
		t.Fatal("a transaction from a rolled-back block is still reported as known; " +
			"it could never be mined onto the new branch")
	}
}

// TestBlockIndexIsRebuiltWhenBlocksAreReplaced guards the same hazard for the
// block index: a stale prefix count would skip the replaced blocks, and their
// parents would then be unfindable.
func TestBlockIndexIsRebuiltWhenBlocksAreReplaced(t *testing.T) {
	bc := forkTestChain(t, 4, uint32(genesisDifficulty))

	bc.mux.Lock()
	bc.indexMainChainLocked()
	indexedBefore := bc.indexedBlocks
	bc.mux.Unlock()

	if indexedBefore != len(bc.Blocks) {
		t.Fatalf("indexed %d of %d blocks", indexedBefore, len(bc.Blocks))
	}

	// Replace the tail with different blocks, keeping the chain the same length,
	// exactly as a same-height reorganisation would.
	replacement := extendValidChain(bc, bc.Blocks[:3], 2, time.Minute, "rival")
	bc.mux.Lock()
	bc.Blocks = append(bc.Blocks[:3], replacement...)
	bc.indexMainChainLocked()
	bc.mux.Unlock()

	for _, block := range replacement {
		bc.mux.Lock()
		_, known := bc.blockIndex[block.Hash]
		bc.mux.Unlock()
		if !known {
			t.Fatalf("block %s replaced the chain tip but was never indexed; its "+
				"children could not find their parent", block.Index.String())
		}
	}
}

// TestTransactionIndexFindsEveryMinedTransaction is the basic contract.
func TestTransactionIndexFindsEveryMinedTransaction(t *testing.T) {
	bc := integrationChain(t, t.TempDir())

	alice := integrationWallet(t, "perf-index-alice")
	bob := integrationWallet(t, "perf-index-bob")

	var ids []string
	for i := 0; i < 5; i++ {
		tx := freeMessage(t, alice, bob, fmt.Sprintf("indexed %d", i))
		if !bc.AddTransactionLocal(tx) {
			t.Fatalf("transaction %d refused", i)
		}
		ids = append(ids, tx.GetID())
		bc.createNewBlock(bc.CurrentDifficulty())
	}

	for _, id := range ids {
		if !bc.HasTransactionID(id) {
			t.Fatalf("mined transaction %s is not reported as known", id)
		}
	}
	if bc.HasTransactionID("definitely-not-a-transaction") {
		t.Fatal("an unknown transaction is reported as known")
	}
	if bc.HasTransactionID("") {
		t.Fatal("an empty ID is reported as known")
	}
}

// linearScanHasTransactionID reproduces the previous implementation, so the
// benchmark below measures the change rather than describing it.
func linearScanHasTransactionID(bc *Blockchain, id string) bool {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	for _, tx := range bc.TransactionQueue {
		if tx != nil && tx.GetID() == id {
			return true
		}
	}
	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx != nil && tx.GetID() == id {
				return true
			}
		}
	}
	return false
}

// BenchmarkHasTransactionIDLinearScan is the baseline the index replaced.
func BenchmarkHasTransactionIDLinearScan(b *testing.B) {
	for _, height := range []int{100, 1000, 5000} {
		b.Run(fmt.Sprintf("chain-%d", height), func(b *testing.B) {
			bc := benchChain(b, height)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				linearScanHasTransactionID(bc, "no-such-transaction")
			}
		})
	}
}

// indexMainChainFullRebuild reproduces the previous implementation: every block
// re-inserted on every call.
func indexMainChainFullRebuild(bc *Blockchain) {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	if bc.blockIndex == nil {
		bc.blockIndex = map[string]*Block{}
	}
	for _, b := range bc.Blocks {
		bc.blockIndex[b.Hash] = b
	}
}

// BenchmarkIndexMainChainFullRebuild is the baseline. This ran once per accepted
// block, so syncing n blocks cost n(n+1)/2 map writes under the chain lock.
func BenchmarkIndexMainChainFullRebuild(b *testing.B) {
	for _, height := range []int{100, 1000, 5000} {
		b.Run(fmt.Sprintf("chain-%d", height), func(b *testing.B) {
			bc := benchChain(b, height)

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				indexMainChainFullRebuild(bc)
			}
		})
	}
}
