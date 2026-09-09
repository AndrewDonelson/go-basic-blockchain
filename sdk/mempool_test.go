package sdk

import (
	"fmt"
	"testing"
)

// fakeTx is a Transaction with a fee, a priority and a size we control.
//
// It embeds *Tx so it satisfies the whole interface, and overrides only the
// three things mempool policy actually reads. Building real signed transactions
// here would make the fee/size grid impossible to arrange and would dominate the
// test's runtime for no extra coverage of the policy itself.
type fakeTx struct {
	*Tx
	id   string
	size int
}

func (f *fakeTx) GetID() string { return f.id }
func (f *fakeTx) Size() int     { return f.size }

func newFakeTx(id string, fee float64, size int) *fakeTx {
	return &fakeTx{
		Tx:   &Tx{ID: NewPUIDEmpty(), Fee: fee},
		id:   id,
		size: size,
	}
}

// withParties gives a fake transaction real wallets, which anything that reaches
// block commit needs -- Validate refuses a nil sender or recipient, and the
// funds check needs an address to look up.
func (f *fakeTx) withParties(from, to *Wallet) *fakeTx {
	f.From = from
	f.To = to
	f.Protocol = MessageProtocolID
	f.Version = TransactionVersion
	return f
}

func (f *fakeTx) withPriority(p int) *fakeTx {
	f.SetPriority(p)
	return f
}

// mempoolTestChain builds a chain whose only interesting property is its
// mempool capacity.
func mempoolTestChain(t *testing.T, capacity int) *Blockchain {
	t.Helper()

	bc := forkTestChain(t, 0, uint32(genesisDifficulty))
	bc.cfg.MaxMempoolTxs = capacity
	return bc
}

func queuedIDs(bc *Blockchain) []string {
	ids := make([]string, 0, len(bc.TransactionQueue))
	for _, tx := range bc.TransactionQueue {
		ids = append(ids, tx.GetID())
	}
	return ids
}

func contains(ids []string, want string) bool {
	for _, id := range ids {
		if id == want {
			return true
		}
	}
	return false
}

// -----------------------------------------------------------------------------
// Fee rate
// -----------------------------------------------------------------------------

// TestFeeRateIsPerByte: ordering by raw fee would let one large transaction
// crowd out several small ones that together pay the miner more.
func TestFeeRateIsPerByte(t *testing.T) {
	big := newFakeTx("big", 5.0, 20000)
	small := newFakeTx("small", 1.0, 200)

	if txFeeRate(small) <= txFeeRate(big) {
		t.Fatalf("the small transaction pays %.8f per byte and the large one %.8f; "+
			"per-byte ranking should prefer the small one",
			txFeeRate(small), txFeeRate(big))
	}
	if !outranks(small, big) {
		t.Fatal("outranks should prefer the higher fee rate")
	}
}

// TestFeeRateHandlesDegenerateSize guards a divide by zero, and the crafted
// transaction that would rank infinitely valuable.
func TestFeeRateHandlesDegenerateSize(t *testing.T) {
	if got := txFeeRate(nil); got != 0 {
		t.Fatalf("a nil transaction should rate 0, got %v", got)
	}

	zero := newFakeTx("zero", 10.0, 0)
	rate := txFeeRate(zero)
	if rate != 10.0 {
		t.Fatalf("a zero-size transaction should be treated as one byte, got %v", rate)
	}

	negative := newFakeTx("negative", 10.0, -50)
	if got := txFeeRate(negative); got != 10.0 {
		t.Fatalf("a negative size should be treated as one byte, got %v", got)
	}
}

// TestPriorityOutranksFee makes the previously dead SetPriority load-bearing.
func TestPriorityOutranksFee(t *testing.T) {
	cheap := newFakeTx("cheap", 0.01, 250).withPriority(10)
	rich := newFakeTx("rich", 100.0, 250)

	if !outranks(cheap, rich) {
		t.Fatal("a higher priority should win regardless of fee, " +
			"otherwise SetPriority is stored and never read")
	}
}

// -----------------------------------------------------------------------------
// Admission
// -----------------------------------------------------------------------------

// TestMempoolIsBounded is the whole point of the cap: an unbounded queue is an
// attacker-controlled allocation.
func TestMempoolIsBounded(t *testing.T) {
	bc := mempoolTestChain(t, 8)

	for i := 0; i < 200; i++ {
		bc.mux.Lock()
		_ = bc.admitToMempoolLocked(newFakeTx(fmt.Sprintf("tx-%d", i), 0.05, 250))
		bc.mux.Unlock()
	}

	if got := len(bc.TransactionQueue); got != 8 {
		t.Fatalf("mempool holds %d transactions against a cap of 8", got)
	}
}

