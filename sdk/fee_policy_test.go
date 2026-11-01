package sdk

import (
	"math/big"
	"strings"
	"testing"
)

// feePolicyChain builds a chain whose genesis fixes the fee policy: a coinbase
// minting to `funded` and naming `dev` as the developer at devPercent.
//
// The node's own config is then pointed somewhere else entirely, which is the
// situation the fee bug was about: a node whose DEV_ADDRESS and MINER_ADDRESS
// say one thing while the chain says another.
func feePolicyChain(t *testing.T, funded *Wallet, dev string, devPercent float64) *Blockchain {
	t.Helper()

	genesisCfg := NewConfig()
	genesisCfg.DevAddress = dev
	genesisCfg.DevRewardPCT = devPercent
	cb, err := NewCoinbaseTransaction(funded, funded, genesisCfg)
	if err != nil {
		t.Fatalf("genesis coinbase: %v", err)
	}
	cb.TokenCount = 1000
	// The wallet's own balance is what NewBankTransaction checks; the chain's is
	// what the coinbase above gives it.
	if err := funded.SetData("balance", 1000.0); err != nil {
		t.Fatalf("set balance: %v", err)
	}

	bc := forkTestChainWithGenesisTxs(t, 0, 4, cb)
	bc.cfg.MinerAddress = strings.Repeat("a", 64)
	bc.cfg.DevAddress = strings.Repeat("b", 64)
	bc.cfg.DevRewardPCT = 90
	return bc
}

// peerFeeBlock builds the next block on bc, naming `miner` and carrying one
// signed transfer, as a peer would send it.
func peerFeeBlock(t *testing.T, bc *Blockchain, miner string, from, to *Wallet) (*Block, int64) {
	t.Helper()

	tx, err := NewBankTransaction(from, to, 10)
	if err != nil {
		t.Fatalf("transfer: %v", err)
	}
	signForTest(t, tx)

	b := NewBlock([]Transaction{tx}, bc.HeadHash())
	b.Index = *big.NewInt(int64(len(bc.Blocks)))
	b.Header.Difficulty = 4
	b.Header.Miner = miner
	b.Header.MerkleRoot = b.CalculateMerkleRoot()
	b.Hash = b.CalculateHash()
	return b, AmountToUnits(tx.GetFee())
}

// TestPeerBlockFeesGoToTheMinerItNames pins the consensus bug: fees used to be
// paid to the ACCEPTING node's configured miner and developer, so a block from a
// peer paid its fees to a different address on every node, and the UTXO sets of
// the network disagreed from the first transaction on.
func TestPeerBlockFeesGoToTheMinerItNames(t *testing.T) {
	from := newTestWallet(t, "fee-from", 0)
	to := newTestWallet(t, "fee-to", 0)
	peerMiner := coinbaseTestWallet(t, "fee-peer-miner").GetAddress()
	dev := coinbaseTestWallet(t, "fee-dev").GetAddress()

	bc := feePolicyChain(t, from, dev, 50)
	block, feeUnits := peerFeeBlock(t, bc, peerMiner, from, to)
	if feeUnits <= 0 {
		t.Fatal("the transfer carries no fee; the test would prove nothing")
	}

	if _, err := bc.AcceptBlockWithResult(block); err != nil {
		t.Fatalf("accept peer block: %v", err)
	}

	devWant := feeUnits / 2
	if got := bc.GetBalanceUnits(dev); got != devWant {
		t.Fatalf("genesis developer got %s, want %s", formatUnits(got), formatUnits(devWant))
	}
	if got := bc.GetBalanceUnits(peerMiner); got != feeUnits-devWant {
		t.Fatalf("the block's miner got %s, want %s", formatUnits(got), formatUnits(feeUnits-devWant))
	}
	for name, addr := range map[string]string{
		"local MINER_ADDRESS": bc.cfg.MinerAddress,
		"local DEV_ADDRESS":   bc.cfg.DevAddress,
	} {
		if got := bc.GetBalanceUnits(addr); got != 0 {
			t.Fatalf("the %s was paid %s for a block it did not mine", name, formatUnits(got))
		}
	}
}

// TestNodesAgreeOnFeesWhateverTheirConfig is the network-level statement of the
// same rule: two nodes configured differently apply one block identically.
func TestNodesAgreeOnFeesWhateverTheirConfig(t *testing.T) {
	from := newTestWallet(t, "agree-from", 0)
	to := newTestWallet(t, "agree-to", 0)
	peerMiner := coinbaseTestWallet(t, "agree-miner").GetAddress()
	dev := coinbaseTestWallet(t, "agree-dev").GetAddress()

	a := feePolicyChain(t, from, dev, 50)
	b := &Blockchain{cfg: NewConfig(), TXLookup: NewTXLookupManager(), Blocks: append([]*Block{}, a.Blocks...)}
	b.cfg.DataPath = a.cfg.DataPath
	b.cfg.MinerAddress = strings.Repeat("c", 64)
	b.cfg.DevAddress = ""
	b.cfg.DevRewardPCT = 0
	if err := b.RebuildUTXOSet(); err != nil {
		t.Fatalf("rebuild: %v", err)
	}

	block, _ := peerFeeBlock(t, a, peerMiner, from, to)
	for _, node := range []*Blockchain{a, b} {
		if _, err := node.ensureUTXOSetLocked().ApplyBlock(block, node.feePolicyLocked()); err != nil {
			t.Fatalf("apply: %v", err)
		}
	}

	for _, addr := range []string{from.GetAddress(), to.GetAddress(), peerMiner, dev,
		a.cfg.MinerAddress, a.cfg.DevAddress, b.cfg.MinerAddress} {
		if ga, gb := a.GetBalanceUnits(addr), b.GetBalanceUnits(addr); ga != gb {
			t.Fatalf("nodes disagree on %s: %s vs %s", addr, formatUnits(ga), formatUnits(gb))
		}
	}
}

