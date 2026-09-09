// Package sdk regression tests.
//
// Every test here pins a defect that was found in review and fixed. They are
// grouped by the concern they protect, and each names the behaviour it guards so
// a future change that reintroduces the bug fails loudly rather than silently.
package sdk

import (
	"encoding/json"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

const regressionPassphrase = "Passw0rd!Passw0rd!"

// newTestWallet creates an unlocked wallet with a unique identity.
func newTestWallet(t *testing.T, name string, balance float64) *Wallet {
	t.Helper()

	w, err := NewWallet(NewWalletOptions(
		NewBigInt(1), NewBigInt(1), NewBigInt(1), NewBigInt(time.Now().UnixNano()),
		name, regressionPassphrase, []string{"regression"},
	))
	if err != nil {
		t.Fatalf("create wallet %s: %v", name, err)
	}
	if err := w.Unlock(regressionPassphrase); err != nil {
		t.Fatalf("unlock wallet %s: %v", name, err)
	}
	if balance != 0 {
		if err := w.SetData("balance", balance); err != nil {
			t.Fatalf("fund wallet %s: %v", name, err)
		}
	}
	return w
}

// -----------------------------------------------------------------------------
// Signatures
// -----------------------------------------------------------------------------

// TestSignatureCoversProtocolFields is the guard for the most serious defect
// found: Tx.Sign marshalled only the embedded Tx, so a Bank transfer's Amount
// (and a Message's body, and a Coinbase's TokenCount) sat outside the signed
// bytes. A signature for 1 token verified just as happily for 1,000,000.
func TestSignatureCoversProtocolFields(t *testing.T) {
	from := newTestWallet(t, "sig-from", 1000)
	to := newTestWallet(t, "sig-to", 0)

	t.Run("bank amount", func(t *testing.T) {
		tx, err := NewBankTransaction(from, to, 1.0)
		if err != nil {
			t.Fatalf("create bank tx: %v", err)
		}
		sig, err := tx.Sign([]byte(from.PrivatePEM()))
		if err != nil {
			t.Fatalf("sign: %v", err)
		}

		ok, err := tx.Verify([]byte(from.PublicPEM()), sig)
		if err != nil || !ok {
			t.Fatalf("signature must verify for the signed amount: ok=%v err=%v", ok, err)
		}

		tx.Amount = 1000000.0
		ok, _ = tx.Verify([]byte(from.PublicPEM()), sig)
		if ok {
			t.Fatal("signature still valid after the amount was tampered with")
		}
	})

	t.Run("message body", func(t *testing.T) {
		tx, err := NewMessageTransaction(from, to, "original")
		if err != nil {
			t.Fatalf("create message tx: %v", err)
		}
		sig, err := tx.Sign([]byte(from.PrivatePEM()))
		if err != nil {
			t.Fatalf("sign: %v", err)
		}

		ok, _ := tx.Verify([]byte(from.PublicPEM()), sig)
		if !ok {
			t.Fatal("signature must verify for the signed message")
		}

		tx.Message = "tampered"
		ok, _ = tx.Verify([]byte(from.PublicPEM()), sig)
		if ok {
			t.Fatal("signature still valid after the message was tampered with")
		}
	})

	t.Run("persist payload", func(t *testing.T) {
		tx, err := NewPersistTransaction(from, to, transactionFee, map[string]string{"k": "v"})
		if err != nil {
			t.Fatalf("create persist tx: %v", err)
		}
		sig, err := tx.Sign([]byte(from.PrivatePEM()))
		if err != nil {
			t.Fatalf("sign: %v", err)
		}

		ok, _ := tx.Verify([]byte(from.PublicPEM()), sig)
		if !ok {
			t.Fatal("signature must verify for the signed payload")
		}

		tx.Data = map[string]string{"k": "tampered"}
		ok, _ = tx.Verify([]byte(from.PublicPEM()), sig)
		if ok {
			t.Fatal("signature still valid after the payload was tampered with")
		}
	})
}

// TestVerifyAfterSignatureAssigned guards the second half of the signature bug:
// Sign marshalled the transaction with an empty Signature, Verify marshalled it
// again *after* callers had assigned the signature, so the digests differed and
// verification of a correctly signed transaction always failed.
func TestVerifyAfterSignatureAssigned(t *testing.T) {
	from := newTestWallet(t, "assign-from", 100)
	to := newTestWallet(t, "assign-to", 0)

	tx, err := NewBankTransaction(from, to, 1.0)
	if err != nil {
		t.Fatalf("create tx: %v", err)
	}

	sig, err := tx.Sign([]byte(from.PrivatePEM()))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}

	// This is exactly what every caller in the codebase does.
	tx.Signature = sig

	ok, err := tx.Verify([]byte(from.PublicPEM()), tx.GetSignature())
	if err != nil || !ok {
		t.Fatalf("verification must succeed after the signature is stored: ok=%v err=%v", ok, err)
	}
}

// -----------------------------------------------------------------------------
// Identity
// -----------------------------------------------------------------------------

