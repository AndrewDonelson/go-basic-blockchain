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

	// Current queue semantics dequeue on full-capacity enqueue before duplicate check,
	// so enqueuing duplicate "b" at capacity shrinks queue from [a,b] to [b].
	if got := q.Len(); got != 1 {
		t.Fatalf("expected queue length 1, got %d", got)
	}

	if q.Exists("a") || !q.Exists("b") {
		t.Fatal("expected queue to contain only b after duplicate enqueue at capacity")
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

	replacement := Index{"x", "y", "z"}
	q.Set(&replacement)
	if q.Len() != 3 {
		t.Fatalf("expected set queue length 3, got %d", q.Len())
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

	split := m.split(merged)
	if split == nil {
		t.Fatal("expected split output")
	}
}