// TestFeePolicyComesFromGenesisNotConfig: the developer cut is a chain
// parameter. Changing DEV_ADDRESS or DEV_REWARD_PCT on one node must not move it.
func TestFeePolicyComesFromGenesisNotConfig(t *testing.T) {
	bc := integrationChain(t, t.TempDir())

	bc.mux.Lock()
	policy := bc.feePolicyLocked()
	bc.mux.Unlock()

	if !IsAddress(policy.DevAddress) {
		t.Fatalf("a freshly created chain has no developer address in its genesis: %q", policy.DevAddress)
	}
	if policy.DevPercent != devRewardPCT {
		t.Fatalf("genesis developer share = %.2f%%, want the default %.2f%%", policy.DevPercent, float64(devRewardPCT))
	}

	bc.cfg.DevAddress = strings.Repeat("d", 64)
	bc.cfg.DevRewardPCT = 99

	bc.mux.Lock()
	after := bc.feePolicyLocked()
	bc.mux.Unlock()
	if after != policy {
		t.Fatalf("editing the node's config moved the fee policy from %+v to %+v", policy, after)
	}
}

// TestMinerIsCommittedByTheHash: if the miner were outside the hash, anyone
// relaying a block could rewrite who its fees go to.
func TestMinerIsCommittedByTheHash(t *testing.T) {
	b := NewBlock(nil, "prev")
	b.Index = *big.NewInt(1)
	b.Header.Miner = strings.Repeat("1", 64)
	hash := b.CalculateHash()
	preimage := string(b.createBlockHeaderForMining())

	b.Header.Miner = strings.Repeat("2", 64)
	if b.CalculateHash() == hash {
		t.Fatal("changing the miner did not change the block hash")
	}
	if string(b.createBlockHeaderForMining()) == preimage {
		t.Fatal("changing the miner did not change the proof-of-work preimage")
	}
}

// TestMinerMustBeAnAddress: fees paid to a string nobody can spend from are
// burned in all but name.
func TestMinerMustBeAnAddress(t *testing.T) {
	bc := subsidyTestChain(t)

	block := NewBlock(nil, bc.HeadHash())
	block.Index = *big.NewInt(1)
	block.Header.Miner = "not-an-address"
	if err := bc.validateSubsidyLocked(block); err == nil {
		t.Fatal("a block naming a non-address as its miner was accepted")
	}

	block.Header.Miner = ""
	if err := bc.validateSubsidyLocked(block); err != nil {
		t.Fatalf("a block naming no miner was refused: %v", err)
	}
}

// TestSubsidyMustPayTheHeaderMiner: one block, one miner.
func TestSubsidyMustPayTheHeaderMiner(t *testing.T) {
	bc := subsidyTestChain(t)
	miner := coinbaseTestWallet(t, "header-miner")
	other := coinbaseTestWallet(t, "other-miner")

	block := NewBlock([]Transaction{mintSubsidyFor(t, bc, 1, miner.GetAddress())}, bc.HeadHash())
	block.Index = *big.NewInt(1)
	block.Header.Miner = other.GetAddress()
	if err := bc.validateSubsidyLocked(block); err == nil {
		t.Fatal("a subsidy paying one address in a block naming another was accepted")
	}

	block.Header.Miner = miner.GetAddress()
	if err := bc.validateSubsidyLocked(block); err != nil {
		t.Fatalf("a subsidy paying the header's miner was refused: %v", err)
	}
}

// TestEmitFeesRouting covers the edge cases of the split.
func TestEmitFeesRouting(t *testing.T) {
	type paid map[string]int64
	run := func(miner string, policy feePolicy) paid {
		out := paid{}
		NewUTXOSet().emitFees(1000, miner, policy, func(addr string, units int64, _ bool) {
			out[addr] += units
		})
		return out
	}

	cases := []struct {
		name   string
		miner  string
		policy feePolicy
		want   paid
	}{
		{"split", "m", feePolicy{"d", 50}, paid{"m": 500, "d": 500}},
		{"no miner gives all to dev", "", feePolicy{"d", 50}, paid{"d": 1000}},
		{"no dev gives all to miner", "m", feePolicy{"", 50}, paid{"m": 1000}},
		{"over 100 is ignored", "m", feePolicy{"d", 150}, paid{"m": 1000}},
		{"negative is ignored", "m", feePolicy{"d", -5}, paid{"m": 1000}},
		{"full share", "m", feePolicy{"d", 100}, paid{"d": 1000}},
	}
	for _, c := range cases {
		got := run(c.miner, c.policy)
		if len(got) != len(c.want) {
			t.Fatalf("%s: paid %v, want %v", c.name, got, c.want)
		}
		for k, v := range c.want {
			if got[k] != v {
				t.Fatalf("%s: paid %v, want %v", c.name, got, c.want)
			}
		}
	}
}

// TestShortAddressNeverSlicesPastTheEnd: a reloaded node kept the five-letter
// "MINER" placeholder as its miner address, and the transaction log line sliced
// it to eight characters, panicking on the first block the node mined.
func TestShortAddressNeverSlicesPastTheEnd(t *testing.T) {
	for in, want := range map[string]string{
		"":                       "",
		"MINER":                  "MINER",
		strings.Repeat("ab", 32): "abababab",
	} {
		if got := shortAddress(in); got != want {
			t.Fatalf("shortAddress(%q) = %q, want %q", in, got, want)
		}
	}
}
