package sdk

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"testing"
)

// TestTransactionCodecRoundTripsEveryProtocol guards the encoding that makes
// block persistence possible. Message and Persist previously had no MarshalJSON
// at all, so Go promoted Tx.MarshalJSON and their payloads were silently dropped;
// Bank and Coinbase base64-encoded an already-JSON document into "data"; and
// nothing could decode any of it back into the Transaction interface.
func TestTransactionCodecRoundTripsEveryProtocol(t *testing.T) {
	from := newTestWallet(t, "codec-from", 10000)
	to := newTestWallet(t, "codec-to", 0)

	bank, err := NewBankTransaction(from, to, 987.65)
	if err != nil {
		t.Fatalf("bank: %v", err)
	}
	message, err := NewMessageTransaction(from, to, "a message that must survive")
	if err != nil {
		t.Fatalf("message: %v", err)
	}
	persist, err := NewPersistTransaction(from, to, transactionFee, map[string]string{"alpha": "1", "beta": "2"})
	if err != nil {
		t.Fatalf("persist: %v", err)
	}
	coinbase, err := NewCoinbaseTransaction(from, to, NewConfig())
	if err != nil {
		t.Fatalf("coinbase: %v", err)
	}

	cases := []struct {
		name   string
		tx     Transaction
		verify func(t *testing.T, decoded Transaction)
	}{
		{"bank", bank, func(t *testing.T, decoded Transaction) {
			got, ok := decoded.(*Bank)
			if !ok {
				t.Fatalf("expected *Bank, got %T", decoded)
			}
			if got.Amount != 987.65 {
				t.Fatalf("amount lost: %v", got.Amount)
			}
		}},
		{"message", message, func(t *testing.T, decoded Transaction) {
			got, ok := decoded.(*Message)
			if !ok {
				t.Fatalf("expected *Message, got %T", decoded)
			}
			if got.Message != "a message that must survive" {
				t.Fatalf("message lost: %q", got.Message)
			}
		}},
		{"persist", persist, func(t *testing.T, decoded Transaction) {
			got, ok := decoded.(*Persist)
			if !ok {
				t.Fatalf("expected *Persist, got %T", decoded)
			}
			if got.Data["alpha"] != "1" || got.Data["beta"] != "2" {
				t.Fatalf("persist payload lost: %v", got.Data)
			}
		}},
		{"coinbase", coinbase, func(t *testing.T, decoded Transaction) {
			got, ok := decoded.(*Coinbase)
			if !ok {
				t.Fatalf("expected *Coinbase, got %T", decoded)
			}
			if got.TokenCount != NewConfig().TokenCount {
				t.Fatalf("token count lost: %v", got.TokenCount)
			}
		}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Sign first: the signature must survive too.
			signature, err := tc.tx.Sign([]byte(from.PrivatePEM()))
			if err != nil {
				t.Fatalf("sign: %v", err)
			}
			setTransactionSignature(tc.tx, signature)

			encoded, err := json.Marshal(tc.tx)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}

			decoded, err := DecodeTransaction(encoded)
			if err != nil {
				t.Fatalf("decode: %v", err)
			}

			tc.verify(t, decoded)

			if decoded.GetID() != tc.tx.GetID() {
				t.Fatalf("ID changed: %s -> %s", tc.tx.GetID(), decoded.GetID())
			}
			if decoded.GetProtocol() != tc.tx.GetProtocol() {
				t.Fatalf("protocol changed: %s -> %s", tc.tx.GetProtocol(), decoded.GetProtocol())
			}
			if decoded.GetFee() != tc.tx.GetFee() {
				t.Fatalf("fee changed: %v -> %v", tc.tx.GetFee(), decoded.GetFee())
			}
			if decoded.GetSenderWallet().GetAddress() != from.GetAddress() {
				t.Fatal("sender address lost")
			}
			if decoded.GetRecipientWallet().GetAddress() != to.GetAddress() {
				t.Fatal("recipient address lost")
			}

			// The decoded transaction must be independently verifiable, which
			// requires the sender's public key to travel with it.
			ok, err := decoded.Verify([]byte(decoded.GetSenderWallet().PublicPEM()), decoded.GetSignature())
			if err != nil || !ok {
				t.Fatalf("decoded transaction must verify: ok=%v err=%v", ok, err)
			}
		})
	}
}

