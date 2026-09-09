package sdk

import (
	"testing"
	"time"
)

// End-to-end tests that exercise the whole node rather than one component.
//
// The suite used to pass while the product did not work: blocks with
// transactions could not be reloaded from disk at all (the decoder did not
// exist), and no test ever restarted a chain and read it back. Every one of
// those P0 defects would have been caught by a test in this file.

// integrationChain builds a chain on its own storage, ready to mine.
func integrationChain(t *testing.T, dir string) *Blockchain {
	t.Helper()

	if err := NewLocalStorage(dir); err != nil {
		t.Fatalf("init storage: %v", err)
	}
	t.Cleanup(func() { _ = NewLocalStorage("./test_data") })

	cfg := NewConfig()
	cfg.DataPath = dir
	cfg.Difficulty = 1 // real mining, but cheap enough to run in a test
	cfg.EnableAPI = false

	bc := NewBlockchain(cfg)
	if bc == nil {
		t.Fatal("failed to create blockchain")
	}
	bc.useHeliosMining = false
	return bc
}

func integrationWallet(t *testing.T, name string) *Wallet {
	t.Helper()
	w, err := NewWallet(NewWalletOptions(
		ThisBlockchainOrganizationID, ThisBlockchainAppID,
		ThisBlockchainAdminUserID, ThisBlockchainDevAssetID,
		name, testPassPhrase, []string{"integration"}))
	if err != nil {
		t.Fatalf("wallet %s: %v", name, err)
	}
	if err := w.Open(testPassPhrase); err != nil {
		t.Fatalf("open wallet %s: %v", name, err)
	}
	return w
}

// freeMessage builds a zero-fee MESSAGE transaction.
//
// A zero fee spends nothing, so the transaction needs no funding and replays
// correctly when the chain is reloaded from disk. That matters for the restart
// test: funding an address by poking a UTXO into the set (as the unit tests do)
// leaves nothing in any block, so a replay has nothing to spend.
func freeMessage(t *testing.T, from, to *Wallet, body string) *Message {
	t.Helper()

	tx, err := NewMessageTransaction(from, to, body)
	if err != nil {
		t.Fatalf("build message: %v", err)
	}
	tx.Fee = 0

	signature, err := tx.Sign([]byte(from.PrivatePEM()))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	tx.Signature = signature
	return tx
}

// TestEndToEndSubmitMineRestartAndRevalidate is the test whose absence let every
// P0 through: submit a transaction, mine it, restart the node, and confirm the
// chain and balances survived.
func TestEndToEndSubmitMineRestartAndRevalidate(t *testing.T) {
	dir := t.TempDir()
	bc := integrationChain(t, dir)

	alice := integrationWallet(t, "e2e-alice")
	bob := integrationWallet(t, "e2e-bob")

	// 1. Submit a transaction.
	message := freeMessage(t, alice, bob, "hello from the integration test")
	if !bc.AddTransactionLocal(message) {
		t.Fatal("the transaction was refused by the mempool")
	}
	if bc.GetMempoolSize() != 1 {
		t.Fatalf("mempool holds %d transactions, want 1", bc.GetMempoolSize())
	}

	// 2. Mine it.
	heightBefore := bc.Height()
	bc.createNewBlock(bc.CurrentDifficulty())

	if bc.Height() != heightBefore+1 {
		t.Fatalf("no block was mined: height is still %d", bc.Height())
	}
	if bc.GetMempoolSize() != 0 {
		t.Fatal("the mined transaction is still in the mempool")
	}

	mined := bc.GetLatestBlock()
	if len(mined.Transactions) != 1 {
		t.Fatalf("the mined block holds %d transactions, want 1", len(mined.Transactions))
	}
	if mined.Transactions[0].GetID() != message.GetID() {
		t.Fatal("the mined block does not contain the submitted transaction")
	}

	minedHash := mined.Hash
	minedHeight := bc.Height()
	minedTxID := message.GetID()

	// 3. Restart: a completely fresh chain over the same storage.
	restarted := integrationChain(t, dir)

	if restarted.Height() != minedHeight {
		t.Fatalf("after restart the chain is %d blocks, want %d -- blocks carrying "+
			"transactions could not be reloaded at all before the wire format existed",
			restarted.Height(), minedHeight)
	}

	head := restarted.GetLatestBlock()
	if head.Hash != minedHash {
		t.Fatalf("the reloaded head hash is %s, want %s -- block hashes must be "+
			"stable across serialisation", head.Hash, minedHash)
	}
	if len(head.Transactions) != 1 {
		t.Fatalf("the reloaded block holds %d transactions, want 1", len(head.Transactions))
	}
	if head.Transactions[0].GetID() != minedTxID {
		t.Fatalf("the reloaded transaction is %s, want %s",
			head.Transactions[0].GetID(), minedTxID)
	}
	if head.Transactions[0].GetProtocol() != MessageProtocolID {
		t.Fatalf("the transaction came back as %s, not %s -- the protocol "+
			"discriminator is what makes the interface decodable",
			head.Transactions[0].GetProtocol(), MessageProtocolID)
	}

	// 4. Re-validate.
	if err := restarted.ValidateChain(); err != nil {
		t.Fatalf("the reloaded chain does not validate: %v", err)
	}
}

