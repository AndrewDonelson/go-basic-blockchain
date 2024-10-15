// Package sdk is a software development kit for building blockchain applications.
// File sdk/vault.go - Vault for all Vault related Protocol based transactions
package sdk

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"encoding/pem"
	"log"
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
	Data map[string]interface{} // Data (keypairs) associated with the wallet
	Key  *ecdsa.PrivateKey
	Pem  *PEM
}

// NewVault creates a new Vault struct
func NewVault() *Vault {
	log.Printf("Creating new Vault\n")
	newVault := &Vault{
		Data: make(map[string]interface{}),
		Key:  nil,
		Pem:  nil,
	}

	// Generate a new private key.
	log.Printf("Generating new keypair\n")
	err := newVault.NewKeyPair()
	if err != nil {
		return nil
	}

	return newVault
}

func NewVaultWithData(name string, tags []string, balance float64) *Vault {
	newVault := NewVault()
	newVault.SetData("name", name)
	newVault.SetData("tags", tags)
	newVault.SetData("balance", balance)
	return newVault
}

// SetData sets the data (keypairs) associated with the wallet.
// This wallet allows the user to store arbitrary data (keypairs) in the wallet.
// The data included built-in data such as the wallet name, tags, and balance.
func (v *Vault) SetData(key string, value interface{}) error {
	if v == nil {
		log.Fatalln("Vault is nil")
		return nil
	}

	if v.Data == nil {
		if verbose {
			log.Fatalln("Vault Data is nil")
		}
		return nil
	}

	if verbose {
		log.Printf("Setting data: %s to %v\n", key, value)
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
	v.Key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	v.Pem = NewPEM(v.Key)
	return nil
}

// PrivatePEM returns the private key
func (v *Vault) PrivatePEM() string {
	return v.Pem.GetPrivate()
}

// PublicPEM returns the public key
func (v *Vault) PublicPEM() string {
	return v.Pem.GetPublic()
}
