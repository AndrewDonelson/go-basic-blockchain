// Package sdk is a software development kit for building blockchain applications.
// File sdk/vault.go - Vault for all Vault related Protocol based transactions
package sdk

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"time"
)

// PEM is a struct that holds the PEM encoded private and public keys for a cryptographic key pair.
// The PrivateKey field contains the PEM encoded private key, and the PublicKey field
// contains the PEM encoded public key.
type PEM struct {
	PrivateKey string
	PublicKey  string
}

// NewPEM creates a new PEM struct containing the PEM-encoded private and public keys
// for the provided ECDSA private key.
func NewPEM(key *ecdsa.PrivateKey) *PEM {
	pem := &PEM{}
	pem.PrivateKey, pem.PublicKey = pem.Encode(key, &key.PublicKey)
	return pem
}

// Encode encodes the provided ECDSA private and public keys into PEM format.
// The function returns the PEM-encoded private key and public key as strings.
// The private key is encoded using the "PRIVATE KEY" PEM block type, and the
// public key is encoded using the "PUBLIC KEY" PEM block type.
func (p *PEM) Encode(privateKey *ecdsa.PrivateKey, publicKey *ecdsa.PublicKey) (string, string) {
	x509Encoded, _ := x509.MarshalECPrivateKey(privateKey)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	x509EncodedPub, _ := x509.MarshalPKIXPublicKey(publicKey)
	pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})

	return string(pemEncoded), string(pemEncodedPub)
}

// Decode decodes the private and public keys from PEM format. It takes the PEM-encoded
// private and public keys as input, and returns the corresponding ECDSA private and
// public keys. The function first decodes the PEM-encoded private key, then decodes
// the PEM-encoded public key, and returns both the private and public keys.
// Decode decodes the private and public keys from PEM format.
//
// Every step is checked. The previous version dereferenced the results of
// pem.Decode without a nil check (a panic on any malformed PEM) and did an
// unchecked type assertion on the parsed public key (a panic on any non-ECDSA
// key), while discarding all four errors.
func (p *PEM) Decode(pemEncoded string, pemEncodedPub string) (*ecdsa.PrivateKey, *ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(pemEncoded))
	if block == nil {
		return nil, nil, errors.New("failed to decode PEM block containing the private key")
	}
	privateKey, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse EC private key: %w", err)
	}

	blockPub, _ := pem.Decode([]byte(pemEncodedPub))
	if blockPub == nil {
		return nil, nil, errors.New("failed to decode PEM block containing the public key")
	}
	genericPublicKey, err := x509.ParsePKIXPublicKey(blockPub.Bytes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse public key: %w", err)
	}
	publicKey, ok := genericPublicKey.(*ecdsa.PublicKey)
	if !ok {
		return nil, nil, errors.New("public key is not an ECDSA key")
	}

	return privateKey, publicKey, nil
}

// GetPrivate returns the PEM encoded private key
func (p *PEM) GetPrivate() string {
	return p.PrivateKey
}

// GetPublic returns the PEM encoded public key
func (p *PEM) GetPublic() string {
	return p.PublicKey
}

// AsBytes returns the PEM encoded keys as bytes
func (p *PEM) AsBytes(s string) []byte {
	return []byte(s)
}

// Vault is a struct that holds the data (keypairs) associated with the wallet as well as the private key and PEM encoded keys
type Vault struct {
	ID       string                 `json:"id"`
	Name     string                 `json:"name"`
	Tags     []string               `json:"tags"`
	Balance  float64                `json:"balance"`
	Created  time.Time              `json:"created"`
	Modified time.Time              `json:"modified"`
	Data     map[string]interface{} // Data (keypairs) associated with the wallet
	Key      *ecdsa.PrivateKey      `json:"-"`
	Pem      *PEM
}

// RestoreKeyFromPEM reconstructs the ecdsa.PrivateKey from the PEM string after loading from disk
func (v *Vault) RestoreKeyFromPEM() error {
	if v.Pem == nil || v.Pem.PrivateKey == "" {
		return fmt.Errorf("PEM or private key PEM is empty")
	}
	block, _ := pem.Decode([]byte(v.Pem.PrivateKey))
	if block == nil {
		return fmt.Errorf("failed to decode PEM block")
	}
	key, err := x509.ParseECPrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse EC private key: %v", err)
	}
	v.Key = key
	return nil
}

// NewVault creates a new Vault struct
func NewVault() *Vault {
	LogInfof("Creating new Vault")
	newVault := &Vault{
		Data: make(map[string]interface{}),
		Key:  nil,
		Pem:  nil,
	}

	// Generate a new private key.
	LogVerbosef("Generating new keypair")
	err := newVault.NewKeyPair()
	if err != nil {
		return nil
	}

	return newVault
}

// NewVaultWithData creates a new vault with the given name, tags, and initial balance.
func NewVaultWithData(name string, tags []string, balance float64) *Vault {
	vault := &Vault{
		ID:       NewPUIDEmpty().String(),
		Name:     name,
		Tags:     tags,
		Balance:  balance,
		Created:  time.Now(),
		Modified: time.Now(),
		Data:     make(map[string]interface{}),
	}

	// Set initial data. SetData only fails on a nil vault, which cannot happen
	// here, but the errors are surfaced rather than silently discarded.
	for key, value := range map[string]interface{}{"name": name, "tags": tags, "balance": balance} {
		if err := vault.SetData(key, value); err != nil {
			LogInfof("failed to seed vault field %q: %v", key, err)
		}
	}

	return vault
}

// SetData sets the data (keypairs) associated with the wallet.
// This wallet allows the user to store arbitrary data (keypairs) in the wallet.
// The data included built-in data such as the wallet name, tags, and balance.
func (v *Vault) SetData(key string, value interface{}) error {
	if v == nil {
		LogInfof("Vault is nil")
		return nil
	}

	if v.Data == nil {
		v.Data = make(map[string]interface{})
	}

	if verbose {
		LogVerbosef("Setting data: %s to %v", key, value)
	}

	v.Data[key] = value
	return nil
}

// GetData returns a value from the vault's key/value store.
//
// A missing key is now an error. Returning (nil, nil) meant callers such as
// GetWalletName did `value.(string)` on a nil interface and panicked.
func (v *Vault) GetData(key string) (interface{}, error) {
	if v == nil || v.Data == nil {
		return nil, fmt.Errorf("vault has no data")
	}

	value, ok := v.Data[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found in vault", key)
	}
	return value, nil
}

// NewKeyPair creates a new keypair for the wallet
// NewKeyPair creates a new P-256 keypair for the wallet.
//
// The previous version looped over P-256/P-384/P-521 "in order of preference".
// That fallback is unreachable -- ecdsa.GenerateKey on P-256 fails only if the
// system entropy source is broken, in which case the other curves fail too -- and
// the loop shadowed err, so the final error message always formatted the outer
// nil as "%!v(<nil>)".
func (v *Vault) NewKeyPair() error {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return fmt.Errorf("failed to generate P-256 keypair: %w", err)
	}

	v.Key = key
	v.Pem = NewPEM(key)
	return nil
}

// PrivatePEM returns the PEM-encoded private key, or "" when there is none.
func (v *Vault) PrivatePEM() string {
	if v == nil || v.Pem == nil {
		return ""
	}
	return v.Pem.GetPrivate()
}

// PublicPEM returns the PEM-encoded public key, or "" when there is none.
func (v *Vault) PublicPEM() string {
	if v == nil || v.Pem == nil {
		return ""
	}
	return v.Pem.GetPublic()
}
