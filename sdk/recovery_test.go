package sdk

import (
	"crypto/elliptic"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

func recoveryOptions(name string) *WalletOptions {
	return NewWalletOptions(
		ThisBlockchainOrganizationID, ThisBlockchainAppID,
		ThisBlockchainAdminUserID, ThisBlockchainDevAssetID,
		name, testPassPhrase, []string{"recovery"})
}

// -----------------------------------------------------------------------------
// Derivation
// -----------------------------------------------------------------------------

// TestDerivationIsDeterministic is the property the whole feature rests on: if
// the same phrase did not reproduce the same key, recovery would silently hand
// back an empty wallet at a different address.
func TestDerivationIsDeterministic(t *testing.T) {
	mnemonic, err := GenerateMnemonic()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	seed, err := SeedFromMnemonic(mnemonic, "")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	first, err := deriveP256Key(seed)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	for i := 0; i < 10; i++ {
		again, err := deriveP256Key(seed)
		if err != nil {
			t.Fatalf("derive again: %v", err)
		}
		if first.D.Cmp(again.D) != 0 {
			t.Fatal("the same seed produced two different keys")
		}
	}
}

// TestDerivedKeyIsOnTheCurveAndInRange: a scalar outside [1, n-1] is not a usable
// private key, and reducing modulo n instead of resampling would bias the low end
// of the range.
func TestDerivedKeyIsOnTheCurveAndInRange(t *testing.T) {
	curve := elliptic.P256()
	n := curve.Params().N

	for i := 0; i < 25; i++ {
		mnemonic, err := GenerateMnemonic()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		seed, err := SeedFromMnemonic(mnemonic, "")
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		key, err := deriveP256Key(seed)
		if err != nil {
			t.Fatalf("derive: %v", err)
		}

		if key.D.Sign() <= 0 || key.D.Cmp(n) >= 0 {
			t.Fatalf("derived scalar is outside [1, n-1]")
		}
		if !curve.IsOnCurve(key.PublicKey.X, key.PublicKey.Y) {
			t.Fatal("derived public key is not on P-256")
		}
	}
}

// TestDerivationUsesP256NotSecp256k1 pins why DeriveKeyPair could not be reused.
func TestDerivationUsesP256NotSecp256k1(t *testing.T) {
	mnemonic, err := GenerateMnemonic()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	seed, err := SeedFromMnemonic(mnemonic, "")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	key, err := deriveP256Key(seed)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}

	if key.Curve != elliptic.P256() {
		t.Fatal("wallets are ECDSA P-256; a key on any other curve cannot sign " +
			"a transaction this chain will verify")
	}
}

// TestDifferentPhrasesGiveDifferentKeys.
func TestDifferentPhrasesGiveDifferentKeys(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 20; i++ {
		mnemonic, err := GenerateMnemonic()
		if err != nil {
			t.Fatalf("generate: %v", err)
		}
		seed, err := SeedFromMnemonic(mnemonic, "")
		if err != nil {
			t.Fatalf("seed: %v", err)
		}
		key, err := deriveP256Key(seed)
		if err != nil {
			t.Fatalf("derive: %v", err)
		}
		d := key.D.String()
		if seen[d] {
			t.Fatal("two different phrases derived the same key")
		}
		seen[d] = true
	}
}

// TestSeedPassphraseChangesTheWallet documents BIP-39's "25th word": the wrong
// seed passphrase silently yields a different, empty wallet rather than an error.
func TestSeedPassphraseChangesTheWallet(t *testing.T) {
	mnemonic, err := GenerateMnemonic()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}

	plain, err := SeedFromMnemonic(mnemonic, "")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	salted, err := SeedFromMnemonic(mnemonic, "a second factor")
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	a, err := deriveP256Key(plain)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	b, err := deriveP256Key(salted)
	if err != nil {
		t.Fatalf("derive: %v", err)
	}
	if a.D.Cmp(b.D) == 0 {
		t.Fatal("the seed passphrase had no effect on the derived key")
	}
}