// TestFullMempoolRejectsALowerBid: evicting unconditionally would let an attacker
// flush every paying transaction out with a stream of cheap ones.
func TestFullMempoolRejectsALowerBid(t *testing.T) {
	bc := mempoolTestChain(t, 3)

	for i := 0; i < 3; i++ {
		bc.mux.Lock()
		if err := bc.admitToMempoolLocked(newFakeTx(fmt.Sprintf("rich-%d", i), 10.0, 250)); err != nil {
			t.Fatalf("admit: %v", err)
		}
		bc.mux.Unlock()
	}

	bc.mux.Lock()
	err := bc.admitToMempoolLocked(newFakeTx("cheap", 0.0001, 250))
	bc.mux.Unlock()

	if err == nil {
		t.Fatal("a cheaper transaction was admitted to a full mempool, so an attacker " +
			"could evict paying transactions with worthless ones")
	}
	for _, id := range queuedIDs(bc) {
		if id == "cheap" {
			t.Fatal("the cheap transaction is in the mempool")
		}
	}
}

// TestFullMempoolEvictsForAHigherBid is the other half: early cheap transactions
// must not hold the mempool hostage against later paying ones.
func TestFullMempoolEvictsForAHigherBid(t *testing.T) {
	bc := mempoolTestChain(t, 3)

	for i := 0; i < 3; i++ {
		bc.mux.Lock()
		if err := bc.admitToMempoolLocked(newFakeTx(fmt.Sprintf("cheap-%d", i), 0.01, 250)); err != nil {
			t.Fatalf("admit: %v", err)
		}
		bc.mux.Unlock()
	}

	bc.mux.Lock()
	err := bc.admitToMempoolLocked(newFakeTx("rich", 50.0, 250))
	bc.mux.Unlock()

	if err != nil {
		t.Fatalf("a higher-paying transaction was refused by a full mempool: %v", err)
	}
	if !contains(queuedIDs(bc), "rich") {
		t.Fatal("the higher-paying transaction is not in the mempool")
	}
	if got := len(bc.TransactionQueue); got != 3 {
		t.Fatalf("eviction left %d transactions against a cap of 3", got)
	}
}

func TestAdmitRejectsNil(t *testing.T) {
	bc := mempoolTestChain(t, 4)
	bc.mux.Lock()
	err := bc.admitToMempoolLocked(nil)
	bc.mux.Unlock()
	if err == nil {
		t.Fatal("a nil transaction was admitted")
	}
}

func TestMempoolCapacityFallsBackWhenUnset(t *testing.T) {
	bc := mempoolTestChain(t, 0)
	if got := bc.mempoolCapacity(); got != defaultMaxMempoolTxs {
		t.Fatalf("a zero cap should fall back to %d, got %d", defaultMaxMempoolTxs, got)
	}
}

// -----------------------------------------------------------------------------
// Block selection
// -----------------------------------------------------------------------------

// TestBlockSelectionRespectsMaxBlockSize is the defect this closes:
// CanAddTransaction and MaxBlockSize both existed, and createNewBlock drained the
// entire mempool into a block regardless.
func TestBlockSelectionRespectsMaxBlockSize(t *testing.T) {
	bc := mempoolTestChain(t, 100)

	// Twenty transactions of 100KB each is 2MB against a 1MB cap.
	each := 100000
	for i := 0; i < 20; i++ {
		bc.mux.Lock()
		if err := bc.admitToMempoolLocked(newFakeTx(fmt.Sprintf("fat-%d", i), 1.0, each)); err != nil {
			t.Fatalf("admit: %v", err)
		}
		bc.mux.Unlock()
	}

	bc.mux.Lock()
	selected := bc.selectBlockTransactionsLocked()
	leftover := len(bc.TransactionQueue)
	bc.mux.Unlock()

	total := blockHeaderOverhead
	for _, tx := range selected {
		total += tx.Size()
	}
	if total > MaxBlockSize {
		t.Fatalf("selection produced %d bytes against a %d byte limit", total, MaxBlockSize)
	}
	if len(selected) == 0 {
		t.Fatal("selection produced an empty block despite a full mempool")
	}
	// What did not fit must still be queued, not dropped.
	if len(selected)+leftover != 20 {
		t.Fatalf("%d selected + %d left over != 20 submitted; transactions were lost",
			len(selected), leftover)
	}
}

