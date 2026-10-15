// File sidechain/review_test.go - Backlogs, malformed anchors, escaping and registration limits.
package sidechain

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestLongBacklogsAnchorInSpans: more than MaxAnchorSpan pending blocks are committed in
// several contiguous anchors instead of none at all.
func TestLongBacklogsAnchorInSpans(t *testing.T) {
	id, _ := NewID(1, 1)
	chain := buildChain(t, id, int(MaxAnchorSpan)+3)
	first, err := BuildAnchor(chain)
	if err != nil || first == nil {
		t.Fatalf("BuildAnchor over a long backlog: %v", err)
	}
	if first.FromHeight != 0 || first.ToHeight != MaxAnchorSpan-1 {
		t.Fatalf("first anchor %d..%d, want 0..%d", first.FromHeight, first.ToHeight, MaxAnchorSpan-1)
	}
	if err := VerifyAgainstChain(*first, chain); err != nil {
		t.Fatal(err)
	}
	if err := chain.MarkAnchored(first.ToHeight); err != nil {
		t.Fatal(err)
	}
	second, err := BuildAnchor(chain)
	if err != nil || second == nil {
		t.Fatalf("second anchor: %v", err)
	}
	if second.FromHeight != MaxAnchorSpan || second.ToHeight != MaxAnchorSpan+2 {
		t.Fatalf("second anchor %d..%d", second.FromHeight, second.ToHeight)
	}
}

// TestVerifyAgainstChainRefusesMalformedAnchors: a backwards range whose tip hash matches is an
// error, not a panic.
func TestVerifyAgainstChainRefusesMalformedAnchors(t *testing.T) {
	id, _ := NewID(1, 1)
	chain := buildChain(t, id, 4)
	tip, _ := chain.BlockAt(1)
	bad := Anchor{PublisherID: 1, GameID: 1, FromHeight: 3, ToHeight: 1, TipHash: tip.Hash, HeaderRoot: make([]byte, 32)}
	if err := VerifyAgainstChain(bad, chain); !errors.Is(err, ErrInvalidAnchor) {
		t.Fatalf("a backwards anchor: %v", err)
	}
}

// TestClientEscapesPathSegmentsOnce: an id with characters a path must escape reaches the node
// escaped exactly once, with the base URL's own prefix kept.
func TestClientEscapesPathSegmentsOnce(t *testing.T) {
	var got string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		got = r.URL.EscapedPath() + " " + r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id": "x", "status": "pending"}`))
	}))
	defer srv.Close()
	c, err := NewClient(srv.URL+"/node", "key")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := c.Transaction(context.Background(), "a/b c:1"); err != nil {
		t.Fatal(err)
	}
	if want := "/node/v1/transactions/a%2Fb%20c:1 /node/v1/transactions/a/b c:1"; got != want {
		t.Fatalf("request path %q, want %q", got, want)
	}
}

// TestRegistrationsAreCheckedBeforeSigning: names and bonds the node would refuse are refused
// here, with ErrInvalidTransaction.
func TestRegistrationsAreCheckedBeforeSigning(t *testing.T) {
	k, _ := GenerateKey()
	long := strings.Repeat("x", MaxPublisherNameLength+1)
	for name, err := range map[string]error{
		"empty publisher name": func() error {
			_, err := NewPublisherRegistration(k, k, "", MinPublisherBondUnits, TxOptions{})
			return err
		}(),
		"long publisher name": func() error {
			_, err := NewPublisherRegistration(k, k, long, MinPublisherBondUnits, TxOptions{})
			return err
		}(),
		"small bond": func() error {
			_, err := NewPublisherRegistration(k, k, "Studio", MinPublisherBondUnits-1, TxOptions{})
			return err
		}(),
		"empty game name": func() error {
			_, err := NewGameRegistration(k, k, 1, "", TxOptions{})
			return err
		}(),
		"long game name": func() error {
			_, err := NewGameRegistration(k, k, 1, long, TxOptions{})
			return err
		}(),
	} {
		if !errors.Is(err, ErrInvalidTransaction) {
			t.Errorf("%s: %v", name, err)
		}
	}
	if _, err := NewPublisherRegistration(k, k, strings.Repeat("x", MaxPublisherNameLength), MinPublisherBondUnits, TxOptions{}); err != nil {
		t.Errorf("a name at the limit and the minimum bond are fine: %v", err)
	}
}