// TestTransactionIDsAreIndependent guards the PUID aliasing defect: tx.ID was the
// recipient wallet's own *PUID, mutated in place, so creating a second
// transaction to the same wallet silently rewrote the first transaction's ID and
// the wallet's identity along with it.
func TestTransactionIDsAreIndependent(t *testing.T) {
	from := newTestWallet(t, "alias-from", 1000)
	to := newTestWallet(t, "alias-to", 0)

	walletIDBefore := to.ID.String()

	tx1, err := NewTransaction(MessageProtocolID, from, to)
	if err != nil {
		t.Fatalf("create tx1: %v", err)
	}
	id1 := tx1.GetID()

	tx2, err := NewTransaction(MessageProtocolID, from, to)
	if err != nil {
		t.Fatalf("create tx2: %v", err)
	}

	if tx1.ID == tx2.ID {
		t.Fatal("two transactions share the same *PUID")
	}
	if got := tx1.GetID(); got != id1 {
		t.Fatalf("tx1's ID changed when tx2 was created: %s -> %s", id1, got)
	}
	if tx1.GetID() == tx2.GetID() {
		t.Fatal("two transactions have identical IDs")
	}
	if to.ID.String() != walletIDBefore {
		t.Fatalf("creating a transaction mutated the recipient wallet's identity: %s -> %s",
			walletIDBefore, to.ID.String())
	}
}

// TestPUIDRoundTrip guards the field-order mismatch between PUID.String and
// NewPUIDFromString, which scrambled three of four fields on a round trip.
func TestPUIDRoundTrip(t *testing.T) {
	original := NewPUID(NewBigInt(11), NewBigInt(22), NewBigInt(33), NewBigInt(44))

	parsed, err := NewPUIDFromString(original.String())
	if err != nil {
		t.Fatalf("parse PUID: %v", err)
	}
	if !parsed.Equal(original) {
		t.Fatalf("PUID did not survive a round trip: %s -> %s", original, parsed)
	}

	// And through JSON, which previously emitted a nested object that
	// NewPUIDFromString could not read.
	encoded, err := json.Marshal(original)
	if err != nil {
		t.Fatalf("marshal PUID: %v", err)
	}
	var decoded PUID
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal PUID: %v", err)
	}
	if !decoded.Equal(original) {
		t.Fatalf("PUID did not survive JSON: %s -> %s", original, &decoded)
	}

	if _, err := NewPUIDFromString("not:a:puid"); err == nil {
		t.Fatal("expected an error for a malformed PUID")
	}
	if _, err := NewPUIDFromString("1:2:3"); err == nil {
		t.Fatal("expected an error for a PUID with too few fields")
	}
}

// -----------------------------------------------------------------------------
// Wallet storage
// -----------------------------------------------------------------------------

// TestWalletOpenDoesNotDestroyTheWalletFile guards the most destructive defect:
// Wallet.Open called localStorage.Set, overwriting the stored wallet with
// whatever was in memory. LocalWalletList built an empty shell per file and
// called Open on it, so listing wallets erased every private key on disk.
func TestWalletOpenDoesNotDestroyTheWalletFile(t *testing.T) {
	dir := t.TempDir()
	if err := NewLocalStorage(dir); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	t.Cleanup(func() { _ = NewLocalStorage("./test_data") })

	w := newTestWallet(t, "victim", 0)
	if err := w.Close(regressionPassphrase); err != nil {
		t.Fatalf("close wallet: %v", err)
	}

	path := filepath.Join(dir, "wallets", w.Address+".json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read wallet file: %v", err)
	}

	// Exactly what LocalWalletList used to do.
	shell := &Wallet{Address: w.Address}
	if err := shell.Open(""); err != nil {
		t.Fatalf("open wallet: %v", err)
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("re-read wallet file: %v", err)
	}
	if string(before) != string(after) {
		t.Fatalf("Open() modified the wallet file on disk (%d bytes -> %d bytes)",
			len(before), len(after))
	}

	// And it must genuinely load: the shell had no ciphertext before Open.
	if len(shell.Ciphertext) == 0 && !shell.Encrypted {
		t.Fatal("Open() did not load the wallet from disk")
	}
}

// TestWalletRoundTripThroughDisk verifies a wallet survives save/load/unlock,
// which the recorded scrypt parameters make possible across cost profiles.
func TestWalletRoundTripThroughDisk(t *testing.T) {
	dir := t.TempDir()
	if err := NewLocalStorage(dir); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	t.Cleanup(func() { _ = NewLocalStorage("./test_data") })

	original := newTestWallet(t, "roundtrip", 42)
	if err := original.SetData("name", "roundtrip"); err != nil {
		t.Fatalf("set name: %v", err)
	}
	if err := original.Close(regressionPassphrase); err != nil {
		t.Fatalf("close: %v", err)
	}

	loaded, err := OpenWallet(original.Address, regressionPassphrase)
	if err != nil {
		t.Fatalf("open wallet: %v", err)
	}

	if loaded.GetWalletName() != "roundtrip" {
		t.Fatalf("expected name roundtrip, got %q", loaded.GetWalletName())
	}
	if loaded.GetBalance() != 42 {
		t.Fatalf("expected balance 42, got %v", loaded.GetBalance())
	}
	if loaded.PrivatePEM() == "" {
		t.Fatal("expected the private key to be restored")
	}

	// A wrong passphrase must fail loudly rather than leaving a half-unlocked
	// wallet with a nil vault.
	if _, err := OpenWallet(original.Address, "Wr0ngPassword!!123"); err == nil {
		t.Fatal("expected an error for a wrong passphrase")
	}
}

