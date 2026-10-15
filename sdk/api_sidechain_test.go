// Package sdk is a software development kit for building blockchain applications.
// File sdk/api_sidechain_test.go - Tests for the application-facing endpoints.
package sdk

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newApplicationAPI serves a fresh chain's API on an in-process listener, with
// the same middleware a running node installs.
func newApplicationAPI(t *testing.T) (*Blockchain, *httptest.Server) {
	t.Helper()
	t.Setenv(envBlockchainAPIKey, apiKey)
	// A fresh seed per test server: nothing about a test node's identity is fixed in source.
	seed := make([]byte, 32)
	if _, err := rand.Read(seed); err != nil {
		t.Fatal(err)
	}
	t.Setenv(envServerSeed, hex.EncodeToString(seed))

	bc := forkTestChain(t, 2, uint32(genesisDifficulty))
	api, err := NewAPIWithError(bc)
	if err != nil {
		t.Fatalf("NewAPIWithError: %v", err)
	}
	server := httptest.NewServer(api.Handler())
	t.Cleanup(server.Close)
	return bc, server
}

// appRequest sends an authenticated request and returns the status and body.
func appRequest(t *testing.T, method, url string, body []byte) (int, []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(context.Background(), method, url, bytes.NewReader(body))
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, url, err)
	}
	defer func() { _ = resp.Body.Close() }()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	return resp.StatusCode, data
}

// signedTransfer builds a signed BANK transfer from a sender the chain has
// funded (or not).
func signedTransfer(t *testing.T, bc *Blockchain, fund bool) *Bank {
	t.Helper()
	from := newTestWallet(t, "app-sender", 0)
	to := newTestWallet(t, "app-recipient", 0)
	if fund {
		fundWalletForTest(t, bc, from, 50)
	} else if err := from.SetData("balance", 50.0); err != nil {
		// Only the wallet's own advisory figure, so the transfer can be built
		// but the chain will not let it be spent.
		t.Fatalf("set balance: %v", err)
	}

	tx, err := NewBankTransaction(from, to, 1)
	if err != nil {
		t.Fatalf("NewBankTransaction: %v", err)
	}
	signForTest(t, tx)
	tx.Signature, err = tx.Sign([]byte(from.PrivatePEM()))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	return tx
}

// forgeSender builds a transfer that names address as its sender but carries,
// and is signed by, a different key.
func forgeSender(t *testing.T, address string) []byte {
	t.Helper()
	forger := newTestWallet(t, "forger", 0)
	if err := forger.SetData("balance", 50.0); err != nil {
		t.Fatalf("set balance: %v", err)
	}
	tx, err := NewBankTransaction(forger, newTestWallet(t, "payee", 0), 10)
	if err != nil {
		t.Fatalf("NewBankTransaction: %v", err)
	}
	tx.From = walletFromWire(address, forger.PublicPEM())
	tx.Signature, err = tx.Sign([]byte(forger.PrivatePEM()))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	body, err := EncodeTransaction(tx)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	return body
}

func decodeSubmission(t *testing.T, body []byte) TransactionSubmission {
	t.Helper()
	var out TransactionSubmission
	if err := json.Unmarshal(body, &out); err != nil {
		t.Fatalf("decode submission %s: %v", body, err)
	}
	return out
}

func TestSubmitTransactionQueuesAndIsIdempotent(t *testing.T) {
	bc, server := newApplicationAPI(t)
	tx := signedTransfer(t, bc, true)
	body, err := EncodeTransaction(tx)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	status, reply := appRequest(t, http.MethodPost, server.URL+"/v1/transactions", body)
	if status != http.StatusAccepted {
		t.Fatalf("status %d, want 202: %s", status, reply)
	}
	first := decodeSubmission(t, reply)
	if first.ID != tx.GetID() || first.Status != SubmissionPending || first.Known {
		t.Fatalf("unexpected reply %+v", first)
	}
	if !bc.HasTransactionID(tx.GetID()) {
		t.Fatal("the transaction was acknowledged but is not in the mempool")
	}

	// A retry after a lost reply must be a success, not a failure.
	status, reply = appRequest(t, http.MethodPost, server.URL+"/v1/transactions", body)
	if status != http.StatusOK {
		t.Fatalf("resubmission status %d, want 200: %s", status, reply)
	}
	second := decodeSubmission(t, reply)
	if second.ID != tx.GetID() || !second.Known || second.Status != SubmissionPending {
		t.Fatalf("unexpected resubmission reply %+v", second)
	}
	if got := len(bc.GetPendingTransactions()); got != 1 {
		t.Fatalf("mempool holds %d transactions, want 1", got)
	}
}