// TestDecodeTransactionRejectsBadInput covers the failure paths.
func TestDecodeTransactionRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		in   string
	}{
		{"not json", `not json at all`},
		{"empty object", `{}`},
		{"missing protocol", `{"id":"1:1:1:1","version":1}`},
		{"blank protocol", `{"protocol":"   "}`},
		{"unknown protocol", `{"protocol":"DOGECOIN"}`},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := DecodeTransaction([]byte(tc.in)); err == nil {
				t.Fatalf("expected %s to be rejected", tc.name)
			}
		})
	}

	if _, err := EncodeTransaction(nil); err == nil {
		t.Fatal("expected an error when encoding a nil transaction")
	}
}

// TestDecodeTransactionAcceptsControlProtocols covers CHAIN and P2P, which decode
// to the base transaction.
func TestDecodeTransactionAcceptsControlProtocols(t *testing.T) {
	for _, protocol := range []string{ChainProtocolID, P2PProtocolID} {
		payload := `{"protocol":"` + protocol + `","version":1,"id":"1:2:3:4"}`
		tx, err := DecodeTransaction([]byte(payload))
		if err != nil {
			t.Fatalf("decode %s: %v", protocol, err)
		}
		if tx.GetProtocol() != protocol {
			t.Fatalf("expected %s, got %s", protocol, tx.GetProtocol())
		}
	}
}

// TestBlockPersistenceRoundTrip is the end-to-end guard for the defect that made
// the chain non-persistent: LoadExistingBlocks could not decode any block with a
// transaction, logged the error, skipped the block, and so silently discarded the
// entire history on every restart.
func TestBlockPersistenceRoundTrip(t *testing.T) {
	dir := t.TempDir()
	if err := NewLocalStorage(dir); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	t.Cleanup(func() { _ = NewLocalStorage("./test_data") })

	from := newTestWallet(t, "persist-from", 10000)
	to := newTestWallet(t, "persist-to", 0)

	cfg := NewConfig()
	cfg.DataPath = dir
	bc := &Blockchain{cfg: cfg, TXLookup: NewTXLookupManager(), Blocks: []*Block{}}

	// Build a small chain by hand so the linkage is explicit.
	previousHash := ""
	for i := 0; i < 4; i++ {
		bank, err := NewBankTransaction(from, to, float64(i+1))
		if err != nil {
			t.Fatalf("bank tx %d: %v", i, err)
		}
		bank.Signature, err = bank.Sign([]byte(from.PrivatePEM()))
		if err != nil {
			t.Fatalf("sign %d: %v", i, err)
		}

		block := NewBlock([]Transaction{bank}, previousHash)
		block.Index = *big.NewInt(int64(i))
		block.Hash = block.CalculateHash()
		if err := block.save(); err != nil {
			t.Fatalf("save block %d: %v", i, err)
		}

		bc.Blocks = append(bc.Blocks, block)
		previousHash = block.Hash
	}

	// Reload into a fresh chain.
	reloaded := &Blockchain{cfg: cfg, TXLookup: NewTXLookupManager(), Blocks: []*Block{}}
	if err := reloaded.LoadExistingBlocks(); err != nil {
		t.Fatalf("load blocks: %v", err)
	}

	if len(reloaded.Blocks) != 4 {
		t.Fatalf("expected 4 blocks, loaded %d", len(reloaded.Blocks))
	}

	for i, block := range reloaded.Blocks {
		if got := block.Index.Int64(); got != int64(i) {
			t.Fatalf("block %d loaded out of order (index %d)", i, got)
		}
		if len(block.Transactions) != 1 {
			t.Fatalf("block %d lost its transactions", i)
		}
		bank, ok := block.Transactions[0].(*Bank)
		if !ok {
			t.Fatalf("block %d transaction decoded as %T", i, block.Transactions[0])
		}
		if bank.Amount != float64(i+1) {
			t.Fatalf("block %d amount changed: %v", i, bank.Amount)
		}
		// The hash must still match after a disk round trip.
		if block.Hash != block.CalculateHash() {
			t.Fatalf("block %d hash is unstable across persistence", i)
		}
	}

	if reloaded.NextBlockIndex != 4 {
		t.Fatalf("expected next index 4, got %d", reloaded.NextBlockIndex)
	}
}