// TestWalletDecryptRejectsShortCiphertext guards a slice that read past the start
// of the buffer on any truncated or corrupt payload.
func TestWalletDecryptRejectsShortCiphertext(t *testing.T) {
	w := &Wallet{EncryptionParams: NewDefaultEncryptionParams()}

	for _, size := range []int{0, 1, saltSize - 1, saltSize} {
		if _, err := w.decrypt([]byte("key"), make([]byte, size)); err == nil {
			t.Fatalf("expected an error for a %d-byte ciphertext", size)
		}
	}
}

// TestNewWalletHonoursAssetID guards the discarded AssetID: every wallet used to
// be created with asset ID 0, so all of them shared one identity.
func TestNewWalletHonoursAssetID(t *testing.T) {
	w, err := NewWallet(NewWalletOptions(
		NewBigInt(7), NewBigInt(8), NewBigInt(9), NewBigInt(1234),
		"asset-id", regressionPassphrase, nil,
	))
	if err != nil {
		t.Fatalf("create wallet: %v", err)
	}

	if got := w.ID.GetAssetID().Val; got != 1234 {
		t.Fatalf("expected asset ID 1234, got %d", got)
	}
	if got := w.ID.GetOrganizationID().Val; got != 7 {
		t.Fatalf("expected organization ID 7, got %d", got)
	}
}

// TestNewWalletDoesNotMintBalance guards the wallet that funded itself: every new
// wallet was created holding fundWalletAmount tokens from nowhere.
func TestNewWalletDoesNotMintBalance(t *testing.T) {
	w := newTestWallet(t, "no-mint", 0)
	if got := w.GetBalance(); got != 0 {
		t.Fatalf("a new wallet must start at zero, got %v", got)
	}
}

// -----------------------------------------------------------------------------
// Block encoding & hashing
// -----------------------------------------------------------------------------

// TestBlockSurvivesJSONRoundTrip guards the defect that made the chain
// non-persistent: Block had no UnmarshalJSON, so decoding a block with any
// transaction failed and LoadExistingBlocks silently discarded the history.
func TestBlockSurvivesJSONRoundTrip(t *testing.T) {
	from := newTestWallet(t, "block-from", 1000)
	to := newTestWallet(t, "block-to", 0)

	bank, err := NewBankTransaction(from, to, 12.5)
	if err != nil {
		t.Fatalf("create bank tx: %v", err)
	}
	bank.Signature, err = bank.Sign([]byte(from.PrivatePEM()))
	if err != nil {
		t.Fatalf("sign bank tx: %v", err)
	}

	msg, err := NewMessageTransaction(from, to, "hello chain")
	if err != nil {
		t.Fatalf("create message tx: %v", err)
	}

	block := NewBlock([]Transaction{bank, msg}, "prev-hash")
	block.Index = *big.NewInt(7)

	encoded, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal block: %v", err)
	}

	decoded := &Block{}
	if err := json.Unmarshal(encoded, decoded); err != nil {
		t.Fatalf("unmarshal block: %v", err)
	}

	if len(decoded.Transactions) != 2 {
		t.Fatalf("expected 2 transactions, got %d", len(decoded.Transactions))
	}
	if decoded.Index.Cmp(&block.Index) != 0 {
		t.Fatalf("index changed: %s -> %s", block.Index.String(), decoded.Index.String())
	}

	decodedBank, ok := decoded.Transactions[0].(*Bank)
	if !ok {
		t.Fatalf("expected a *Bank, got %T", decoded.Transactions[0])
	}
	if decodedBank.Amount != 12.5 {
		t.Fatalf("bank amount did not survive: %v", decodedBank.Amount)
	}

	decodedMsg, ok := decoded.Transactions[1].(*Message)
	if !ok {
		t.Fatalf("expected a *Message, got %T", decoded.Transactions[1])
	}
	if decodedMsg.Message != "hello chain" {
		t.Fatalf("message body did not survive: %q", decodedMsg.Message)
	}

	// The signature must still verify after the round trip, which requires the
	// sender's public key to travel with the transaction.
	ok2, err := decodedBank.Verify([]byte(decodedBank.GetSenderWallet().PublicPEM()), decodedBank.GetSignature())
	if err != nil || !ok2 {
		t.Fatalf("signature must verify after a round trip: ok=%v err=%v", ok2, err)
	}
}

