package sdk

import (
	"math/big"
	"testing"
)

type mockTransaction struct {
	id       string
	hash     string
	protocol string
}

func (m *mockTransaction) Process() string                    { return "" }
func (m *mockTransaction) GetProtocol() string                { return m.protocol }
func (m *mockTransaction) GetID() string                      { return m.id }
func (m *mockTransaction) GetHash() string                    { return m.hash }
func (m *mockTransaction) GetSignature() string               { return "" }
func (m *mockTransaction) GetSenderWallet() *Wallet           { return nil }
func (m *mockTransaction) GetRecipientWallet() *Wallet        { return nil }
func (m *mockTransaction) GetFee() float64                    { return 0 }
func (m *mockTransaction) GetStatus() TransactionStatus       { return StatusPending }
func (m *mockTransaction) SetStatus(status TransactionStatus) {}
func (m *mockTransaction) Sign(privPEM []byte) (string, error) {
	return "", nil
}
func (m *mockTransaction) Verify(pubKey []byte, sign string) (bool, error) {
	return true, nil
}
func (m *mockTransaction) Send(bc *Blockchain) error { return nil }
func (m *mockTransaction) String() string            { return m.id }
func (m *mockTransaction) Hex() string               { return "" }
func (m *mockTransaction) Hash() string {
	if m.hash == "" {
		m.hash = "generated-hash"
	}
	return m.hash
}
func (m *mockTransaction) Bytes() []byte { return []byte(m.id) }
func (m *mockTransaction) JSON() string  { return "{}" }
func (m *mockTransaction) Validate() error {
	return nil
}
func (m *mockTransaction) Size() int { return len(m.id) }
func (m *mockTransaction) SigningBytes() ([]byte, error) {
	return []byte(m.id + ":" + m.protocol), nil
}

func (m *mockTransaction) EstimateFee(feePerByte float64) float64 {
	return 0
}
func (m *mockTransaction) SetPriority(priority int) {}
func (m *mockTransaction) GetPriority() int         { return 0 }

func TestFIFOQueueOperations(t *testing.T) {
	q := NewFIFOQueue(2)

	if !q.IsEmpty() {
		t.Fatal("expected new queue to be empty")
	}

	if got := q.Dequeue(); got != "" {
		t.Fatalf("expected empty dequeue result, got %q", got)
	}

	q.Enqueue("a")
	q.Enqueue("b")
	q.Enqueue("b") // duplicate should be ignored

	// Enqueuing a duplicate at capacity must be a no-op. The old implementation
	// dequeued *before* the duplicate check, so re-adding "b" evicted the
	// unrelated entry "a" -- silent index loss, which this test used to assert as
	// intended behaviour.
	if got := q.Len(); got != 2 {
		t.Fatalf("expected queue length 2, got %d", got)
	}

	if !q.Exists("a") || !q.Exists("b") {
		t.Fatal("a duplicate enqueue must not evict an unrelated entry")
	}

	if got := q.Find("b"); got != "b" {
		t.Fatalf("expected Find to return 'b', got %q", got)
	}

	if got := q.Find("missing"); got != "" {
		t.Fatalf("expected Find miss to return empty string, got %q", got)
	}

	q.Enqueue("c") // capacity hit, oldest (a) should be removed
	if q.Exists("a") {
		t.Fatal("expected oldest element to be evicted at capacity")
	}

	idx := q.Get()
	if len(*idx) != 2 {
		t.Fatalf("expected backing index length 2, got %d", len(*idx))
	}

	// Set now respects the queue's capacity. It used to replace the backing slice
	// wholesale, so a restore could push the "bounded" cache past its bound and
	// keep it there.
	replacement := Index{"x", "y", "z"}
	q.Set(&replacement)
	if q.Len() != 2 {
		t.Fatalf("expected the restored queue to be capped at 2, got %d", q.Len())
	}
	if q.Exists("x") {
		t.Fatal("expected the oldest restored entry to be dropped at capacity")
	}
	if !q.Exists("y") || !q.Exists("z") {
		t.Fatal("expected the most recent restored entries to be retained")
	}
}

