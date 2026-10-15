// Package sdk is a software development kit for building blockchain applications.
// File sdk/sidechain_lite_test.go - The dependency-free sidechain package builds transactions the node accepts.
package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/AndrewDonelson/go-basic-blockchain/sidechain"
)

func liteKeys(t *testing.T) (wallet, identity *sidechain.Key) {
	t.Helper()
	w, err := sidechain.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	id, err := sidechain.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	return w, id
}

// decodeLite encodes a lite transaction and decodes it the way the node does.
func decodeLite(t *testing.T, tx *sidechain.Transaction) Transaction {
	t.Helper()
	raw, err := json.Marshal(tx)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := DecodeTransaction(raw)
	if err != nil {
		t.Fatalf("DecodeTransaction: %v", err)
	}
	if decoded.GetID() != tx.ID() {
		t.Fatalf("id %q decoded as %q", tx.ID(), decoded.GetID())
	}
	if err := VerifyTransactionAuthority(decoded); err != nil {
		t.Fatalf("the node refuses the lite transaction's signature: %v", err)
	}
	liteBytes, err := tx.SigningBytes()
	if err != nil {
		t.Fatal(err)
	}
	nodeBytes, err := decoded.SigningBytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(liteBytes, nodeBytes) {
		t.Fatalf("signing payloads differ:\nlite %s\nnode %s", liteBytes, nodeBytes)
	}
	return decoded
}

// TestLiteTransactionsAreNodeTransactions keeps sidechain.Transaction in step with the node's
// RegistrationTx and AnchorTx: same wire form, same signing payload, accepted signatures.
func TestLiteTransactionsAreNodeTransactions(t *testing.T) {
	wallet, identity := liteKeys(t)
	registry := NewPublisherRegistry()

	pub, err := sidechain.NewPublisherRegistration(wallet, identity, "Lite Studio", MinPublisherBondUnits,
		sidechain.TxOptions{Nonce: 0, Fee: sidechain.DefaultFee})
	if err != nil {
		t.Fatal(err)
	}
	reg, ok := decodeLite(t, pub).(*RegistrationTx)
	if !ok {
		t.Fatal("a lite publisher registration must decode as a RegistrationTx")
	}
	if reg.Kind != RegistrationKindPublisher || reg.Name != "Lite Studio" || reg.BondUnits != MinPublisherBondUnits ||
		reg.PublicKeyPEM != identity.PublicPEM() || reg.Fee != sidechain.DefaultFee {
		t.Fatalf("decoded registration %+v", reg)
	}
	if err := reg.VerifyRegistrationSignature(registry); err != nil {
		t.Fatalf("registration signature: %v", err)
	}
	fingerprint, err := PublisherKeyFingerprint(identity.PublicPEM())
	if err != nil || fingerprint != identity.Address() {
		t.Fatalf("Key.Address must be the node's key fingerprint: %v", err)
	}
	if wallet.Address() != reg.GetSenderWallet().GetAddress() {
		t.Fatal("the wallet address must be the sender")
	}

	publisherID, err := registry.RegisterPublisher(identity.PublicPEM(), "Lite Studio", MinPublisherBondUnits, 1)
	if err != nil {
		t.Fatal(err)
	}
	game, err := sidechain.NewGameRegistration(wallet, identity, publisherID, "Lite Game", sidechain.TxOptions{Nonce: 1})
	if err != nil {
		t.Fatal(err)
	}
	greg, ok := decodeLite(t, game).(*RegistrationTx)
	if !ok || greg.Kind != RegistrationKindGame || greg.PublisherID != publisherID || greg.GetNonce() != 1 {
		t.Fatalf("decoded game registration %+v", greg)
	}
	if err := greg.VerifyRegistrationSignature(registry); err != nil {
		t.Fatalf("game registration signature: %v", err)
	}
	gameID, err := registry.RegisterGame(publisherID, "Lite Game", 2)
	if err != nil {
		t.Fatal(err)
	}

	id, err := sidechain.NewID(publisherID, gameID)
	if err != nil {
		t.Fatal(err)
	}
	chain, err := sidechain.NewChain(id)
	if err != nil {
		t.Fatal(err)
	}
	var prev *sidechain.Block
	for i := 0; i < 3; i++ {
		b, err := sidechain.NewBlock(id, prev, [][]byte{[]byte("record")}, int64(i+1))
		if err != nil {
			t.Fatal(err)
		}
		if err := chain.Append(b); err != nil {
			t.Fatal(err)
		}
		prev = b
	}
	anchor, err := sidechain.BuildAnchor(chain)
	if err != nil || anchor == nil {
		t.Fatalf("BuildAnchor: %v", err)
	}
	sig, err := sidechain.SignAnchor(identity, *anchor)
	if err != nil {
		t.Fatal(err)
	}
	if err := sidechain.VerifyAnchorSignature(identity.PublicPEM(), *anchor, sig); err != nil {
		t.Fatal(err)
	}
	atx, err := sidechain.NewAnchorTransaction(wallet, *anchor, sig, sidechain.TxOptions{Nonce: 2})
	if err != nil {
		t.Fatal(err)
	}
	decoded, ok := decodeLite(t, atx).(*AnchorTx)
	if !ok {
		t.Fatal("a lite anchor must decode as an AnchorTx")
	}
	ledger := NewAnchorLedger(registry)
	if err := ledger.Verify(decoded.Anchor, decoded.AnchorSignature); err != nil {
		t.Fatalf("the main chain refuses the lite anchor: %v", err)
	}

	t.Run("tampering breaks the signature", func(t *testing.T) {
		raw, _ := json.Marshal(atx)
		var m map[string]any
		_ = json.Unmarshal(raw, &m)
		m["to_height"] = float64(anchor.ToHeight + 1)
		raw, _ = json.Marshal(m)
		tx, err := DecodeTransaction(raw)
		if err != nil {
			t.Fatal(err)
		}
		if VerifyTransactionAuthority(tx) == nil {
			t.Fatal("an edited anchor must not verify")
		}
	})
}