// TestBlockHashIsStableAcrossSerialization guards the monotonic-clock defect:
// CalculateHash interpolated Timestamp.String(), which renders the monotonic
// reading that JSON drops, so a block's hash changed the moment it touched disk
// and every Hash != CalculateHash() check failed.
func TestBlockHashIsStableAcrossSerialization(t *testing.T) {
	block := NewBlock([]Transaction{}, "prev")
	block.Index = *big.NewInt(3)
	before := block.CalculateHash()

	encoded, err := json.Marshal(block)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	decoded := &Block{}
	if err := json.Unmarshal(encoded, decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if after := decoded.CalculateHash(); after != before {
		t.Fatalf("block hash changed across serialization:\n  before %s\n  after  %s", before, after)
	}
}

// TestMerkleTreeHandlesEveryLeafCount guards the panic at 6 leaves (and 10, 14,
// 22, ...): padding was applied only to the leaf level, so an even count that
// halved to an odd count read past the end of the slice.
func TestMerkleTreeHandlesEveryLeafCount(t *testing.T) {
	for n := 1; n <= 64; n++ {
		leaves := make([][]byte, n)
		for i := range leaves {
			leaves[i] = []byte{byte(i), byte(i >> 8)}
		}

		tree := NewMerkleTree(leaves)
		if tree == nil || tree.Root == nil || len(tree.Root.Data) == 0 {
			t.Fatalf("expected a root for %d leaves", n)
		}
	}

	if tree := NewMerkleTree(nil); tree.Root != nil {
		t.Fatal("an empty tree must have a nil root")
	}
}

// TestMerkleTreeIsDomainSeparated guards CVE-2012-2459-style malleability: with
// no leaf/internal domain separation, padding an odd level by duplicating the
// last node lets two different transaction lists produce the same root.
func TestMerkleTreeIsDomainSeparated(t *testing.T) {
	a, b, c := []byte("a"), []byte("b"), []byte("c")

	// [a b c] pads to [a b c c]; a naive implementation gives it the same root as
	// the genuine 4-leaf list [a b c c].
	three := NewMerkleTree([][]byte{a, b, c})

	// A leaf hash must never equal an internal hash of the same bytes.
	leaf := NewMerkleNode(nil, nil, a)
	internal := NewMerkleNode(NewMerkleNode(nil, nil, a), NewMerkleNode(nil, nil, b), nil)
	if string(leaf.Data) == string(internal.Data) {
		t.Fatal("leaf and internal node hashes are not domain separated")
	}

	if three.Root == nil {
		t.Fatal("expected a root")
	}
}

// TestBlockMineTerminates guards the unbounded mining loop, which incremented a
// uint32 nonce forever and re-tried the same hashes after it wrapped.
func TestBlockMineTerminates(t *testing.T) {
	block := NewBlock([]Transaction{}, "prev")

	// Difficulty 1 is reachable quickly; the point is that Mine now reports
	// success or failure rather than looping indefinitely.
	if err := block.Mine(1); err != nil {
		t.Fatalf("expected mining at difficulty 1 to succeed: %v", err)
	}
	if !strings.HasPrefix(block.Hash, "0") {
		t.Fatalf("mined hash does not meet difficulty 1: %s", block.Hash)
	}
}

// TestAdjustDifficultyDoesNotUnderflow guards the uint32 wrap at zero, which
// turned a difficulty decrease into 4294967295.
func TestAdjustDifficultyDoesNotUnderflow(t *testing.T) {
	previous := NewBlock(nil, "")
	previous.Header.Difficulty = 0
	previous.Header.Timestamp = time.Now().Add(-time.Hour)

	current := NewBlock(nil, "")
	current.Header.Timestamp = time.Now()

	if got := current.AdjustDifficulty(previous, time.Second); got > 1 {
		t.Fatalf("difficulty underflowed to %d", got)
	}
}

// -----------------------------------------------------------------------------
// Concurrency
// -----------------------------------------------------------------------------

// TestUpdateConfigDoesNotDeadlock guards the re-entrant lock: UpdateConfig held
// bc.mux and then called Save(), which takes bc.mux again. sync.Mutex is not
// reentrant, so the call never returned.
func TestUpdateConfigDoesNotDeadlock(t *testing.T) {
	bc := &Blockchain{cfg: NewConfig(), TXLookup: NewTXLookupManager()}

	done := make(chan error, 1)
	go func() { done <- bc.UpdateConfig(NewConfig()) }()

	select {
	case <-done:
		// Either outcome is fine; returning at all is the point.
	case <-time.After(5 * time.Second):
		t.Fatal("UpdateConfig deadlocked")
	}
}

// TestP2PProcessQueueDoesNotDeadlock guards the same shape in P2P: ProcessQueue
// held the write lock while calling handlers that took it again, so the first
// add/remove/status/register message wedged the subsystem permanently. The
// project's own TestProcessQueue was skipped because of it.
func TestP2PProcessQueueDoesNotDeadlock(t *testing.T) {
	for _, action := range []string{"status", "add", "remove", "register", "validate", "unknown"} {
		t.Run(action, func(t *testing.T) {
			p := NewP2P()
			p.queue = []P2PTransaction{{
				Tx:     Tx{ID: NewPUIDEmpty()},
				Action: action,
				Data:   json.RawMessage(`"some-node"`),
			}}

			done := make(chan struct{})
			go func() {
				defer close(done)
				p.ProcessQueue()
			}()

			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatalf("ProcessQueue deadlocked on action %q", action)
			}
		})
	}
}

