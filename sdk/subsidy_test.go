package sdk

import (
	"errors"
	"math/big"
	"strings"
	"testing"
	"time"
)

// The subsidy is paid out of a finite reserve rather than minted. These cover the
// schedule, the amount rule that stops a miner overpaying itself, and the
// property the reserve exists for: that no sequence of blocks can create supply.

func subsidyTestChain(t *testing.T) *Blockchain {
	t.Helper()
	bc := forkTestChain(t, 0, uint32(genesisDifficulty))
	bc.cfg.InitialBlockSubsidy = 50
	bc.cfg.SubsidyHalvingInterval = 100
	bc.cfg.ReserveAddress = DeriveReserveAddress(bc.cfg.BlockchainName)
	return bc
}

// -----------------------------------------------------------------------------
// The schedule
// -----------------------------------------------------------------------------

func TestSubsidyHalves(t *testing.T) {
	bc := subsidyTestChain(t)
	full := AmountToUnits(50)

	cases := map[int64]int64{
		0:   0, // genesis is allocation, not subsidy
		1:   full,
		99:  full,
		100: full / 2,
		199: full / 2,
		200: full / 4,
		300: full / 8,
	}

	for height, want := range cases {
		if got := bc.BlockSubsidyUnits(height); got != want {
			t.Fatalf("height %d pays %s, want %s",
				height, formatUnits(got), formatUnits(want))
		}
	}
}

// TestSubsidyEventuallyStops: the schedule must terminate, or the reserve
// calculation is meaningless.
func TestSubsidyEventuallyStops(t *testing.T) {
	bc := subsidyTestChain(t)

	if got := bc.BlockSubsidyUnits(1 << 40); got != 0 {
		t.Fatalf("a far-future block still pays %s", formatUnits(got))
	}
	// And it never goes negative, whatever the height.
	for _, height := range []int64{-1, 0, 1, 1 << 20, 1 << 62} {
		if got := bc.BlockSubsidyUnits(height); got < 0 {
			t.Fatalf("height %d produced a negative subsidy: %d", height, got)
		}
	}
}

// TestSubsidyIsDeterministic: every node computes the amount independently, so it
// has to be a pure function of height.
func TestSubsidyIsDeterministic(t *testing.T) {
	first := subsidyTestChain(t)
	second := subsidyTestChain(t)

	for height := int64(1); height < 500; height += 37 {
		if first.BlockSubsidyUnits(height) != second.BlockSubsidyUnits(height) {
			t.Fatalf("two chains disagree on the subsidy at height %d", height)
		}
	}
}

// TestTotalEmissionFitsTheReserve is the sizing check. A schedule that emits more
// than the reserve holds simply stops early, which is a policy choice -- but it
// should be a deliberate one.
func TestTotalEmissionFitsTheReserve(t *testing.T) {
	cfg := NewConfig()
	bc := &Blockchain{cfg: cfg}

	emission := bc.TotalEmission()
	reserve := new(big.Int).Mul(
		big.NewInt(int64(float64(cfg.TokenCount)*cfg.ReserveAllocationPCT/100.0)),
		big.NewInt(UnitsPerToken))

	t.Logf("schedule emits %s over its life; reserve holds %s",
		formatUnits(emission.Int64()), formatUnits(reserve.Int64()))

	if emission.Cmp(reserve) > 0 {
		t.Fatalf("the schedule emits more than the reserve holds, so the subsidy "+
			"stops early: emission %s vs reserve %s",
			formatUnits(emission.Int64()), formatUnits(reserve.Int64()))
	}
}

// -----------------------------------------------------------------------------
// The amount rule
// -----------------------------------------------------------------------------

// mintSubsidyFor builds a well-formed subsidy without needing a funded reserve.
//
// buildSubsidyTransaction returns nil when the reserve is empty, which is correct
// but would make every test below skip -- and a skipped test of the amount rule
// is the amount rule going unchecked.
func mintSubsidyFor(t *testing.T, bc *Blockchain, height int64, minerAddress string) *Coinbase {
	t.Helper()

	tx, err := NewCoinbaseTransaction(
		addressOnlyWallet(bc.reserveAddress()),
		addressOnlyWallet(minerAddress),
		bc.cfg)
	if err != nil {
		t.Fatalf("build subsidy: %v", err)
	}
	tx.Fee = 0
	tx.SubsidyUnits = bc.BlockSubsidyUnits(height)
	tx.BlockHeight = height
	tx.SetStatus(StatusConfirmed)
	return tx
}