func TestSubmitTransactionRejections(t *testing.T) {
	bc, server := newApplicationAPI(t)

	tampered := signedTransfer(t, bc, true)
	tampered.Amount = 40 // after signing
	tamperedBody, _ := EncodeTransaction(tampered)

	unfunded := signedTransfer(t, bc, false)
	unfundedBody, _ := EncodeTransaction(unfunded)

	coinbase := []byte(`{"protocol":"COINBASE","from":"a","to":"b","version":1}`)

	// Someone else's funded address named as the sender, with the forger's own
	// key attached and used to sign: the signature verifies, and must not count.
	victim := newTestWallet(t, "victim", 0)
	fundWalletForTest(t, bc, victim, 50)
	forgedBody := forgeSender(t, victim.GetAddress())

	cases := []struct {
		name       string
		body       []byte
		wantStatus int
		wantReason string
	}{
		{"coinbase", coinbase, http.StatusUnprocessableEntity, "coinbase"},
		{"signature over different contents", tamperedBody, http.StatusUnprocessableEntity, "signature"},
		{"unaffordable", unfundedBody, http.StatusUnprocessableEntity, "insufficient"},
		{"sender address that is not the signer's", forgedBody, http.StatusUnprocessableEntity,
			"does not belong"},
		{"not a transaction", []byte(`{"protocol":"NOPE"}`), http.StatusBadRequest, ""},
		{"not json", []byte(`garbage`), http.StatusBadRequest, ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, reply := appRequest(t, http.MethodPost, server.URL+"/v1/transactions", tc.body)
			if status != tc.wantStatus {
				t.Fatalf("status %d, want %d: %s", status, tc.wantStatus, reply)
			}
			if tc.wantReason == "" {
				return
			}
			got := decodeSubmission(t, reply)
			if got.Status != SubmissionRejected ||
				!strings.Contains(strings.ToLower(got.Reason), tc.wantReason) {
				t.Fatalf("unexpected rejection %+v, want a reason mentioning %q", got, tc.wantReason)
			}
		})
	}

	if n := len(bc.GetPendingTransactions()); n != 0 {
		t.Fatalf("a rejected transaction reached the mempool (%d queued)", n)
	}
}

// The public endpoint and the peer endpoint must refuse the same transaction
// for the same reason. If they drifted, the looser one would be the way round
// the other.
func TestSubmitAndConsensusShareOneVerification(t *testing.T) {
	bc, server := newApplicationAPI(t)

	tampered := signedTransfer(t, bc, true)
	tampered.Amount = 40
	tamperedBody, _ := EncodeTransaction(tampered)

	victim := newTestWallet(t, "victim", 0)
	fundWalletForTest(t, bc, victim, 50)

	for name, body := range map[string][]byte{
		"tampered":      tamperedBody,
		"forged sender": forgeSender(t, victim.GetAddress()),
	} {
		t.Run(name, func(t *testing.T) {
			status, reply := appRequest(t, http.MethodPost, server.URL+"/v1/transactions", body)
			if status != http.StatusUnprocessableEntity {
				t.Fatalf("public status %d: %s", status, reply)
			}
			public := decodeSubmission(t, reply)

			status, reply = appRequest(t, http.MethodPost, server.URL+"/v1/consensus/tx", body)
			if status != http.StatusUnprocessableEntity {
				t.Fatalf("consensus status %d: %s", status, reply)
			}
			var peer consensusResult
			if err := json.Unmarshal(reply, &peer); err != nil {
				t.Fatalf("decode: %v", err)
			}

			if public.Reason == "" || public.Reason != peer.Reason {
				t.Fatalf("the two endpoints disagree: public %q, consensus %q", public.Reason, peer.Reason)
			}
		})
	}
}

func TestApplicationEndpointsRequireAnAPIKey(t *testing.T) {
	_, server := newApplicationAPI(t)

	for _, path := range []string{
		"/v1/publishers/1",
		"/v1/publishers?key=" + strings.Repeat("ab", 32),
		"/v1/sidechains/1/1/anchors",
	} {
		resp, err := httpGetCtx(server.URL + path)
		if err != nil {
			t.Fatalf("GET %s: %v", path, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("GET %s without a key: status %d, want 401", path, resp.StatusCode)
		}
	}

	resp, err := httpPostCtx(server.URL+"/v1/transactions", "application/json", strings.NewReader("{}"))
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("POST /v1/transactions without a key: status %d, want 401", resp.StatusCode)
	}
}