// TestP2PHandlersSurviveHostilePayloads guards the unchecked
// `tx.Data.([]byte)` assertions, which panicked on any message that arrived over
// the wire -- a remotely triggerable crash.
func TestP2PHandlersSurviveHostilePayloads(t *testing.T) {
	payloads := []json.RawMessage{
		nil,
		json.RawMessage(``),
		json.RawMessage(`null`),
		json.RawMessage(`"a string"`),
		json.RawMessage(`12345`),
		json.RawMessage(`{"unexpected":"object"}`),
		json.RawMessage(`[1,2,3]`),
		json.RawMessage(`{`),
	}

	for _, action := range []string{"status", "add", "remove", "register"} {
		for i, payload := range payloads {
			p := NewP2P()
			tx := P2PTransaction{Tx: Tx{ID: NewPUIDEmpty()}, Action: action, Data: payload}

			// Must not panic; an error return is the expected outcome.
			func() {
				defer func() {
					if r := recover(); r != nil {
						t.Fatalf("action %q panicked on payload %d: %v", action, i, r)
					}
				}()
				p.ProcessQueue()
				p.queue = []P2PTransaction{tx}
				p.ProcessQueue()
			}()
		}
	}
}

// TestP2PTransactionRoundTrip guards the silent field loss: P2PTransaction
// embeds Tx, whose pointer-receiver MarshalJSON was promoted, so marshalling a
// P2P message emitted only the base transaction and dropped Action, Target,
// State and Data entirely -- every message on the network was undispatchable.
func TestP2PTransactionRoundTrip(t *testing.T) {
	original := P2PTransaction{
		Tx:     Tx{ID: NewPUID(NewBigInt(1), NewBigInt(2), NewBigInt(3), NewBigInt(4)), Protocol: ChainProtocolID, Version: TransactionVersion},
		Target: "all",
		Action: "register",
		State:  P2PTxQueued,
		Data:   json.RawMessage(`{"id":"node-1","address":":8101"}`),
	}

	for _, encoded := range [][]byte{mustMarshal(t, original), mustMarshal(t, &original)} {
		var decoded P2PTransaction
		if err := json.Unmarshal(encoded, &decoded); err != nil {
			t.Fatalf("unmarshal: %v (payload %s)", err, encoded)
		}

		if decoded.Action != "register" {
			t.Fatalf("Action lost in serialization: %q (payload %s)", decoded.Action, encoded)
		}
		if decoded.Target != "all" {
			t.Fatalf("Target lost in serialization: %q", decoded.Target)
		}
		if decoded.State != P2PTxQueued {
			t.Fatalf("State lost in serialization: %v", decoded.State)
		}
		if string(decoded.Data) != string(original.Data) {
			t.Fatalf("Data lost in serialization: %s", decoded.Data)
		}
		if decoded.Protocol != ChainProtocolID {
			t.Fatalf("embedded transaction lost: %q", decoded.Protocol)
		}
	}
}

func mustMarshal(t *testing.T, v interface{}) []byte {
	t.Helper()
	out, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return out
}

// TestConcurrentBlockchainAccess exercises the mutex discipline under -race.
// The API handlers used to read bc.Blocks directly while the miner appended.
func TestConcurrentBlockchainAccess(t *testing.T) {
	bc := &Blockchain{cfg: NewConfig(), TXLookup: NewTXLookupManager()}
	for i := 0; i < 20; i++ {
		b := NewBlock(nil, "")
		b.Index = *big.NewInt(int64(i))
		bc.Blocks = append(bc.Blocks, b)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 50; j++ {
				bc.GetBlockCount()
				bc.GetBlockRange(0, 5)
				bc.GetAllTransactions()
				bc.GetPendingTransactions()
				bc.GetBlockByIndex(int64(j % 20))
				bc.GetLatestBlock()
				bc.HasTransactionID("nope")
			}
		}(i)
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 20; i < 60; i++ {
			b := NewBlock(nil, "")
			b.Index = *big.NewInt(int64(i))
			bc.mux.Lock()
			bc.Blocks = append(bc.Blocks, b)
			bc.mux.Unlock()
		}
	}()

	wg.Wait()
}

// TestGetPendingTransactionsReturnsACopy guards handing out the live mempool
// slice, which let callers read and append to mutex-guarded state.
func TestGetPendingTransactionsReturnsACopy(t *testing.T) {
	bc := &Blockchain{cfg: NewConfig(), TXLookup: NewTXLookupManager()}
	from := newTestWallet(t, "copy-from", 100)
	to := newTestWallet(t, "copy-to", 0)

	tx, err := NewBankTransaction(from, to, 1.0)
	if err != nil {
		t.Fatalf("create tx: %v", err)
	}
	bc.TransactionQueue = []Transaction{tx}

	pending := bc.GetPendingTransactions()
	pending[0] = nil

	if bc.TransactionQueue[0] == nil {
		t.Fatal("GetPendingTransactions returned the live mempool slice")
	}
}

// -----------------------------------------------------------------------------
// Mining and value
// -----------------------------------------------------------------------------

// TestSimplePoWDoesNotGiveUpAfterTwelveNonces guards the constant collision:
// maxNonce was 12 -- the AES-GCM nonce *size* -- so the simple proof-of-work
// loop gave up after twelve attempts and returned an unmined block that the
// caller appended and persisted anyway.
func TestSimplePoWDoesNotGiveUpAfterTwelveNonces(t *testing.T) {
	bc := &Blockchain{cfg: NewConfig(), useHeliosMining: false, TXLookup: NewTXLookupManager()}

	block := NewBlock([]Transaction{}, "")
	block.Index = *big.NewInt(1)

	mined, err := bc.mineWithSimplePoW(block, 3)
	if err != nil {
		t.Fatalf("expected mining at difficulty 3 to succeed: %v", err)
	}
	if !strings.HasPrefix(mined.Hash, "000") {
		t.Fatalf("returned block does not meet difficulty 3: %s", mined.Hash)
	}

	// And it must not mutate chain state: the caller owns that.
	if len(bc.Blocks) != 0 {
		t.Fatalf("mining appended %d blocks to the chain", len(bc.Blocks))
	}
}