// TestEndToEndTransferMovesBalance covers the value path, which the restart test
// deliberately avoids so it can fund from the UTXO set directly.
func TestEndToEndTransferMovesBalance(t *testing.T) {
	bc := integrationChain(t, t.TempDir())

	alice := integrationWallet(t, "e2e-transfer-alice")
	bob := integrationWallet(t, "e2e-transfer-bob")

	creditAddressForTest(t, bc, alice.GetAddress(), 100)
	if err := alice.SetData("balance", 100.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	transfer, err := NewBankTransaction(alice, bob, 25)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	signature, err := transfer.Sign([]byte(alice.PrivatePEM()))
	if err != nil {
		t.Fatalf("sign: %v", err)
	}
	transfer.Signature = signature

	if !bc.AddTransactionLocal(transfer) {
		t.Fatal("the transfer was refused")
	}
	bc.createNewBlock(bc.CurrentDifficulty())

	if got := bc.GetBalanceUnits(bob.GetAddress()); got != 25*UnitsPerToken {
		t.Fatalf("Bob holds %s after the transfer, want 25", formatUnits(got))
	}

	// The fee left Alice and reached the miner and developer addresses.
	fees := bc.GetBalanceUnits(bc.cfg.MinerAddress) + bc.GetBalanceUnits(bc.cfg.DevAddress)
	if fees <= 0 {
		t.Fatal("the transaction fee was taken from the sender and paid to nobody")
	}
}

// TestEndToEndDoubleSpendIsRefusedAcrossBlocks: spending the same funds twice
// must fail, whether the second attempt arrives before or after the first is
// mined.
func TestEndToEndDoubleSpendIsRefusedAcrossBlocks(t *testing.T) {
	bc := integrationChain(t, t.TempDir())

	alice := integrationWallet(t, "e2e-spender")
	bob := integrationWallet(t, "e2e-receiver")

	creditAddressForTest(t, bc, alice.GetAddress(), 30)
	if err := alice.SetData("balance", 30.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	first, err := NewBankTransaction(alice, bob, 25)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !bc.AddTransactionLocal(first) {
		t.Fatal("the first transaction was refused")
	}

	// A second transaction spending the same funds must be refused while the
	// first is still queued: the mempool counts committed funds.
	second, err := NewBankTransaction(alice, bob, 25)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if bc.AddTransactionLocal(second) {
		t.Fatal("a double spend was admitted to the mempool")
	}

	bc.createNewBlock(bc.CurrentDifficulty())

	// And still refused after the first is mined and the funds are actually gone.
	third, err := NewBankTransaction(alice, bob, 25)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if bc.AddTransactionLocal(third) {
		t.Fatal("a spend of already-spent funds was admitted")
	}
}

// TestEndToEndChainSurvivesManyBlocks exercises the loop a running node spends
// its life in, then reloads the whole chain from disk.
//
// Zero-fee messages again, so the reload replays cleanly. The target block time
// is deliberately generous: mining every block instantly against a one-second
// target would retarget difficulty upward every window and turn this into a
// minute of proof of work for no extra coverage.
func TestEndToEndChainSurvivesManyBlocks(t *testing.T) {
	dir := t.TempDir()
	bc := integrationChain(t, dir)

	alice := integrationWallet(t, "e2e-loop-alice")
	bob := integrationWallet(t, "e2e-loop-bob")

	for i := 0; i < 8; i++ {
		tx := freeMessage(t, alice, bob, "block filler")
		if !bc.AddTransactionLocal(tx) {
			t.Fatalf("transaction %d was refused", i)
		}
		bc.createNewBlock(bc.CurrentDifficulty())
	}

	if bc.Height() < 8 {
		t.Fatalf("only %d blocks were produced from 8 attempts", bc.Height())
	}
	if err := bc.ValidateChain(); err != nil {
		t.Fatalf("the chain does not validate after 8 blocks: %v", err)
	}

	// Reload the whole thing and validate again.
	reloaded := integrationChain(t, dir)
	if reloaded.Height() != bc.Height() {
		t.Fatalf("reloaded %d blocks, mined %d", reloaded.Height(), bc.Height())
	}
	if err := reloaded.ValidateChain(); err != nil {
		t.Fatalf("the reloaded chain does not validate: %v", err)
	}

	// Every transaction must have survived the round trip.
	var reloadedTxs int
	for _, block := range reloaded.Blocks {
		reloadedTxs += len(block.Transactions)
	}
	var minedTxs int
	for _, block := range bc.Blocks {
		minedTxs += len(block.Transactions)
	}
	if reloadedTxs != minedTxs {
		t.Fatalf("reloaded %d transactions, mined %d", reloadedTxs, minedTxs)
	}
}

// TestEndToEndDifficultyRetargetsWhileMining: difficulty is derived from chain
// history, so a node mining its own chain must retarget and still accept what it
// produced.
func TestEndToEndDifficultyRetargetsWhileMining(t *testing.T) {
	bc := integrationChain(t, t.TempDir())
	bc.cfg.DifficultyWindow = 2
	bc.cfg.BlockTime = 600 // blocks mined instantly are far faster than this

	alice := integrationWallet(t, "e2e-retarget-alice")
	bob := integrationWallet(t, "e2e-retarget-bob")

	start := bc.CurrentDifficulty()
	for i := 0; i < 5; i++ {
		tx := freeMessage(t, alice, bob, "retarget filler")
		if !bc.AddTransactionLocal(tx) {
			t.Fatalf("transaction %d refused", i)
		}
		bc.createNewBlock(bc.CurrentDifficulty())
	}

	if bc.CurrentDifficulty() <= start {
		t.Fatalf("difficulty stayed at %d while blocks were mined far faster than "+
			"the %ds target", bc.CurrentDifficulty(), bc.cfg.BlockTime)
	}
	if err := bc.ValidateChain(); err != nil {
		t.Fatalf("a chain that retargeted does not validate: %v", err)
	}
}

// TestEndToEndSupplyIsConserved: no sequence of ordinary transactions may change
// the money supply.
func TestEndToEndSupplyIsConserved(t *testing.T) {
	bc := integrationChain(t, t.TempDir())

	alice := integrationWallet(t, "e2e-supply-alice")
	bob := integrationWallet(t, "e2e-supply-bob")

	creditAddressForTest(t, bc, alice.GetAddress(), 500)
	if err := alice.SetData("balance", 500.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	before := bc.UTXOSet().TotalUnits()

	for i := 0; i < 5; i++ {
		tx, err := NewBankTransaction(alice, bob, 10)
		if err != nil {
			t.Fatalf("build: %v", err)
		}
		if !bc.AddTransactionLocal(tx) {
			t.Fatalf("transaction %d refused", i)
		}
		bc.createNewBlock(bc.CurrentDifficulty())
	}

	if after := bc.UTXOSet().TotalUnits(); after != before {
		t.Fatalf("supply moved from %s to %s across ordinary transfers; fees move "+
			"coins between addresses, they do not create or destroy them",
			formatUnits(before), formatUnits(after))
	}
}

// TestEndToEndNodeLifecycle starts a real node, runs it, and shuts it down.
func TestEndToEndNodeLifecycle(t *testing.T) {
	n := newTestNode(t)

	if !n.IsReady() {
		t.Fatal("the node did not come up ready")
	}
	if n.Blockchain.Height() < 0 {
		t.Fatal("the node has no chain")
	}

	// The node wallet must be usable, not encrypted with a key nobody has.
	if n.Wallet == nil {
		t.Fatal("the node has no wallet")
	}
	if err := n.Wallet.Open(testPassPhrase); err != nil {
		t.Fatalf("the node wallet cannot be opened with the configured passphrase: %v", err)
	}

	// Metrics must be live from the start.
	if n.Blockchain.Metrics().Uptime() <= 0 {
		t.Fatal("the node reports no uptime")
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		n.shutdown()
	}()

	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("Shutdown did not return; a node that cannot stop cannot flush state")
	}
}

// TestNodeAndChainShareOneProgressIndicator.
//
// NewBlockchain used to adopt the node's indicator via GetNode(). That broke
// silently when construction stopped going through the global: newNode builds
// the chain before registering itself, so the lookup returned nil, the chain
// kept the indicator it made for itself, and the node had a second one. Both
// render to stdout, so the status line alternated between real numbers and a
// permanently empty "Blk:0/0 Up:0s".
func TestNodeAndChainShareOneProgressIndicator(t *testing.T) {
	n := newTestNode(t)

	if n.ProgressIndicator == nil {
		t.Fatal("the node has no progress indicator")
	}
	if n.Blockchain.GetProgressIndicator() != n.ProgressIndicator {
		t.Fatal("the node and its chain hold different progress indicators; both " +
			"render to the terminal and their output interleaves")
	}
}

// TestCreatedWalletsCarryTheirRecoveryPhrase: the phrase is never stored, so the
// creation response is the only chance to keep it. A wallet created without it
// surfacing anywhere is unrecoverable in practice.
func TestCreatedWalletsCarryTheirRecoveryPhrase(t *testing.T) {
	bc := integrationChain(t, t.TempDir())
	_ = bc

	wallet, err := NewWallet(NewWalletOptions(
		ThisBlockchainOrganizationID, ThisBlockchainAppID,
		ThisBlockchainAdminUserID, ThisBlockchainDevAssetID,
		"api-created", testPassPhrase, []string{"test"}))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	phrase := wallet.Mnemonic()
	if phrase == "" {
		t.Fatal("a freshly created wallet exposes no recovery phrase, so the " +
			"API has nothing to return and the wallet cannot be recovered")
	}
	if err := ValidateMnemonic(phrase); err != nil {
		t.Fatalf("the phrase is not a valid BIP-39 mnemonic: %v", err)
	}
}