func TestTXLookupManagerSetGetAndFindErrors(t *testing.T) {
	m := NewTXLookupManager()

	if m.Initialized() {
		t.Fatal("expected manager to start uninitialized")
	}

	if err := m.Set(nil); err != nil {
		t.Fatalf("expected nil set to succeed, got error: %v", err)
	}

	if m.Initialized() {
		t.Fatal("expected manager to remain uninitialized after nil set")
	}

	idx := Index{"1:tx-1:hash-1"}
	if err := m.Set(&idx); err != nil {
		t.Fatalf("expected set to succeed, got error: %v", err)
	}

	if !m.Initialized() {
		t.Fatal("expected manager to be initialized after set")
	}

	if got := m.Get(); got == nil || len(*got) != 1 {
		t.Fatal("expected non-empty index from get")
	}

	_, err := m.Find(&IndexEntry{BlockNumber: *big.NewInt(1), TxID: "tx-1", TxHash: "hash-1"})
	if err == nil {
		t.Fatal("expected Find to reject multiple populated fields")
	}

	_, err = m.Find(&IndexEntry{})
	if err == nil {
		t.Fatal("expected Find to reject empty search")
	}

	_, err = m.Find(&IndexEntry{TxID: "does-not-exist"})
	if err == nil {
		t.Fatal("expected Find miss to fail")
	}

	entry, err := m.Find(&IndexEntry{BlockNumber: *big.NewInt(1)})
	if err != nil {
		t.Fatalf("expected Find hit, got error: %v", err)
	}
	if entry == nil {
		t.Fatal("expected non-nil entry for Find hit")
	}
}

func TestTXLookupManagerAddAndSplitMerge(t *testing.T) {
	m := NewTXLookupManager()
	block := &Block{Index: *big.NewInt(7)}
	block.Transactions = []Transaction{
		&mockTransaction{id: "tx-1", protocol: BankProtocolID},
		&mockTransaction{id: "tx-2", hash: "preset-hash", protocol: PersistProtocolID},
	}

	if err := m.Add(block); err != nil {
		t.Fatalf("expected add to succeed, got error: %v", err)
	}

	got := m.Get()
	if got == nil || len(*got) != 2 {
		t.Fatalf("expected 2 indexed entries, got %v", got)
	}

	merged := m.merge(*big.NewInt(9), "tx-9", "hash-9")
	if merged == "" {
		t.Fatal("expected merge output")
	}

	// split now returns an error and actually parses. Previously it used
	// fmt.Sscanf with "%s:%s:%s" -- which cannot split on colons -- and discarded
	// both return values, so every entry it produced was empty and this assertion
	// passed against a zero-valued struct.
	split, err := m.split(merged)
	if err != nil {
		t.Fatalf("expected split to succeed, got error: %v", err)
	}
	if split.BlockNumber.Int64() != 9 {
		t.Fatalf("expected block number 9, got %s", split.BlockNumber.String())
	}
	if split.TxID != "tx-9" {
		t.Fatalf("expected tx ID tx-9, got %q", split.TxID)
	}
	if split.TxHash != "hash-9" {
		t.Fatalf("expected tx hash hash-9, got %q", split.TxHash)
	}

	if _, err := m.split("not-a-valid-entry"); err == nil {
		t.Fatal("expected an error for a malformed index entry")
	}
	if _, err := m.split("notanumber:tx:hash"); err == nil {
		t.Fatal("expected an error for a non-numeric block number")
	}
}

