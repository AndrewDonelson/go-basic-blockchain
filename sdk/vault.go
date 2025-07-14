// Package sdk is a software development kit for building blockchain applications.
// File sdk/vault.go - Vault for all Vault related Protocol based transactions
package sdk

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
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
func (p *PEM) Decode(pemEncoded string, pemEncodedPub string) (*ecdsa.PrivateKey, *ecdsa.PublicKey) {
	block, _ := pem.Decode([]byte(pemEncoded))
	x509Encoded := block.Bytes
	privateKey, _ := x509.ParseECPrivateKey(x509Encoded)

	blockPub, _ := pem.Decode([]byte(pemEncodedPub))
	x509EncodedPub := blockPub.Bytes
	genericPublicKey, _ := x509.ParsePKIXPublicKey(x509EncodedPub)
	publicKey := genericPublicKey.(*ecdsa.PublicKey)

	return privateKey, publicKey
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

	// Set initial data
	vault.SetData("name", name)
	vault.SetData("tags", tags)
	vault.SetData("balance", balance)

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

// GetData returns the data (keypairs) associated with the wallet.
// This wallet allows the user to store arbitrary data (keypairs) in the wallet.
// The data included built-in data such as the wallet name, tags, and balance.
func (v *Vault) GetData(key string) (interface{}, error) {
	return v.Data[key], nil
}

// NewKeyPair creates a new keypair for the wallet
func (v *Vault) NewKeyPair() (err error) {
	// Try different elliptic curves in order of preference
	curves := []elliptic.Curve{
		elliptic.P256(),
		elliptic.P384(),
		elliptic.P521(),
	}
	curveNames := []string{"P-256", "P-384", "P-521"}
	for i, curve := range curves {
		key, err := ecdsa.GenerateKey(curve, rand.Reader)
		if err == nil {
			LogVerbosef("Successfully generated key with curve %s", curveNames[i])
			v.Key = key
			v.Pem = NewPEM(key)
			return nil
		}
		LogVerbosef("Failed to generate key with curve %s: %v", curveNames[i], err)
	}
	// If all curves fail, do not set v.Key or v.Pem
	return fmt.Errorf("failed to generate keypair with any supported curve: %v", err)
}

// PrivatePEM returns the private key
func (v *Vault) PrivatePEM() string {
	return v.Pem.GetPrivate()
}

// PublicPEM returns the public key
func (v *Vault) PublicPEM() string {
	return v.Pem.GetPublic()
}