// TestLiteClientAgainstTheAPI submits a lite registration through sidechain.Client and reads
// back the account and transaction status endpoints.
func TestLiteClientAgainstTheAPI(t *testing.T) {
	bc, server := newApplicationAPI(t)
	wallet, identity := liteKeys(t)
	creditAddressForTest(t, bc, wallet.Address(), 500)

	c, err := sidechain.NewClient(server.URL, apiKey)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	acct, err := c.Account(ctx, wallet.Address())
	if err != nil {
		t.Fatal(err)
	}
	if acct.BalanceUnits != AmountToUnits(500) || acct.HasNonce || acct.HasPending || acct.NextNonce() != 0 {
		t.Fatalf("fresh account %+v", acct)
	}

	tx, err := sidechain.NewPublisherRegistration(wallet, identity, "Lite Studio", MinPublisherBondUnits,
		sidechain.TxOptions{Nonce: acct.NextNonce(), Fee: sidechain.DefaultFee})
	if err != nil {
		t.Fatal(err)
	}
	reply, err := c.Submit(ctx, tx)
	if err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if reply.Status != SubmissionPending || reply.ID != tx.ID() {
		t.Fatalf("reply %+v", reply)
	}
	again, err := c.Submit(ctx, tx)
	if err != nil || !again.Known {
		t.Fatalf("resubmitting is idempotent: %+v %v", again, err)
	}

	info, err := c.Transaction(ctx, tx.ID())
	if err != nil || info.Status != SubmissionPending {
		t.Fatalf("status %+v %v", info, err)
	}
	acct, err = c.Account(ctx, wallet.Address())
	if err != nil || !acct.HasPending || acct.PendingNonce != 0 || acct.NextNonce() != 1 {
		t.Fatalf("account with a pending transaction %+v %v", acct, err)
	}

	if _, err := c.Transaction(ctx, "0:0:0:1"); !errors.Is(err, sidechain.ErrNotFound) {
		t.Fatalf("unknown id: %v", err)
	}
	if _, err := c.Account(ctx, "nothex"); err == nil {
		t.Fatal("a malformed address is refused")
	}
	wctx, cancel := context.WithTimeout(ctx, 50*time.Millisecond)
	defer cancel()
	if _, err := c.WaitConfirmed(wctx, tx.ID(), 10*time.Millisecond); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("an unmined transaction waits: %v", err)
	}

	status, body := appRequest(t, http.MethodGet, server.URL+"/v1/accounts/"+wallet.Address(), nil)
	if status != http.StatusOK || !bytes.Contains(body, []byte(`"has_pending":true`)) {
		t.Fatalf("%d %s", status, body)
	}
}

// TestGenesisPaysTheConfiguredMinerAddress: a node started with MINER_ADDRESS funds and mines to
// that address instead of replacing it with a generated wallet, so an operator (or an application
// paying for its own transactions) can hold the key.
func TestGenesisPaysTheConfiguredMinerAddress(t *testing.T) {
	key, err := sidechain.GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	cfg := NewConfig()
	cfg.DataPath = t.TempDir()
	cfg.MinerAddress = key.Address()
	if err := NewLocalStorage(cfg.DataPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = NewLocalStorage("./test_data") })
	bc := NewBlockchain(cfg)
	if bc == nil {
		t.Fatal("NewBlockchain failed")
	}
	if cfg.MinerAddress != key.Address() {
		t.Fatalf("miner address replaced with %s", cfg.MinerAddress)
	}
	if bc.GetBalanceUnits(key.Address()) <= 0 {
		t.Fatal("the configured miner address should be funded at genesis")
	}
	if IsAddress("MINER") || !IsAddress(key.Address()) {
		t.Fatal("IsAddress")
	}
}

// TestUnsetMinerAddressKeepsTheGeneratedMinerWallet: with MINER_ADDRESS unset (the "MINER"
// placeholder) a fresh chain generates and funds its own miner wallet, as it always has.
func TestUnsetMinerAddressKeepsTheGeneratedMinerWallet(t *testing.T) {
	cfg := NewConfig()
	cfg.DataPath = t.TempDir()
	if IsAddress(cfg.MinerAddress) {
		t.Skip("MINER_ADDRESS is set in this environment")
	}
	if err := NewLocalStorage(cfg.DataPath); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = NewLocalStorage("./test_data") })
	if bc := NewBlockchain(cfg); bc == nil {
		t.Fatal("NewBlockchain failed")
	} else if !IsAddress(cfg.MinerAddress) || bc.GetBalanceUnits(cfg.MinerAddress) <= 0 {
		t.Fatalf("a generated, funded miner wallet was expected, got %q", cfg.MinerAddress)
	}
}