// TestOverpaidSubsidyIsRejected is the check that stops a miner paying itself
// whatever it likes.
func TestOverpaidSubsidyIsRejected(t *testing.T) {
	bc := subsidyTestChain(t)
	miner := coinbaseTestWallet(t, "greedy-miner")

	subsidy := mintSubsidyFor(t, bc, 1, miner.GetAddress())

	// The honest claim passes, so the failures below are about the amount.
	honest := NewBlock([]Transaction{subsidy}, bc.HeadHash())
	honest.Index = *big.NewInt(1)
	if err := bc.validateSubsidyLocked(honest); err != nil {
		t.Fatalf("a correctly-sized subsidy was refused: %v", err)
	}

	for _, claim := range []int64{
		subsidy.SubsidyUnits + 1,
		subsidy.SubsidyUnits * 1000,
		0,
		-1,
	} {
		greedy := *subsidy
		greedy.SubsidyUnits = claim

		block := NewBlock([]Transaction{&greedy}, bc.HeadHash())
		block.Index = *big.NewInt(1)

		err := bc.validateSubsidyLocked(block)
		if err == nil {
			t.Fatalf("a subsidy claiming %s was accepted where %s is allowed",
				formatUnits(claim), formatUnits(subsidy.SubsidyUnits))
		}
		if !errors.Is(err, ErrSubsidyAmount) {
			t.Fatalf("unexpected rejection for claim %d: %v", claim, err)
		}
	}
}

// TestSubsidyMustComeFromTheReserve: paying it from anywhere else would be
// creating money, which is the thing the reserve exists to prevent.
func TestSubsidyMustComeFromTheReserve(t *testing.T) {
	bc := subsidyTestChain(t)
	miner := coinbaseTestWallet(t, "reserve-check-miner")
	impostor := coinbaseTestWallet(t, "not-the-reserve")

	subsidy := mintSubsidyFor(t, bc, 1, miner.GetAddress())
	subsidy.From = impostor

	block := NewBlock([]Transaction{subsidy}, bc.HeadHash())
	block.Index = *big.NewInt(1)

	if err := bc.validateSubsidyLocked(block); err == nil {
		t.Fatal("a subsidy drawn from an address other than the reserve was accepted")
	}
}

// TestSubsidyMustDeclareItsOwnHeight stops a proof of one block's subsidy being
// reused at another height.
func TestSubsidyMustDeclareItsOwnHeight(t *testing.T) {
	bc := subsidyTestChain(t)
	miner := coinbaseTestWallet(t, "height-miner")

	subsidy := mintSubsidyFor(t, bc, 1, miner.GetAddress())
	subsidy.BlockHeight = 999

	block := NewBlock([]Transaction{subsidy}, bc.HeadHash())
	block.Index = *big.NewInt(1)

	if err := bc.validateSubsidyLocked(block); err == nil {
		t.Fatal("a subsidy declaring the wrong height was accepted")
	}
}

func TestGenesisIsExemptFromTheSubsidyRule(t *testing.T) {
	bc := subsidyTestChain(t)
	w := coinbaseTestWallet(t, "genesis-exempt")

	cb, err := NewCoinbaseTransaction(w, w, bc.cfg)
	if err != nil {
		t.Fatalf("coinbase: %v", err)
	}

	block := NewBlock([]Transaction{cb}, "")
	block.Index = *big.NewInt(0)

	if err := bc.validateSubsidyLocked(block); err != nil {
		t.Fatalf("genesis allocation was refused as a subsidy: %v", err)
	}
}

// -----------------------------------------------------------------------------
// The reserve
// -----------------------------------------------------------------------------

// TestReserveHasNoPrivateKey is the property that makes the reserve safe.
//
// A reserve behind a keypair puts the whole remaining emission behind one secret;
// whoever held it could drain the reserve with an ordinary transfer. Deriving the
// address from a domain string instead means no key produces it.
func TestReserveHasNoPrivateKey(t *testing.T) {
	address := DeriveReserveAddress("Go Basic Blockchain")

	if address == "" {
		t.Fatal("the reserve has no address")
	}
	if len(address) != 64 {
		t.Fatalf("the reserve address is %d characters, want a 64-character hash",
			len(address))
	}

	// Deterministic, so every node agrees on where the reserve is.
	if again := DeriveReserveAddress("Go Basic Blockchain"); again != address {
		t.Fatal("the reserve address is not deterministic")
	}
	// And chain-specific, so two chains do not share one.
	if other := DeriveReserveAddress("Another Chain"); other == address {
		t.Fatal("two different chains derived the same reserve address")
	}

	// No wallet can be opened at that address, because no key maps to it. The
	// closest check available here is that a freshly generated wallet never
	// lands on it.
	for i := 0; i < 5; i++ {
		w, err := NewWallet(NewWalletOptions(
			ThisBlockchainOrganizationID, ThisBlockchainAppID,
			ThisBlockchainAdminUserID, ThisBlockchainDevAssetID,
			"probe", testPassPhrase, []string{"t"}))
		if err != nil {
			t.Fatalf("wallet: %v", err)
		}
		if w.GetAddress() == address {
			t.Fatal("a generated wallet collided with the reserve address")
		}
	}
}

// TestSubsidyStopsWhenTheReserveEmpties. The reserve is finite by design; when it
// runs out miners are on transaction fees alone, which is the steady state the
// schedule bridges to.
func TestSubsidyStopsWhenTheReserveEmpties(t *testing.T) {
	bc := subsidyTestChain(t)

	// Nothing has funded the reserve on this fixture.
	scheduled, payable := bc.SubsidyFor(1)
	if scheduled <= 0 {
		t.Fatal("the schedule pays nothing at height 1")
	}
	if payable != 0 {
		t.Fatalf("an unfunded reserve offered to pay %s", formatUnits(payable))
	}

	tx, err := bc.buildSubsidyTransaction(1, "some-miner-address")
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if tx != nil {
		t.Fatal("a subsidy was built with nothing in the reserve")
	}
}