func TestDeriveRejectsAnEmptySeed(t *testing.T) {
	if _, err := deriveP256Key(nil); err == nil {
		t.Fatal("an empty seed produced a key")
	}
	if _, err := deriveP256Key([]byte{}); err == nil {
		t.Fatal("an empty seed produced a key")
	}
}

// -----------------------------------------------------------------------------
// Mnemonic validation
// -----------------------------------------------------------------------------

// TestMnemonicChecksumCatchesTypos is why a phrase is not just random words: a
// mistyped word is detectable rather than producing a different empty wallet.
func TestMnemonicChecksumCatchesTypos(t *testing.T) {
	mnemonic, err := GenerateMnemonic()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if err := ValidateMnemonic(mnemonic); err != nil {
		t.Fatalf("a freshly generated phrase failed validation: %v", err)
	}

	words := strings.Fields(mnemonic)
	if len(words) != 12 {
		t.Fatalf("expected a 12-word phrase, got %d words", len(words))
	}

	// Fixed BIP-39 vectors rather than a substitution into a random phrase.
	//
	// Substituting one word into a generated phrase is only a 15-in-16 test: the
	// checksum is four bits, so one time in sixteen the altered phrase is still
	// valid and the assertion fails for no reason. These two differ only in the
	// last word, and the checksum is what separates them.
	const (
		validVector   = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon about"
		invalidVector = "abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon abandon"
	)

	if err := ValidateMnemonic(validVector); err != nil {
		t.Fatalf("a known-good BIP-39 vector failed validation: %v", err)
	}
	if err := ValidateMnemonic(invalidVector); err == nil {
		t.Fatal("a phrase whose only fault is its checksum passed validation; " +
			"a mistyped word would go undetected")
	}

	for _, bad := range []string{"", "not a real mnemonic at all", strings.Repeat("zoo ", 12)} {
		if err := ValidateMnemonic(bad); err == nil {
			t.Fatalf("an invalid phrase passed validation: %q", bad)
		}
	}
}

func TestSeedFromInvalidMnemonic(t *testing.T) {
	_, err := SeedFromMnemonic("clearly not a valid phrase", "")
	if !errors.Is(err, ErrInvalidMnemonic) {
		t.Fatalf("expected ErrInvalidMnemonic, got %v", err)
	}
}

// -----------------------------------------------------------------------------
// Wallet recovery
// -----------------------------------------------------------------------------