func TestPublisherEndpoints(t *testing.T) {
	bc, server := newApplicationAPI(t)
	registry := bc.UTXOSet().Anchors().Registry()

	identity, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("NewPeerIdentity: %v", err)
	}
	publisherID, err := registry.RegisterPublisher(identity.PublicPEM, "Example Studio", MinPublisherBondUnits, 1)
	if err != nil {
		t.Fatalf("RegisterPublisher: %v", err)
	}
	for _, name := range []string{"First", "Second", "First"} {
		if _, err := registry.RegisterGame(publisherID, name, 2); err != nil {
			t.Fatalf("RegisterGame: %v", err)
		}
	}

	check := func(t *testing.T, body []byte) {
		t.Helper()
		var info PublisherInfo
		if err := json.Unmarshal(body, &info); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if info.PublisherID != publisherID || info.Name != "Example Studio" ||
			info.PublicKeyPEM != identity.PublicPEM || info.KeyFingerprint != identity.NodeID {
			t.Fatalf("unexpected publisher %+v", info)
		}
		if len(info.Games) != 3 || info.Games[0].GameID != 1 || info.Games[2].GameID != 3 {
			t.Fatalf("games should be listed in id order: %+v", info.Games)
		}
		if named := info.GamesNamed("First"); len(named) != 2 {
			t.Fatalf("GamesNamed(First) = %+v, want both", named)
		}
	}

	t.Run("by id", func(t *testing.T) {
		status, body := appRequest(t, http.MethodGet, server.URL+"/v1/publishers/1", nil)
		if status != http.StatusOK {
			t.Fatalf("status %d: %s", status, body)
		}
		check(t, body)
	})

	t.Run("by key fingerprint", func(t *testing.T) {
		// Case is not significant in hex.
		url := server.URL + "/v1/publishers?key=" + strings.ToUpper(identity.NodeID)
		status, body := appRequest(t, http.MethodGet, url, nil)
		if status != http.StatusOK {
			t.Fatalf("status %d: %s", status, body)
		}
		check(t, body)
	})

	cases := []struct {
		name string
		path string
		want int
	}{
		{"unregistered id", "/v1/publishers/2", http.StatusNotFound},
		{"zero id", "/v1/publishers/0", http.StatusBadRequest},
		{"non-numeric id", "/v1/publishers/acme", http.StatusBadRequest},
		{"no key", "/v1/publishers", http.StatusBadRequest},
		{"key that is not a fingerprint", "/v1/publishers?key=abc", http.StatusBadRequest},
		{"unknown key", "/v1/publishers?key=" + strings.Repeat("0", 64), http.StatusNotFound},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			status, body := appRequest(t, http.MethodGet, server.URL+tc.path, nil)
			if status != tc.want {
				t.Fatalf("status %d, want %d: %s", status, tc.want, body)
			}
		})
	}
}

func TestSidechainAnchorsEndpoint(t *testing.T) {
	bc, server := newApplicationAPI(t)
	ledger := bc.UTXOSet().Anchors()
	identity, id := registerPublisher(t, ledger)
	url := server.URL + "/v1/sidechains/1/1/anchors"

	status, body := appRequest(t, http.MethodGet, url, nil)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	var list AnchorList
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if list.Total != 0 || len(list.Anchors) != 0 || list.Anchors == nil {
		t.Fatalf("a registered sidechain with nothing anchored should list [] , got %s", body)
	}

	// Two anchors, the second carried by a real transaction in main block 2.
	chain := buildChain(t, id, 4)
	first := anchorOver(t, chain, 0, 1)
	second := anchorOver(t, chain, 2, 3)
	var carrierID string
	for i, anchor := range []SidechainAnchor{first, second} {
		signature, err := SignAnchor(identity, anchor)
		if err != nil {
			t.Fatalf("SignAnchor: %v", err)
		}
		if err := ledger.Record(anchor, signature, int64(i+1)); err != nil {
			t.Fatalf("Record: %v", err)
		}
		if i == 1 {
			carrier, err := NewAnchorTransaction(newTestWallet(t, "anchorer", 0), anchor, signature)
			if err != nil {
				t.Fatalf("NewAnchorTransaction: %v", err)
			}
			mainBlock := bc.GetBlockByIndex(2)
			mainBlock.Transactions = append(mainBlock.Transactions, carrier)
			carrierID = carrier.GetID()
		}
	}

	status, body = appRequest(t, http.MethodGet, url+"?page=2&limit=1", nil)
	if status != http.StatusOK {
		t.Fatalf("status %d: %s", status, body)
	}
	list = AnchorList{}
	if err := json.Unmarshal(body, &list); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if list.Total != 2 || len(list.Anchors) != 1 {
		t.Fatalf("page 2 of 1 should hold the second of two anchors: %s", body)
	}
	got := list.Anchors[0]
	if got.FromHeight != 2 || got.ToHeight != 3 || got.MainBlockIndex != 2 ||
		got.MainBlockHash != bc.GetBlockByIndex(2).Hash ||
		got.TipHash != hex.EncodeToString(second.TipHash) || got.TransactionID != carrierID {
		t.Fatalf("unexpected anchor %+v (want transaction %s)", got, carrierID)
	}

	// What the API reports is enough to verify a record against.
	reported, err := got.Anchor()
	if err != nil {
		t.Fatalf("Anchor(): %v", err)
	}
	proof, err := ProveRecord(chain, second, 3, 0)
	if err != nil {
		t.Fatalf("ProveRecord: %v", err)
	}
	block, _ := chain.BlockAt(3)
	if err := VerifyRecord(block.Payloads[0], proof, reported); err != nil {
		t.Fatalf("a record did not verify against the reported anchor: %v", err)
	}

	for _, tc := range []struct {
		path string
		want int
	}{
		{"/v1/sidechains/1/2/anchors", http.StatusNotFound},
		{"/v1/sidechains/2/1/anchors", http.StatusNotFound},
		{"/v1/sidechains/0/1/anchors", http.StatusBadRequest},
		{"/v1/sidechains/1/x/anchors", http.StatusBadRequest},
	} {
		if status, body := appRequest(t, http.MethodGet, server.URL+tc.path, nil); status != tc.want {
			t.Fatalf("GET %s: status %d, want %d: %s", tc.path, status, tc.want, body)
		}
	}
}