// TestBlockSelectionPrefersHigherFeeRates.
func TestBlockSelectionPrefersHigherFeeRates(t *testing.T) {
	bc := mempoolTestChain(t, 100)

	bc.mux.Lock()
	_ = bc.admitToMempoolLocked(newFakeTx("poor", 0.01, 250))
	_ = bc.admitToMempoolLocked(newFakeTx("rich", 5.00, 250))
	_ = bc.admitToMempoolLocked(newFakeTx("middling", 0.50, 250))
	selected := bc.selectBlockTransactionsLocked()
	bc.mux.Unlock()

	if len(selected) != 3 {
		t.Fatalf("expected all three to fit, got %d", len(selected))
	}
	want := []string{"rich", "middling", "poor"}
	for i, id := range want {
		if selected[i].GetID() != id {
			t.Fatalf("position %d is %s, want %s (order was %v)",
				i, selected[i].GetID(), id, func() []string {
					out := []string{}
					for _, tx := range selected {
						out = append(out, tx.GetID())
					}
					return out
				}())
		}
	}
}

// TestBlockSelectionFillsRemainingSpace: stopping at the first transaction that
// does not fit would waste block space a smaller one could use.
func TestBlockSelectionFillsRemainingSpace(t *testing.T) {
	bc := mempoolTestChain(t, 100)

	huge := MaxBlockSize - blockHeaderOverhead - 1000
	blocked := MaxBlockSize / 2

	bc.mux.Lock()
	// Highest rate, and nearly fills the block. The fees are chosen so the
	// per-byte ordering is huge > blocked > tiny; sizes this different make it
	// easy to write a fixture whose rates rank the opposite way to its names.
	_ = bc.admitToMempoolLocked(newFakeTx("huge", 0.004*float64(huge), huge))
	// Next by rate, but cannot possibly fit alongside it.
	_ = bc.admitToMempoolLocked(newFakeTx("blocked", 0.002*float64(blocked), blocked))
	// Lowest rate, but fits in what is left.
	_ = bc.admitToMempoolLocked(newFakeTx("tiny", 0.001*500, 500))
	selected := bc.selectBlockTransactionsLocked()
	bc.mux.Unlock()

	ids := []string{}
	for _, tx := range selected {
		ids = append(ids, tx.GetID())
	}
	if !contains(ids, "huge") {
		t.Fatalf("the highest-paying transaction was not selected: %v", ids)
	}
	if !contains(ids, "tiny") {
		t.Fatalf("a small transaction that fits in the remaining space was skipped: %v", ids)
	}
	if contains(ids, "blocked") {
		t.Fatalf("a transaction that cannot fit was selected: %v", ids)
	}
}

// TestBlockSelectionIsDeterministic: two nodes with the same mempool should build
// the same block, and ties must not reorder run to run.
func TestBlockSelectionIsDeterministic(t *testing.T) {
	build := func() []string {
		bc := mempoolTestChain(t, 100)
		bc.mux.Lock()
		for i := 0; i < 30; i++ {
			// Deliberately many ties.
			_ = bc.admitToMempoolLocked(newFakeTx(fmt.Sprintf("tx-%d", i), 0.05, 250))
		}
		selected := bc.selectBlockTransactionsLocked()
		bc.mux.Unlock()

		ids := []string{}
		for _, tx := range selected {
			ids = append(ids, tx.GetID())
		}
		return ids
	}

	first := build()
	for i := 0; i < 20; i++ {
		next := build()
		if len(next) != len(first) {
			t.Fatalf("selection size varies: %d vs %d", len(next), len(first))
		}
		for j := range first {
			if next[j] != first[j] {
				t.Fatalf("selection order varies at %d: %s vs %s", j, next[j], first[j])
			}
		}
	}
}

func TestBlockSelectionOnAnEmptyMempool(t *testing.T) {
	bc := mempoolTestChain(t, 10)
	bc.mux.Lock()
	selected := bc.selectBlockTransactionsLocked()
	bc.mux.Unlock()
	if len(selected) != 0 {
		t.Fatalf("an empty mempool produced %d transactions", len(selected))
	}
}

// TestBlockSelectionDropsNilEntries: a nil in the queue is unusable and must not
// be carried forever or panic the selector.
func TestBlockSelectionDropsNilEntries(t *testing.T) {
	bc := mempoolTestChain(t, 10)

	bc.mux.Lock()
	bc.TransactionQueue = []Transaction{
		newFakeTx("real", 1.0, 250),
		nil,
		newFakeTx("also-real", 2.0, 250),
	}
	selected := bc.selectBlockTransactionsLocked()
	leftover := len(bc.TransactionQueue)
	bc.mux.Unlock()

	if len(selected) != 2 {
		t.Fatalf("expected the two real transactions, got %d", len(selected))
	}
	if leftover != 0 {
		t.Fatalf("the nil entry is still queued (%d left)", leftover)
	}
}

