package sidechain

import (
	"testing"
	"time"
)

func TestRouteTransactionAndQueueStatus(t *testing.T) {
	r := NewProtocolRouter()

	tx, err := r.RouteTransaction("BANK", []byte("payload"), "alice", "bob")
	if err != nil {
		t.Fatalf("route transaction failed: %v", err)
	}
	if tx == nil || tx.Protocol != "BANK" || tx.Status != StatusPending {
		t.Fatalf("unexpected tx state: %+v", tx)
	}

	status := r.GetQueueStatus()
	if status["BANK"] != 1 {
		t.Fatalf("expected 1 bank tx in queue, got %d", status["BANK"])
	}

	if _, err := r.RouteTransaction("UNKNOWN", []byte("x"), "a", "b"); err == nil {
		t.Fatal("expected unsupported protocol error")
	}
}

func TestValidationAndCallbacks(t *testing.T) {
	r := NewProtocolRouter()
	validatedCh := make(chan struct{}, 1)
	rollupCh := make(chan struct{}, 1)

	r.SetCallbacks(
		func(tx *ProtocolTransaction) error {
			select {
			case validatedCh <- struct{}{}:
			default:
			}
			return nil
		},
		nil,
		func(rb *RollupBlock) error {
			select {
			case rollupCh <- struct{}{}:
			default:
			}
			return nil
		},
	)

	_, err := r.RouteTransaction("MESSAGE", []byte("msg"), "alice", "bob")
	if err != nil {
		t.Fatalf("route transaction failed: %v", err)
	}

	waitFor(t, 500*time.Millisecond, func() bool {
		return len(r.GetValidatedTransactions("MESSAGE")) > 0
	})

	select {
	case <-validatedCh:
	default:
		t.Fatal("expected validation callback signal")
	}

	r.processRollups()
	select {
	case <-rollupCh:
	default:
		t.Fatal("expected rollup callback to be called")
	}

	last := r.GetLastRollup()
	if _, ok := last["MESSAGE"]; !ok {
		t.Fatal("expected message last rollup timestamp")
	}
}

func TestGetValidatedTransactionsAndMerkleRoot(t *testing.T) {
	r := NewProtocolRouter()

	_, err := r.RouteTransaction("BANK", []byte("a"), "s1", "r1")
	if err != nil {
		t.Fatalf("route tx failed: %v", err)
	}
	time.Sleep(150 * time.Millisecond)

	validated := r.GetValidatedTransactions("BANK")
	if len(validated) == 0 {
		t.Fatal("expected validated bank tx")
	}

	root := r.calculateMerkleRoot(validated)
	if root == "" {
		t.Fatal("expected non-empty merkle root")
	}

	emptyRoot := r.calculateMerkleRoot(nil)
	if emptyRoot != "" {
		t.Fatalf("expected empty root for empty tx set, got %q", emptyRoot)
	}
}

func TestFailurePathViaValidationCallback(t *testing.T) {
	r := NewProtocolRouter()
	r.SetCallbacks(
		func(tx *ProtocolTransaction) error { return errForced("forced") },
		nil,
		nil,
	)

	_, err := r.RouteTransaction("BANK", []byte("x"), "alice", "bob")
	if err != nil {
		t.Fatalf("route tx failed: %v", err)
	}

	waitFor(t, 500*time.Millisecond, func() bool {
		return len(r.GetValidatedTransactions("BANK")) == 0 && r.GetQueueStatus()["BANK"] == 1
	})

	validated := r.GetValidatedTransactions("BANK")
	if len(validated) != 0 {
		t.Fatalf("expected failed tx to be excluded from validated set, got %d", len(validated))
	}
}

func waitFor(t *testing.T, timeout time.Duration, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

type errForced string

func (e errForced) Error() string { return string(e) }

// TestValidationRejectsMalformedTransactions covers the path that previously did
// not exist: validateTransaction slept 100ms and then marked every transaction
// StatusValidated without inspecting it.
func TestValidationRejectsMalformedTransactions(t *testing.T) {
	cases := []struct {
		name string
		tx   *ProtocolTransaction
	}{
		{"nil transaction", nil},
		{"empty payload", &ProtocolTransaction{Protocol: "BANK", Data: nil, Sender: "a", Recipient: "b"}},
		{"missing sender", &ProtocolTransaction{Protocol: "BANK", Data: []byte("x"), Recipient: "b"}},
		{"missing recipient", &ProtocolTransaction{Protocol: "BANK", Data: []byte("x"), Sender: "a"}},
		{"unknown protocol", &ProtocolTransaction{Protocol: "NOPE", Data: []byte("x"), Sender: "a", Recipient: "b"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateProtocolTransaction(tc.tx); err == nil {
				t.Fatalf("expected %s to be rejected", tc.name)
			}
		})
	}

	valid := &ProtocolTransaction{Protocol: "MESSAGE", Data: []byte("x"), Sender: "a", Recipient: "b"}
	if err := validateProtocolTransaction(valid); err != nil {
		t.Fatalf("expected a well-formed transaction to pass, got %v", err)
	}
}

// TestFailedValidationInvokesFailureCallback verifies a rejected transaction is
// reported as failed rather than silently validated.
func TestFailedValidationInvokesFailureCallback(t *testing.T) {
	r := NewProtocolRouter()
	defer r.Stop()

	failed := make(chan string, 1)
	r.SetCallbacks(nil, func(tx *ProtocolTransaction, msg string) error {
		select {
		case failed <- msg:
		default:
		}
		return nil
	}, nil)

	// Routed with an empty payload, which the structural checks reject.
	if _, err := r.RouteTransaction("BANK", nil, "alice", "bob"); err != nil {
		t.Fatalf("route transaction failed: %v", err)
	}

	select {
	case msg := <-failed:
		if msg == "" {
			t.Fatal("expected a failure reason")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("failure callback was never invoked")
	}

	if len(r.GetValidatedTransactions("BANK")) != 0 {
		t.Fatal("a rejected transaction must not appear as validated")
	}
}

// TestStopIsIdempotent guards the rollup goroutine's shutdown path.
func TestStopIsIdempotent(t *testing.T) {
	r := NewProtocolRouter()
	r.Stop()
	r.Stop() // must not panic on a double close
}

// TestGeneratedTransactionIDsAreUnique guards against the ID collision that made
// two identical transfers indistinguishable.
func TestGeneratedTransactionIDsAreUnique(t *testing.T) {
	seen := make(map[string]struct{})
	for i := 0; i < 1000; i++ {
		id := generateTransactionID([]byte("same"), "alice", "bob")
		if _, dup := seen[id]; dup {
			t.Fatal("generateTransactionID produced a duplicate for identical inputs")
		}
		seen[id] = struct{}{}
	}
}
