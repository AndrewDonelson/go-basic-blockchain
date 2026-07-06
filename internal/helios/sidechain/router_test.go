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
