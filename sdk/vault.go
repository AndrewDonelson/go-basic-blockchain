// Package sdk is a software development kit for building blockchain applications.
// File sdk/vault.go - Vault for all Vault related Protocol based transactions
package sdk

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
)

// PEM is a struct that holds the PEM encoded private and public keys
type PEM struct {
	PrivateKey string
	PublicKey  string
}

// NewPEM creates a new PEM struct
func NewPEM(key *ecdsa.PrivateKey) *PEM {
	pem := &PEM{}
	pem.PrivateKey, pem.PublicKey = pem.Encode(key, &key.PublicKey)
	return pem
}

// Encode encodes the private and public keys into PEM format
func (p *PEM) Encode(privateKey *ecdsa.PrivateKey, publicKey *ecdsa.PublicKey) (string, string) {
	x509Encoded, _ := x509.MarshalECPrivateKey(privateKey)
	pemEncoded := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: x509Encoded})

	x509EncodedPub, _ := x509.MarshalPKIXPublicKey(publicKey)
	pemEncodedPub := pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: x509EncodedPub})

	return string(pemEncoded), string(pemEncodedPub)
}

// Decode decodes the private and public keys from PEM format
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

// Vault is a struct that holds the data (keypairs) associated with the wallet as well as the private key and PEM encoded keys
type Vault struct {
	Data map[string]interface{} // Data (keypairs) associated with the wallet
	Key  *ecdsa.PrivateKey
	Pem  *PEM
}

// NewVault creates a new Vault struct
func NewVault() *Vault {
	return &Vault{
		Data: make(map[string]interface{}),
		Key:  nil,
		Pem:  nil,
	}
}

// SetData sets the data (keypairs) associated with the wallet.
// This wallet allows the user to store arbitrary data (keypairs) in the wallet.
// The data included built-in data such as the wallet name, tags, and balance.
func (v *Vault) SetData(key string, value interface{}) error {
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
	v.Key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	v.Pem = NewPEM(v.Key)
	return nil
}