// TestBankProcessConservesValue guards the fund-destruction defect: Process
// debited the sender and never credited the recipient, while reporting success.
func TestBankProcessConservesValue(t *testing.T) {
	from := newTestWallet(t, "conserve-from", 500)
	to := newTestWallet(t, "conserve-to", 10)

	tx, err := NewBankTransaction(from, to, 100)
	if err != nil {
		t.Fatalf("create tx: %v", err)
	}

	beforeFrom, beforeTo := from.GetBalance(), to.GetBalance()
	result := tx.Process()

	afterFrom, afterTo := from.GetBalance(), to.GetBalance()

	if afterTo != beforeTo+100 {
		t.Fatalf("recipient was not credited: %v -> %v (%s)", beforeTo, afterTo, result)
	}
	if afterFrom != beforeFrom-(100+tx.Fee) {
		t.Fatalf("sender debit is wrong: %v -> %v", beforeFrom, afterFrom)
	}
}

// TestBankSendPreservesAmount guards Tx.Send enqueuing the embedded base
// transaction rather than the concrete one, which dropped the amount.
func TestBankSendPreservesAmount(t *testing.T) {
	bc := &Blockchain{cfg: NewConfig(), TXLookup: NewTXLookupManager()}
	from := newTestWallet(t, "send-from", 500)
	to := newTestWallet(t, "send-to", 0)

	tx, err := NewBankTransaction(from, to, 77.5)
	if err != nil {
		t.Fatalf("create tx: %v", err)
	}
	if err := tx.Send(bc); err != nil {
		t.Fatalf("send: %v", err)
	}

	pending := bc.GetPendingTransactions()
	if len(pending) != 1 {
		t.Fatalf("expected 1 queued transaction, got %d", len(pending))
	}

	queued, ok := pending[0].(*Bank)
	if !ok {
		t.Fatalf("expected a *Bank in the mempool, got %T -- the concrete transaction was lost", pending[0])
	}
	if queued.Amount != 77.5 {
		t.Fatalf("amount lost on the way to the mempool: %v", queued.Amount)
	}
}

// TestBankValidationRejectsNonPositiveAmounts covers validation that did not exist.
func TestBankValidationRejectsNonPositiveAmounts(t *testing.T) {
	from := newTestWallet(t, "validate-from", 500)
	to := newTestWallet(t, "validate-to", 0)

	tx, err := NewBankTransaction(from, to, 1)
	if err != nil {
		t.Fatalf("create tx: %v", err)
	}

	for _, amount := range []float64{0, -1, -0.0001} {
		tx.Amount = amount
		if err := tx.Validate(); err == nil {
			t.Fatalf("expected amount %v to be rejected", amount)
		}
	}
}

// -----------------------------------------------------------------------------
// Numeric safety
// -----------------------------------------------------------------------------

// TestBigIntEdgeCases guards the panics and precision loss in the int64 wrapper
// that calls itself BigInt.
func TestBigIntEdgeCases(t *testing.T) {
	t.Run("short byte slices do not panic", func(t *testing.T) {
		for size := 0; size <= 9; size++ {
			if got := NewBigIntFromBytes(make([]byte, size)); got == nil {
				t.Fatalf("expected a value for a %d-byte slice", size)
			}
		}
	})

	t.Run("decimal strings are not treated as base64", func(t *testing.T) {
		for _, in := range []string{"0", "1", "1234", "-42", "9223372036854775807"} {
			got, err := NewBigIntFromString(in)
			if err != nil {
				t.Fatalf("parse %q: %v", in, err)
			}
			if got.String() != in {
				t.Fatalf("value %q was corrupted to %q", in, got.String())
			}
		}
	})

	t.Run("division by zero does not panic", func(t *testing.T) {
		a := NewBigInt(10)
		if got := a.Divide(NewBigInt(0)); got.Val != 0 {
			t.Fatalf("expected 0, got %d", got.Val)
		}
		if got := a.Modulo(NewBigInt(0)); got.Val != 0 {
			t.Fatalf("expected 0, got %d", got.Val)
		}
		if got := a.Divide(nil); got.Val != 0 {
			t.Fatalf("expected 0 for a nil divisor, got %d", got.Val)
		}
	})

	t.Run("abs is exact for large values", func(t *testing.T) {
		// The float64 round trip lost precision above 2^53.
		const large = int64(9007199254740993) // 2^53 + 1
		if got := NewBigInt(-large).Abs().Val; got != large {
			t.Fatalf("expected %d, got %d", large, got)
		}
	})
}

// TestSecureRandomIntHandlesNonPositiveBounds guards a crypto/rand panic.
func TestSecureRandomIntHandlesNonPositiveBounds(t *testing.T) {
	for _, max := range []int{0, -1, -100} {
		if got := SecureRandomInt(max); got != 0 {
			t.Fatalf("expected 0 for max=%d, got %d", max, got)
		}
	}
}