// -----------------------------------------------------------------------------
// The invariant
// -----------------------------------------------------------------------------

// TestSubsidiesDoNotCreateSupply is the whole point of paying from a reserve.
//
// A minting subsidy is only as safe as the check on its amount. A reserve-funded
// one cannot create coins whatever the schedule says -- the worst a mistake can
// do is empty the reserve early.
func TestSubsidiesDoNotCreateSupply(t *testing.T) {
	bc := integrationChain(t, t.TempDir())

	alice := integrationWallet(t, "supply-alice")
	bob := integrationWallet(t, "supply-bob")

	before := bc.UTXOSet().TotalUnits()
	if before <= 0 {
		t.Fatal("the chain starts with no supply")
	}

	for i := 0; i < 5; i++ {
		tx := freeMessage(t, alice, bob, "filler")
		if !bc.AddTransactionLocal(tx) {
			t.Fatalf("transaction %d refused", i)
		}
		bc.createNewBlock(bc.CurrentDifficulty())
	}

	if bc.Height() < 5 {
		t.Fatalf("only %d blocks were mined", bc.Height())
	}

	after := bc.UTXOSet().TotalUnits()
	if after != before {
		t.Fatalf("supply moved from %s to %s across %d subsidised blocks; the "+
			"subsidy is creating coins rather than moving them",
			formatUnits(before), formatUnits(after), bc.Height())
	}
}

// TestMinerIsActuallyPaid closes the loop: the subsidy has to arrive somewhere.
func TestMinerIsActuallyPaid(t *testing.T) {
	bc := integrationChain(t, t.TempDir())

	miner := bc.cfg.MinerAddress
	if miner == "" {
		t.Skip("no miner address configured")
	}

	reserveBefore := bc.GetBalanceUnits(bc.reserveAddress())
	minerBefore := bc.GetBalanceUnits(miner)
	if reserveBefore <= 0 {
		t.Fatal("the emission reserve was not funded at genesis")
	}

	alice := integrationWallet(t, "paid-alice")
	bob := integrationWallet(t, "paid-bob")
	tx := freeMessage(t, alice, bob, "pay the miner")
	if !bc.AddTransactionLocal(tx) {
		t.Fatal("transaction refused")
	}
	bc.createNewBlock(bc.CurrentDifficulty())

	minerAfter := bc.GetBalanceUnits(miner)
	reserveAfter := bc.GetBalanceUnits(bc.reserveAddress())

	paid := minerAfter - minerBefore
	drawn := reserveBefore - reserveAfter

	t.Logf("miner gained %s; reserve fell by %s", formatUnits(paid), formatUnits(drawn))

	if paid <= 0 {
		t.Fatal("the miner was not paid a subsidy")
	}
	if drawn != paid {
		t.Fatalf("the reserve fell by %s but the miner gained %s; the two must match",
			formatUnits(drawn), formatUnits(paid))
	}
}

// TestNlaakFeeShareStillApplies. The platform's per-transaction cut is the fee
// split, which the subsidy does not disturb.
func TestNlaakFeeShareStillApplies(t *testing.T) {
	bc := integrationChain(t, t.TempDir())

	if bc.cfg.DevAddress == "" {
		t.Skip("no developer address configured")
	}
	if bc.cfg.DevRewardPCT <= 0 {
		t.Skip("developer share is zero")
	}

	split := bc.feeSplitFor()
	if split.DevAddress != bc.cfg.DevAddress {
		t.Fatalf("the fee split names %q as the developer address, config says %q",
			split.DevAddress, bc.cfg.DevAddress)
	}
	if split.DevPercent != bc.cfg.DevRewardPCT {
		t.Fatalf("the fee split takes %.2f%%, config says %.2f%%",
			split.DevPercent, bc.cfg.DevRewardPCT)
	}
}

func TestSubsidyConfigIsValidated(t *testing.T) {
	for name, mutate := range map[string]func(*Config){
		"negative subsidy":       func(c *Config) { c.InitialBlockSubsidy = -1 },
		"negative interval":      func(c *Config) { c.SubsidyHalvingInterval = -1 },
		"reserve share over 100": func(c *Config) { c.ReserveAllocationPCT = 101 },
		"negative reserve share": func(c *Config) { c.ReserveAllocationPCT = -1 },
	} {
		t.Run(name, func(t *testing.T) {
			cfg := NewConfig()
			mutate(cfg)
			err := cfg.Validate()
			if err == nil {
				t.Fatal("an invalid subsidy configuration validated")
			}
			if !strings.Contains(err.Error(), "subsidy") &&
				!strings.Contains(err.Error(), "reserve") {
				t.Fatalf("the error does not mention the offending field: %v", err)
			}
		})
	}
}

var _ = time.Minute