// TestLoadExistingBlocksOrdersNumerically guards sort.Strings, under which
// "10.json" sorted before "2.json" so bc.Blocks[i] was not block i and the
// previousHash chain was scrambled.
func TestLoadExistingBlocksOrdersNumerically(t *testing.T) {
	dir := t.TempDir()
	if err := NewLocalStorage(dir); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	t.Cleanup(func() { _ = NewLocalStorage("./test_data") })

	cfg := NewConfig()
	cfg.DataPath = dir

	// 12 blocks: enough that lexical and numeric order differ (2 vs 10, 11).
	previousHash := ""
	for i := 0; i < 12; i++ {
		block := NewBlock(nil, previousHash)
		block.Index = *big.NewInt(int64(i))
		block.Hash = block.CalculateHash()
		if err := block.save(); err != nil {
			t.Fatalf("save block %d: %v", i, err)
		}
		previousHash = block.Hash
	}

	bc := &Blockchain{cfg: cfg, TXLookup: NewTXLookupManager(), Blocks: []*Block{}}
	if err := bc.LoadExistingBlocks(); err != nil {
		t.Fatalf("load blocks: %v", err)
	}

	if len(bc.Blocks) != 12 {
		t.Fatalf("expected 12 blocks, got %d", len(bc.Blocks))
	}
	for i, block := range bc.Blocks {
		if got := block.Index.Int64(); got != int64(i) {
			t.Fatalf("position %d holds block %d -- blocks were not sorted numerically", i, got)
		}
	}
}

// TestLoadExistingBlocksDetectsABrokenChain guards the silent skip: a corrupt or
// missing block used to be logged and stepped over, leaving a chain with a hole.
func TestLoadExistingBlocksDetectsABrokenChain(t *testing.T) {
	dir := t.TempDir()
	if err := NewLocalStorage(dir); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	t.Cleanup(func() { _ = NewLocalStorage("./test_data") })

	cfg := NewConfig()
	cfg.DataPath = dir

	previousHash := ""
	for i := 0; i < 3; i++ {
		block := NewBlock(nil, previousHash)
		block.Index = *big.NewInt(int64(i))
		block.Hash = block.CalculateHash()
		if err := block.save(); err != nil {
			t.Fatalf("save block %d: %v", i, err)
		}
		previousHash = block.Hash
	}

	// Break the link on block 1.
	tampered := NewBlock(nil, "a-hash-that-links-to-nothing")
	tampered.Index = *big.NewInt(1)
	tampered.Hash = tampered.CalculateHash()
	encoded, err := json.Marshal(tampered)
	if err != nil {
		t.Fatalf("marshal tampered block: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "blocks", "1.json"), encoded, 0644); err != nil {
		t.Fatalf("write tampered block: %v", err)
	}

	bc := &Blockchain{cfg: cfg, TXLookup: NewTXLookupManager(), Blocks: []*Block{}}
	if err := bc.LoadExistingBlocks(); err == nil {
		t.Fatal("expected a broken hash chain to be reported at load time")
	}

	// And an unparseable block must be reported, not skipped.
	if err := os.WriteFile(filepath.Join(dir, "blocks", "1.json"), []byte("{not json"), 0644); err != nil {
		t.Fatalf("write corrupt block: %v", err)
	}
	bc = &Blockchain{cfg: cfg, TXLookup: NewTXLookupManager(), Blocks: []*Block{}}
	if err := bc.LoadExistingBlocks(); err == nil {
		t.Fatal("expected a corrupt block file to be reported at load time")
	}
}
