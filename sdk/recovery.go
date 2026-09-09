// Package sdk is a software development kit for building blockchain applications.
// File sdk/recovery.go - Deterministic wallet derivation and recovery from a
// BIP-39 mnemonic.
package sdk

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors"
	"fmt"
	"math/big"

	bip39 "github.com/tyler-smith/go-bip39"
)

// mnemonicKeyInfo domain-separates wallet key derivation from every other use of
// HKDF in this codebase, so the same seed can never produce the same bytes for
// two different purposes.
const mnemonicKeyInfo = "gbb/wallet/p256/v1"

// maxDerivationAttempts bounds the rejection-sampling loop in deriveP256Key.
//
// A candidate is rejected only when it lands outside [1, n-1]. For P-256 that is
// overwhelmingly unlikely on any single attempt, so this is a safety valve
// against an infinite loop, not an expected code path.
const maxDerivationAttempts = 256

// ErrInvalidMnemonic is returned when a recovery phrase fails its checksum.
var ErrInvalidMnemonic = errors.New("invalid mnemonic recovery phrase")

// ValidateMnemonic reports whether a recovery phrase is well formed.
//
// BIP-39 phrases carry a checksum, so a mistyped word is detectable rather than
// silently producing a different -- and empty -- wallet.
func ValidateMnemonic(mnemonic string) error {
	if mnemonic == "" {
		return ErrInvalidMnemonic
	}
	if !bip39.IsMnemonicValid(mnemonic) {
		return ErrInvalidMnemonic
	}
	return nil
}

// deriveP256Key derives an ECDSA P-256 private key deterministically from a seed.
//
// The existing DeriveKeyPair could not be used for wallets: it returns BIP-32
// secp256k1 material, while every wallet, signature and address in this project
// is ECDSA P-256. Handing its output to a wallet produces a key the chain cannot
// verify, which is why nothing called it.
//
// The scalar is drawn by HKDF and rejected unless it lands in [1, n-1]. Reducing
// a uniform value modulo n instead -- the obvious shortcut -- biases the low end
// of the range, and biased ECDSA nonces and keys are a well-worn way to leak a
// private key.
func deriveP256Key(seed []byte) (*ecdsa.PrivateKey, error) {
	if len(seed) == 0 {
		return nil, errors.New("cannot derive a key from an empty seed")
	}

	curve := elliptic.P256()
	n := curve.Params().N
	byteLen := (n.BitLen() + 7) / 8

	for attempt := 0; attempt < maxDerivationAttempts; attempt++ {
		info := fmt.Sprintf("%s/%d", mnemonicKeyInfo, attempt)
		material, err := hkdfSHA256(seed, nil, []byte(info), byteLen)
		if err != nil {
			return nil, fmt.Errorf("derive key material: %w", err)
		}

		d := new(big.Int).SetBytes(material)
		if d.Sign() <= 0 || d.Cmp(n) >= 0 {
			continue
		}

		key := &ecdsa.PrivateKey{D: d}
		key.PublicKey.Curve = curve
		key.PublicKey.X, key.PublicKey.Y = curve.ScalarBaseMult(d.Bytes())
		if key.PublicKey.X == nil {
			continue
		}
		return key, nil
	}

	return nil, errors.New("failed to derive a valid key from the seed")
}

// SeedFromMnemonic turns a recovery phrase and optional passphrase into a seed.
//
// The passphrase is BIP-39's "25th word": a different passphrase over the same
// phrase yields a completely different wallet. It is not the passphrase that
// encrypts the wallet file, and confusing the two loses funds, so callers should
// keep the distinction explicit.
func SeedFromMnemonic(mnemonic, passphrase string) ([]byte, error) {
	if err := ValidateMnemonic(mnemonic); err != nil {
		return nil, err
	}
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, passphrase)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidMnemonic, err)
	}
	return seed, nil
}

// NewWalletFromMnemonic recreates a wallet from a BIP-39 recovery phrase.
//
// The same phrase, seed passphrase and wallet identity always produce the same
// key and therefore the same address. That is what makes a lost wallet file
// recoverable: the file holds no secret the phrase cannot regenerate.
//
// options.Passphrase encrypts the wallet at rest and may be chosen freely on
// recovery; seedPassphrase is part of the key derivation and must match what was
// used originally, or a different -- and empty -- wallet comes back.
func NewWalletFromMnemonic(options *WalletOptions, mnemonic, seedPassphrase string) (*Wallet, error) {
	if options == nil {
		return nil, errors.New("options cannot be nil")
	}
	if testPasswordStrength(options.Passphrase) != nil {
		return nil, errors.New("password is too weak")
	}

	seed, err := SeedFromMnemonic(mnemonic, seedPassphrase)
	if err != nil {
		return nil, err
	}

	key, err := deriveP256Key(seed)
	if err != nil {
		return nil, err
	}

	LogInfof("Recovering wallet: %s", options.Name)
	wallet := &Wallet{
		ID:               NewPUID(options.OrganizationID, options.AppID, options.UserID, options.AssetID),
		Address:          "",
		Encrypted:        false,
		EncryptionParams: NewDefaultEncryptionParams(),
		vault:            NewVaultWithData(options.Name, options.Tags, 0),
		Ciphertext:       []byte{},
	}

	wallet.vault.Key = key
	wallet.vault.Pem = NewPEM(key)
	wallet.GetAddress()

	// The phrase is held in memory for the caller to display and is never
	// written to the wallet file -- a recovery phrase stored beside the wallet it
	// recovers protects nothing.
	wallet.mnemonic = mnemonic

	if err := wallet.Close(options.Passphrase); err != nil {
		return nil, fmt.Errorf("failed to save wallet: %w", err)
	}

	LogVerbosef("Wallet recovered: %s", wallet.GetAddress())
	return wallet, nil
}

// NewRecoverableWallet creates a wallet along with the recovery phrase that
// regenerates it.
//
// The phrase is returned rather than stored: it is the one secret that must
// leave the process, and writing it next to the wallet file would defeat the
// point. Call Wallet.Mnemonic() to read it back before the wallet is discarded.
func NewRecoverableWallet(options *WalletOptions) (*Wallet, string, error) {
	mnemonic, err := GenerateMnemonic()
	if err != nil {
		return nil, "", fmt.Errorf("generate recovery phrase: %w", err)
	}

	wallet, err := NewWalletFromMnemonic(options, mnemonic, "")
	if err != nil {
		return nil, "", err
	}
	return wallet, mnemonic, nil
}

// Mnemonic returns the recovery phrase for a wallet created or recovered in this
// process, or "" for one loaded from disk.
//
// It is deliberately not persisted, so this is empty for any wallet that was
// merely opened. There is no way to recover a phrase from a wallet file.
func (w *Wallet) Mnemonic() string {
	if w == nil {
		return ""
	}
	return w.mnemonic
}