// -----------------------------------------------------------------------------
// Trimming
// -----------------------------------------------------------------------------

// TestRequeueCannotExceedTheCap: requeue paths bypass admission control by
// design, so the bound has to be re-applied or a reorg could blow past it.
func TestRequeueCannotExceedTheCap(t *testing.T) {
	bc := mempoolTestChain(t, 5)

	restored := make([]Transaction, 0, 50)
	for i := 0; i < 50; i++ {
		restored = append(restored, newFakeTx(fmt.Sprintf("restored-%d", i), 1.0, 250))
	}
	bc.requeueTransactions(restored)

	if got := len(bc.TransactionQueue); got != 5 {
		t.Fatalf("requeue left %d transactions against a cap of 5", got)
	}
}

// TestTrimKeepsTheHighestPayers: if trimming dropped the wrong end, a reorg would
// discard exactly the transactions worth keeping.
func TestTrimKeepsTheHighestPayers(t *testing.T) {
	bc := mempoolTestChain(t, 3)

	bc.mux.Lock()
	bc.TransactionQueue = []Transaction{
		newFakeTx("cheap-a", 0.01, 250),
		newFakeTx("rich-1", 90.0, 250),
		newFakeTx("cheap-b", 0.02, 250),
		newFakeTx("rich-2", 80.0, 250),
		newFakeTx("cheap-c", 0.03, 250),
		newFakeTx("rich-3", 70.0, 250),
	}
	bc.trimMempoolLocked()
	bc.mux.Unlock()

	ids := queuedIDs(bc)
	if len(ids) != 3 {
		t.Fatalf("trim left %d transactions against a cap of 3: %v", len(ids), ids)
	}
	for _, want := range []string{"rich-1", "rich-2", "rich-3"} {
		if !contains(ids, want) {
			t.Fatalf("trim dropped %s, which is among the highest payers: %v", want, ids)
		}
	}
}

func TestTrimIsANoOpBelowCapacity(t *testing.T) {
	bc := mempoolTestChain(t, 10)

	bc.mux.Lock()
	bc.TransactionQueue = []Transaction{
		newFakeTx("a", 1.0, 250),
		newFakeTx("b", 2.0, 250),
	}
	bc.trimMempoolLocked()
	got := len(bc.TransactionQueue)
	bc.mux.Unlock()

	if got != 2 {
		t.Fatalf("trim removed transactions while under capacity: %d remain", got)
	}
}

// -----------------------------------------------------------------------------
// End to end
// -----------------------------------------------------------------------------

// TestMinedBlockRespectsMaxBlockSize closes the loop through createNewBlock,
// which is where the unbounded drain actually lived.
func TestMinedBlockRespectsMaxBlockSize(t *testing.T) {
	bc := mempoolTestChain(t, 100)
	bc.useHeliosMining = false

	newWallet := func(name string) *Wallet {
		w, err := NewWallet(NewWalletOptions(
			ThisBlockchainOrganizationID, ThisBlockchainAppID,
			ThisBlockchainAdminUserID, ThisBlockchainDevAssetID,
			name, testPassPhrase, []string{"test"}))
		if err != nil {
			t.Fatalf("%s wallet: %v", name, err)
		}
		return w
	}
	from, to := newWallet("mempool-sender"), newWallet("mempool-recipient")
	// Only the UTXO set matters here: ValidateTransactionFunds is the
	// authoritative check, and the wallet's own cached balance is advisory (and
	// unreachable while the wallet is encrypted).
	creditAddressForTest(t, bc, from.GetAddress(), 1000)

	for i := 0; i < 20; i++ {
		bc.mux.Lock()
		_ = bc.admitToMempoolLocked(
			newFakeTx(fmt.Sprintf("fat-%d", i), 1.0, 100000).withParties(from, to))
		bc.mux.Unlock()
	}

	startHeight := bc.Height()
	bc.createNewBlock(bc.CurrentDifficulty())

	head := bc.GetLatestBlock()
	if head == nil {
		t.Fatal("no block was produced")
	}
	// Without this the test passes vacuously when the block is refused and the
	// head is still genesis.
	if bc.Height() != startHeight+1 {
		t.Fatalf("no block was committed: height is still %d", bc.Height())
	}

	size := blockHeaderOverhead
	for _, tx := range head.Transactions {
		size += tx.Size()
	}
	if size > MaxBlockSize {
		t.Fatalf("the mined block is %d bytes, above the %d byte limit", size, MaxBlockSize)
	}
	if bc.GetMempoolSize() == 0 {
		t.Fatal("the whole mempool went into one block, so the size limit did nothing")
	}
}
