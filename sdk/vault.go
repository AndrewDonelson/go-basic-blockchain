package sdk

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
)

type Vault struct {
	data map[string]interface{} // Data (keypairs) associated with the wallet
	Key  *ecdsa.PrivateKey
}

func NewVault() *Vault {
	return &Vault{
		data: make(map[string]interface{}),
		Key:  nil,
	}
}

// SetData sets the data (keypairs) associated with the wallet.
// This wallet allows the user to store arbitrary data (keypairs) in the wallet.
// The data included built-in data such as the wallet name, tags, and balance.
func (v *Vault) SetData(key string, value interface{}) error {
	v.data[key] = value
	return nil
}

// GetData returns the data (keypairs) associated with the wallet.
// This wallet allows the user to store arbitrary data (keypairs) in the wallet.
// The data included built-in data such as the wallet name, tags, and balance.
func (v *Vault) GetData(key string) (interface{}, error) {
	return v.data[key], nil
}

func (v *Vault) NewKeyPair() (err error) {
	v.Key, err = ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	return nil
}