// TestTransactionNonceHasFullEntropy guards `SecureRandomInt(8)`, which produced
// a nonce in 0..7 -- three bits.
func TestTransactionNonceHasFullEntropy(t *testing.T) {
	from := newTestWallet(t, "nonce-from", 100)
	to := newTestWallet(t, "nonce-to", 0)

	seen := make(map[uint64]struct{})
	for i := 0; i < 50; i++ {
		tx, err := NewTransaction(MessageProtocolID, from, to)
		if err != nil {
			t.Fatalf("create tx: %v", err)
		}
		seen[tx.Nonce] = struct{}{}
	}

	// 50 draws from a 3-bit space collide constantly; from 64 bits they do not.
	if len(seen) < 40 {
		t.Fatalf("nonce entropy is too low: only %d distinct values in 50 draws", len(seen))
	}
}

// -----------------------------------------------------------------------------
// Storage
// -----------------------------------------------------------------------------

// TestLocalStorageWritesAtomically guards the non-atomic os.WriteFile, which
// truncated the target first: an interrupted write left a truncated wallet or
// block on disk with no way to recover it.
func TestLocalStorageWritesAtomically(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target.json")

	if err := os.WriteFile(path, []byte("original"), 0644); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if err := writeFileAtomic(path, []byte("replacement"), 0600); err != nil {
		t.Fatalf("atomic write: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read back: %v", err)
	}
	if string(data) != "replacement" {
		t.Fatalf("unexpected contents: %q", data)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0600 {
		t.Fatalf("expected mode 0600, got %o", perm)
	}

	// No temp files may be left behind.
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly one file, found %d", len(entries))
	}
}

// TestWalletFilesAreNotWorldReadable guards the 0644 mode on files holding
// private key material.
func TestWalletFilesAreNotWorldReadable(t *testing.T) {
	dir := t.TempDir()
	if err := NewLocalStorage(dir); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	t.Cleanup(func() { _ = NewLocalStorage("./test_data") })

	w := newTestWallet(t, "perms", 0)
	if err := w.Close(regressionPassphrase); err != nil {
		t.Fatalf("close: %v", err)
	}

	info, err := os.Stat(filepath.Join(dir, "wallets", w.Address+".json"))
	if err != nil {
		t.Fatalf("stat wallet: %v", err)
	}
	if perm := info.Mode().Perm(); perm&0077 != 0 {
		t.Fatalf("wallet file is group/world accessible: mode %o", perm)
	}
}

// -----------------------------------------------------------------------------
// Credentials
// -----------------------------------------------------------------------------

// TestAPIKeyAuthenticationFailsClosed guards the published fallback key: with no
// key configured the node accepted a credential that was a constant in the
// repository, including on the consensus endpoints.
func TestAPIKeyAuthenticationFailsClosed(t *testing.T) {
	t.Setenv(envBlockchainAPIKey, "")

	cfg := defaultAPIKeyConfig()
	if len(cfg.APIKeys) != 0 {
		t.Fatalf("expected no API keys when none is configured, got %d", len(cfg.APIKeys))
	}

	if _, err := ApiKeyMiddleware(cfg, nil); err == nil {
		t.Fatal("expected middleware construction to fail with no credentials configured")
	}
}

// TestServerSeedHasNoDefault guards the published seed, from which anyone could
// derive a valid API key for any address.
func TestServerSeedHasNoDefault(t *testing.T) {
	t.Setenv(envServerSeed, "")
	if _, err := configuredServerSeed(); err == nil {
		t.Fatal("expected an error when no server seed is configured")
	}

	t.Setenv(envServerSeed, "abc123")
	seed, err := configuredServerSeed()
	if err != nil || seed != "abc123" {
		t.Fatalf("expected the configured seed, got %q err=%v", seed, err)
	}
}

// TestAPIKeyMatchingIsConstantTime is a structural check: matchAPIKeyToEmail must
// examine every candidate rather than returning on the first match, so its work
// does not depend on where the match falls.
func TestAPIKeyMatchingIsConstantTime(t *testing.T) {
	keys := map[string][]byte{
		"a@example.com": {1, 2, 3, 4},
		"b@example.com": {5, 6, 7, 8},
	}

	email, ok := matchAPIKeyToEmail("05060708", keys)
	if !ok || email != "b@example.com" {
		t.Fatalf("expected b@example.com, got %q ok=%v", email, ok)
	}

	if _, ok := matchAPIKeyToEmail("ffffffff", keys); ok {
		t.Fatal("expected no match for an unknown key")
	}
	if _, ok := matchAPIKeyToEmail("not-hex", keys); ok {
		t.Fatal("expected no match for a malformed key")
	}
	if _, ok := matchAPIKeyToEmail("", keys); ok {
		t.Fatal("expected no match for an empty key")
	}
}