// TestTXLookupFindByEachField covers lookups that were previously unreachable:
// Find gated on `BlockNumber.String() != ""`, and a zero big.Int stringifies to
// "0", so BlockNumber always looked populated and every TxID/TxHash lookup was
// rejected.
func TestTXLookupFindByEachField(t *testing.T) {
	m := NewTXLookupManager()
	entry := m.merge(*big.NewInt(42), "tx-42", "hash-42")
	m.index.Enqueue(entry)

	byID, err := m.Find(&IndexEntry{TxID: "tx-42"})
	if err != nil {
		t.Fatalf("find by tx ID failed: %v", err)
	}
	if byID.TxID != "tx-42" {
		t.Fatalf("expected tx-42, got %q", byID.TxID)
	}

	byHash, err := m.Find(&IndexEntry{TxHash: "hash-42"})
	if err != nil {
		t.Fatalf("find by tx hash failed: %v", err)
	}
	if byHash.TxHash != "hash-42" {
		t.Fatalf("expected hash-42, got %q", byHash.TxHash)
	}

	byBlock, err := m.Find(&IndexEntry{BlockNumber: *big.NewInt(42)})
	if err != nil {
		t.Fatalf("find by block number failed: %v", err)
	}
	if byBlock.BlockNumber.Int64() != 42 {
		t.Fatalf("expected block 42, got %s", byBlock.BlockNumber.String())
	}

	if _, err := m.Find(&IndexEntry{}); err == nil {
		t.Fatal("expected an error when no field is populated")
	}
	if _, err := m.Find(&IndexEntry{TxID: "a", TxHash: "b"}); err == nil {
		t.Fatal("expected an error when more than one field is populated")
	}
	if _, err := m.Find(nil); err == nil {
		t.Fatal("expected an error for a nil index entry")
	}
	if _, err := m.Find(&IndexEntry{TxID: "missing"}); err == nil {
		t.Fatal("expected an error for an unknown transaction")
	}
}

// TestFIFOQueueEvictionAndDeduplication covers the ring buffer that replaced the
// O(n^2), unbounded-memory slice implementation.
func TestFIFOQueueEvictionAndDeduplication(t *testing.T) {
	q := NewFIFOQueue(3)

	q.Enqueue("a")
	q.Enqueue("b")
	q.Enqueue("c")
	if q.Len() != 3 {
		t.Fatalf("expected 3 entries, got %d", q.Len())
	}

	// Re-adding an existing element must not evict an unrelated one. The old
	// Enqueue dequeued first and only then checked for duplicates.
	q.Enqueue("a")
	if q.Len() != 3 || !q.Exists("a") || !q.Exists("b") || !q.Exists("c") {
		t.Fatalf("duplicate enqueue disturbed the queue: len=%d", q.Len())
	}

	// Exceeding capacity evicts the oldest.
	q.Enqueue("d")
	if q.Exists("a") {
		t.Fatal("expected the oldest entry to be evicted")
	}
	if !q.Exists("d") || q.Len() != 3 {
		t.Fatalf("expected d present at capacity 3, got len=%d", q.Len())
	}

	// Insertion order is preserved.
	snapshot := q.Get()
	if len(*snapshot) != 3 || (*snapshot)[0] != "b" || (*snapshot)[2] != "d" {
		t.Fatalf("unexpected snapshot order: %v", *snapshot)
	}

	// Empty-queue behaviour.
	empty := NewFIFOQueue(2)
	if !empty.IsEmpty() || empty.Dequeue() != "" || empty.Find("x") != "" {
		t.Fatal("unexpected behaviour on an empty queue")
	}
	empty.Enqueue("")
	if !empty.IsEmpty() {
		t.Fatal("empty strings must not be indexed")
	}
}

// TestFIFOQueueSetTruncatesToCapacity guards the restore path.
func TestFIFOQueueSetTruncatesToCapacity(t *testing.T) {
	q := NewFIFOQueue(2)
	idx := Index{"one", "two", "three", "three"}
	q.Set(&idx)

	if q.Len() != 2 {
		t.Fatalf("expected the queue to hold at most 2 entries, got %d", q.Len())
	}
	if !q.Exists("three") {
		t.Fatal("expected the most recent entries to be retained")
	}
	q.Set(nil) // must not panic
}