func TestPublisherKeyFingerprintIsCanonical(t *testing.T) {
	registry := NewPublisherRegistry()
	identity, err := NewPeerIdentity()
	if err != nil {
		t.Fatalf("NewPeerIdentity: %v", err)
	}

	fingerprint, err := PublisherKeyFingerprint(identity.PublicPEM)
	if err != nil {
		t.Fatalf("PublisherKeyFingerprint: %v", err)
	}
	if fingerprint != identity.NodeID {
		t.Fatal("the fingerprint should equal the identity's NodeID")
	}

	// The same key in a different PEM text is the same key.
	crlf := strings.ReplaceAll(identity.PublicPEM, "\n", "\r\n")
	if again, err := PublisherKeyFingerprint(crlf); err != nil || again != fingerprint {
		t.Fatalf("CRLF changed the fingerprint: %q, %v", again, err)
	}

	publisherID, err := registry.RegisterPublisher(identity.PublicPEM, "one", MinPublisherBondUnits, 0)
	if err != nil {
		t.Fatalf("RegisterPublisher: %v", err)
	}
	if _, err := registry.RegisterPublisher(crlf, "two", MinPublisherBondUnits, 0); !errors.Is(err, ErrKeyAlreadyRegistered) {
		t.Fatalf("a second encoding of one key should not hold a second identity, got %v", err)
	}

	found, ok := registry.PublisherByKeyFingerprint(fingerprint)
	if !ok || found.PublisherID != publisherID {
		t.Fatalf("lookup by fingerprint found %+v (%v)", found, ok)
	}

	// Rolling the registration back frees the key under both indexes.
	if err := registry.unregisterPublisher(publisherID); err != nil {
		t.Fatalf("unregister: %v", err)
	}
	if _, ok := registry.PublisherByKeyFingerprint(fingerprint); ok {
		t.Fatal("a rolled-back publisher is still found by fingerprint")
	}
	if _, err := registry.RegisterPublisher(crlf, "two", MinPublisherBondUnits, 0); err != nil {
		t.Fatalf("the key should be free again after the rollback: %v", err)
	}
}

func TestAnchorLedgerRecordsWhereEachAnchorLanded(t *testing.T) {
	f := newAnchorFixture(t, 2)

	first := f.signedAnchorTx(t)
	blockOne := utxoBlock(t, 1, "genesis", first)
	if _, err := f.set.ApplyBlock(blockOne, utxoTestSplit()); err != nil {
		t.Fatalf("apply: %v", err)
	}
	if err := f.chain.MarkAnchored(first.Anchor.ToHeight); err != nil {
		t.Fatalf("MarkAnchored: %v", err)
	}
	block, err := NewSidechainBlock(f.id, f.chain.Tip(), [][]byte{[]byte("more")}, 9)
	if err != nil {
		t.Fatalf("NewSidechainBlock: %v", err)
	}
	if err := f.chain.Append(block); err != nil {
		t.Fatalf("Append: %v", err)
	}
	second := f.signedAnchorTx(t)
	blockTwo := utxoBlock(t, 2, blockOne.Hash, second)
	if _, err := f.set.ApplyBlock(blockTwo, utxoTestSplit()); err != nil {
		t.Fatalf("apply: %v", err)
	}

	records := f.set.Anchors().Anchors(f.id)
	if len(records) != 2 || records[0].MainHeight != 1 || records[1].MainHeight != 2 {
		t.Fatalf("unexpected records %+v", records)
	}

	if err := f.set.RevertBlock(blockTwo.Hash); err != nil {
		t.Fatalf("revert: %v", err)
	}
	records = f.set.Anchors().Anchors(f.id)
	if len(records) != 1 || records[0].MainHeight != 1 {
		t.Fatalf("after the revert: %+v", records)
	}
}