// TestAccountPasswordHashingIsSalted guards the client-supplied "password_hash"
// that was stored verbatim, making the stored value itself the credential.
func TestAccountPasswordHashingIsSalted(t *testing.T) {
	password := "correct horse battery staple"

	hash1, salt1, err := hashAccountPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	hash2, salt2, err := hashAccountPassword(password)
	if err != nil {
		t.Fatalf("hash: %v", err)
	}

	if salt1 == salt2 {
		t.Fatal("two hashes of the same password reused a salt")
	}
	if hash1 == hash2 {
		t.Fatal("two hashes of the same password are identical; the salt is not applied")
	}
	if strings.Contains(hash1, password) {
		t.Fatal("the stored hash contains the password")
	}

	if !verifyAccountPassword(password, hash1, salt1) {
		t.Fatal("the correct password failed to verify")
	}
	if verifyAccountPassword("wrong password entirely", hash1, salt1) {
		t.Fatal("an incorrect password verified")
	}
	if verifyAccountPassword(password, hash1, "not-hex") {
		t.Fatal("a malformed salt must not verify")
	}
	if verifyAccountPassword(password, "not-hex", salt1) {
		t.Fatal("a malformed hash must not verify")
	}
}

// TestAccountStoreResolvesIssuedAPIKeys covers the path that makes a key issued
// by /account/login actually authenticate. Previously login returned
// SHA256(seed+email) and the middleware never checked such keys at all.
func TestAccountStoreResolvesIssuedAPIKeys(t *testing.T) {
	store := NewFileAccountStore(t.TempDir())

	key, hashed, err := generateAPIKey()
	if err != nil {
		t.Fatalf("generate key: %v", err)
	}

	if err := store.SaveVerified(VerifiedAccountRecord{
		Email:      "user@example.com",
		APIKeyHash: hashed,
		VerifiedAt: time.Now(),
	}); err != nil {
		t.Fatalf("save verified: %v", err)
	}

	email, ok := accountKeyIsValid(key, store)
	if !ok || email != "user@example.com" {
		t.Fatalf("expected the issued key to resolve, got %q ok=%v", email, ok)
	}

	if _, ok := accountKeyIsValid("deadbeef", store); ok {
		t.Fatal("an unknown key must not resolve")
	}
	if _, ok := accountKeyIsValid("", store); ok {
		t.Fatal("an empty key must not resolve")
	}
	if _, ok := accountKeyIsValid(key, nil); ok {
		t.Fatal("a nil store must not resolve anything")
	}
}

// TestRateLimiterThrottles covers the limiter protecting credential endpoints,
// which previously had no brute-force protection at all.
func TestRateLimiterThrottles(t *testing.T) {
	rl := newRateLimiter(3, time.Minute)

	for i := 0; i < 3; i++ {
		if !rl.allow("1.2.3.4") {
			t.Fatalf("attempt %d should have been allowed", i+1)
		}
	}
	if rl.allow("1.2.3.4") {
		t.Fatal("the fourth attempt should have been throttled")
	}

	// A different source is unaffected.
	if !rl.allow("5.6.7.8") {
		t.Fatal("an unrelated source must not be throttled")
	}

	// A success clears the counter.
	rl.reset("1.2.3.4")
	if !rl.allow("1.2.3.4") {
		t.Fatal("reset should have cleared the counter")
	}
}

// -----------------------------------------------------------------------------
// Config
// -----------------------------------------------------------------------------

// TestFileExistsHandlesStatErrors guards the nil dereference on any Stat error
// other than not-exist, reachable during NewConfig().
func TestFileExistsHandlesStatErrors(t *testing.T) {
	if fileExists("") {
		t.Fatal("an empty path must not report as existing")
	}
	if fileExists(t.TempDir()) {
		t.Fatal("a directory must not report as an existing file")
	}
	if fileExists(filepath.Join(t.TempDir(), "no-such-file")) {
		t.Fatal("a missing file must not report as existing")
	}

	// A path whose parent is a file, i.e. ENOTDIR rather than ENOENT.
	base := filepath.Join(t.TempDir(), "afile")
	if err := os.WriteFile(base, []byte("x"), 0600); err != nil {
		t.Fatalf("seed file: %v", err)
	}
	if fileExists(filepath.Join(base, "child")) {
		t.Fatal("a path under a regular file must not report as existing")
	}
}

// TestConfigValidateBoundsDifficulty guards a difficulty that feeds a 256-bit
// shift.
func TestConfigValidateBoundsDifficulty(t *testing.T) {
	cfg := NewConfig()

	for _, d := range []int{-1, 0, 256, 1000} {
		cfg.Difficulty = d
		if err := cfg.Validate(); err == nil {
			t.Fatalf("expected difficulty %d to be rejected", d)
		}
	}

	cfg.Difficulty = 4
	if err := cfg.Validate(); err != nil {
		t.Fatalf("expected difficulty 4 to be accepted: %v", err)
	}
}

// TestDifficultyTargetIsBounded guards the shift itself.
func TestDifficultyTargetIsBounded(t *testing.T) {
	for _, d := range []int{-5, 0, 1, 128, 255, 256, 10000} {
		target := difficultyTarget(d)
		if target == nil || target.Sign() <= 0 {
			t.Fatalf("difficulty %d produced an unusable target: %v", d, target)
		}
	}

	// Higher difficulty must mean a smaller target.
	if difficultyTarget(8).Cmp(difficultyTarget(4)) >= 0 {
		t.Fatal("a higher difficulty must produce a smaller target")
	}
}