// TestWalletRecoversToTheSameAddress is the user-visible promise: the wallet file
// holds no secret the phrase cannot regenerate.
func TestWalletRecoversToTheSameAddress(t *testing.T) {
	original, mnemonic, err := NewRecoverableWallet(recoveryOptions("recover-me"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if mnemonic == "" {
		t.Fatal("no recovery phrase was returned")
	}

	// Simulate losing the wallet file entirely and recovering from the phrase.
	recovered, err := NewWalletFromMnemonic(recoveryOptions("recover-me"), mnemonic, "")
	if err != nil {
		t.Fatalf("recover: %v", err)
	}

	if recovered.GetAddress() != original.GetAddress() {
		t.Fatalf("recovery produced a different wallet: %s vs %s",
			recovered.GetAddress(), original.GetAddress())
	}
}

// TestRecoveredWalletCanSign proves the recovered key is actually usable, not
// merely equal at the address level.
func TestRecoveredWalletCanSign(t *testing.T) {
	_, mnemonic, err := NewRecoverableWallet(recoveryOptions("signer"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	recovered, err := NewWalletFromMnemonic(recoveryOptions("signer"), mnemonic, "")
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if err := recovered.Open(testPassPhrase); err != nil {
		t.Fatalf("open: %v", err)
	}

	other, err := NewWallet(recoveryOptions("counterparty"))
	if err != nil {
		t.Fatalf("counterparty: %v", err)
	}

	tx, err := NewTransaction(MessageProtocolID, recovered, other)
	if err != nil {
		t.Fatalf("tx: %v", err)
	}
	signature, err := tx.Sign([]byte(recovered.PrivatePEM()))
	if err != nil {
		t.Fatalf("sign with the recovered key: %v", err)
	}
	if signature == "" {
		t.Fatal("the recovered key produced an empty signature")
	}
}

// TestRecoveryWithTheWrongPhraseGivesADifferentWallet: the failure mode is a
// different address, not an error, which is why the checksum matters.
func TestRecoveryWithTheWrongPhraseGivesADifferentWallet(t *testing.T) {
	original, _, err := NewRecoverableWallet(recoveryOptions("original"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	other, err := GenerateMnemonic()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	wrong, err := NewWalletFromMnemonic(recoveryOptions("original"), other, "")
	if err != nil {
		t.Fatalf("recover: %v", err)
	}

	if wrong.GetAddress() == original.GetAddress() {
		t.Fatal("a different phrase recovered the same wallet")
	}
}

func TestRecoveryRejectsAnInvalidPhrase(t *testing.T) {
	_, err := NewWalletFromMnemonic(recoveryOptions("bad"), "these words are not valid", "")
	if !errors.Is(err, ErrInvalidMnemonic) {
		t.Fatalf("expected ErrInvalidMnemonic, got %v", err)
	}
}

func TestRecoveryRejectsAWeakWalletPassword(t *testing.T) {
	mnemonic, err := GenerateMnemonic()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	opts := recoveryOptions("weak")
	opts.Passphrase = "x"
	if _, err := NewWalletFromMnemonic(opts, mnemonic, ""); err == nil {
		t.Fatal("a weak wallet password was accepted")
	}
}

func TestRecoveryRejectsNilOptions(t *testing.T) {
	mnemonic, err := GenerateMnemonic()
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if _, err := NewWalletFromMnemonic(nil, mnemonic, ""); err == nil {
		t.Fatal("nil options were accepted")
	}
}

// TestTheRecoveryPhraseIsNotPersisted: a phrase stored beside the wallet it
// recovers protects nothing.
func TestTheRecoveryPhraseIsNotPersisted(t *testing.T) {
	wallet, mnemonic, err := NewRecoverableWallet(recoveryOptions("not-persisted"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if wallet.Mnemonic() != mnemonic {
		t.Fatal("the in-memory wallet does not carry the phrase it was created with")
	}

	encoded, err := json.Marshal(wallet)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if strings.Contains(string(encoded), mnemonic) {
		t.Fatal("the recovery phrase was serialised into the wallet")
	}

	// A wallet loaded from disk has no phrase to give.
	loaded := &Wallet{}
	if loaded.Mnemonic() != "" {
		t.Fatal("a wallet that was merely loaded reported a recovery phrase")
	}
	var nilWallet *Wallet
	if nilWallet.Mnemonic() != "" {
		t.Fatal("a nil wallet reported a recovery phrase")
	}
}

// TestEveryWalletIsRecoverable: recovery that is opt-in is recovery nobody has
// when they need it. NewWallet used to produce a raw random key with no way to
// reproduce it, which made a lost wallet file final.
func TestEveryWalletIsRecoverable(t *testing.T) {
	wallet, err := NewWallet(recoveryOptions("default-recoverable"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	phrase := wallet.Mnemonic()
	if phrase == "" {
		t.Fatal("a wallet from NewWallet has no recovery phrase, so losing its " +
			"file loses the funds")
	}
	if err := ValidateMnemonic(phrase); err != nil {
		t.Fatalf("the phrase NewWallet produced is not valid: %v", err)
	}

	recovered, err := NewWalletFromMnemonic(recoveryOptions("default-recoverable"), phrase, "")
	if err != nil {
		t.Fatalf("recover: %v", err)
	}
	if recovered.GetAddress() != wallet.GetAddress() {
		t.Fatalf("recovery gave a different address: %s vs %s",
			recovered.GetAddress(), wallet.GetAddress())
	}
}

// TestWalletsRemainUnique guards the obvious way deterministic derivation could
// go wrong: every wallet sharing one address.
func TestWalletsRemainUnique(t *testing.T) {
	seen := map[string]bool{}
	for i := 0; i < 15; i++ {
		w, err := NewWallet(recoveryOptions("unique"))
		if err != nil {
			t.Fatalf("create: %v", err)
		}
		addr := w.GetAddress()
		if seen[addr] {
			t.Fatal("two wallets share an address")
		}
		seen[addr] = true
	}
}
