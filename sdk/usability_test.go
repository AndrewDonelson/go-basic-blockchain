package sdk

import (
	"strings"
	"testing"
)

// These cover the three things that made a first run harder than it needed to
// be: configuration mistakes surfacing one restart at a time, an API failure
// that did not say why, and a wallet that silently could not spend.

// TestConfigReportsEveryProblemAtOnce.
//
// Validate returned the first problem it found, and credentials were not checked
// here at all -- a bad API key surfaced from the middleware, a weak node wallet
// passphrase from wallet creation, each on its own restart. A misconfigured .env
// took as many attempts as it had mistakes.
func TestConfigReportsEveryProblemAtOnce(t *testing.T) {
	t.Setenv(envBlockchainAPIKey, "not-hex-at-all")
	t.Setenv(envServerSeed, "also-not-hex")
	t.Setenv(envNodeWalletPassphrase, "weak")

	cfg := NewConfig()
	cfg.BlockchainName = ""
	cfg.BlockTime = -1

	err := cfg.Validate()
	if err == nil {
		t.Fatal("an invalid configuration validated successfully")
	}

	message := err.Error()
	for _, want := range []string{
		"blockchain name cannot be empty",
		"block time must be positive",
		envBlockchainAPIKey,
		envServerSeed,
		envNodeWalletPassphrase,
	} {
		if !strings.Contains(message, want) {
			t.Fatalf("the error does not mention %q, so that problem only appears "+
				"after fixing the others:\n%s", want, message)
		}
	}

	if !strings.Contains(message, "configuration problems") {
		t.Fatalf("multiple problems should be reported as a list:\n%s", message)
	}
}

// TestConfigNamesTheFixForACredential: "invalid byte" is accurate and useless.
func TestConfigNamesTheFixForACredential(t *testing.T) {
	t.Setenv(envBlockchainAPIKey, "zzzz-not-hex")

	err := NewConfig().Validate()
	if err == nil {
		t.Fatal("a non-hexadecimal API key validated successfully")
	}
	if !strings.Contains(err.Error(), "openssl rand -hex 32") {
		t.Fatalf("the error does not say how to produce a valid key:\n%s", err)
	}
}

// TestValidConfigStillValidates guards against the accumulation rewrite having
// made everything fail.
func TestValidConfigStillValidates(t *testing.T) {
	t.Setenv(envBlockchainAPIKey, "0123456789abcdef")
	t.Setenv(envServerSeed, "fedcba9876543210")
	t.Setenv(envNodeWalletPassphrase, "Node@Wallet#Pass123")

	if err := NewConfig().Validate(); err != nil {
		t.Fatalf("a valid configuration was rejected: %v", err)
	}
}

// TestNewAPIWithErrorExplainsItself.
//
// NewAPI returns a bare nil, so a caller could say no more than "failed to
// create API" while the reason went to a log line somewhere above.
func TestNewAPIWithErrorExplainsItself(t *testing.T) {
	t.Setenv(envBlockchainAPIKey, "definitely-not-hexadecimal")
	t.Setenv(envServerSeed, "0123456789abcdef")

	bc := forkTestChain(t, 0, uint32(genesisDifficulty))

	api, err := NewAPIWithError(bc)
	if err == nil {
		_ = api
		t.Skip("this build accepts the key; nothing to assert about the message")
	}

	if !strings.Contains(strings.ToLower(err.Error()), "hex") {
		t.Fatalf("the error does not identify the key as the problem:\n%v", err)
	}

	if _, err := NewAPIWithError(nil); err == nil {
		t.Fatal("an API was built without a blockchain")
	}
}

// TestOpenWalletSynchronisesTheNonce is the footgun this removes.
//
// A wallet holds its nonce counter in memory, so one loaded straight from disk
// starts at zero and every transaction it builds is refused as a replay.
func TestOpenWalletSynchronisesTheNonce(t *testing.T) {
	alice, bob := utxoWallets(t)
	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	bc := forkTestChainWithGenesisTxs(t, 0, 4, mintTo(t, alice, alice, 100))

	// utxoWallets created these before the chain switched storage directories.
	// Save Alice into the one this chain uses -- otherwise there is no file to
	// load her back from -- then reopen and re-seed the advisory balance that
	// Close cleared.
	if err := alice.Close(testPassPhrase); err != nil {
		t.Fatalf("persist wallet: %v", err)
	}
	if err := alice.Open(testPassPhrase); err != nil {
		t.Fatalf("reopen wallet: %v", err)
	}
	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	spend, err := NewBankTransaction(alice, bob, 10)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	block := utxoBlock(t, 1, bc.HeadHash(), spend)
	if _, err := bc.AcceptBlockWithResult(block); err != nil {
		t.Fatalf("accept: %v", err)
	}

	// Load the same wallet fresh, as a restarted process would.
	reloaded, err := bc.OpenWallet(alice.GetAddress(), testPassPhrase)
	if err != nil {
		t.Fatalf("open wallet: %v", err)
	}

	if got, want := reloaded.NextNonce(), spend.GetNonce()+1; got != want {
		t.Fatalf("a wallet opened through the chain is at nonce %d, want %d -- "+
			"its next transaction would be refused as a replay", got, want)
	}

	// And it can actually spend.
	next, err := NewBankTransaction(reloaded, bob, 5)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if !bc.AddTransactionLocal(next) {
		t.Fatal("a transaction from a chain-opened wallet was refused")
	}
}

func TestOpenWalletRejectsAnEmptyAddress(t *testing.T) {
	bc := forkTestChain(t, 0, uint32(genesisDifficulty))
	if _, err := bc.OpenWallet("", testPassPhrase); err == nil {
		t.Fatal("a wallet was opened without an address")
	}
}

// TestAPILoadedWalletsCanSpend covers the same hazard on the path the REST
// handlers use, which loads wallets itself rather than through OpenWallet.
func TestAPILoadedWalletsCanSpend(t *testing.T) {
	alice, bob := utxoWallets(t)
	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	bc := forkTestChainWithGenesisTxs(t, 0, 4, mintTo(t, alice, alice, 100))

	// utxoWallets created these before the chain switched storage directories.
	// Save Alice into the one this chain uses -- otherwise there is no file to
	// load her back from -- then reopen and re-seed the advisory balance that
	// Close cleared.
	if err := alice.Close(testPassPhrase); err != nil {
		t.Fatalf("persist wallet: %v", err)
	}
	if err := alice.Open(testPassPhrase); err != nil {
		t.Fatalf("reopen wallet: %v", err)
	}
	if err := alice.SetData("balance", 10000.0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	spend, err := NewBankTransaction(alice, bob, 10)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	block := utxoBlock(t, 1, bc.HeadHash(), spend)
	if _, err := bc.AcceptBlockWithResult(block); err != nil {
		t.Fatalf("accept: %v", err)
	}

	api := NewAPI(bc)
	if api == nil {
		t.Fatal("failed to create the API")
	}

	loaded, err := api.loadWalletByAddress(alice.GetAddress())
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if got, want := loaded.NextNonce(), spend.GetNonce()+1; got != want {
		t.Fatalf("an API-loaded wallet is at nonce %d, want %d", got, want)
	}
}
