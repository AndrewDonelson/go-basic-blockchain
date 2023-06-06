// file: sdk/wallet.go
// package: sdk
// description: Wallet represents a user's wallet.
package sdk

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/pborman/uuid"
	"github.com/xdg-go/pbkdf2"
)

var RequiredWalletProperties = []string{
	"name",
	"tags",
	"balance",
}

// Wallet represents a user's wallet.
type Wallet struct {
	ID                  string
	Address             string
	PrivateKey          *ecdsa.PrivateKey
	PublicKey           *ecdsa.PublicKey
	Encrypted           bool                   // Flag to indicate if the private key is encrypted
	EncryptedPrivateKey []byte                 // Encrypted private key
	EncryptionParams    *EncryptionParams      // Encryption parameters for the private key
	data                map[string]interface{} // Data (keypairs) associated with the wallet
	// TODO: Should I move Private & Public Keys in to the data map?
}

// EncryptionParams holds the encryption parameters for the private key.
type EncryptionParams struct {
	SaltSize  int // Size of the salt used for key derivation
	NonceSize int // Size of the nonce used for encryption
}

func NewEncryptionParams(saltSize, nonceSize int) *EncryptionParams {
	return &EncryptionParams{
		SaltSize:  saltSize,
		NonceSize: nonceSize,
	}
}

func NewDefaultEncryptionParams() *EncryptionParams {
	return NewEncryptionParams(saltSize, maxNonce)
}

// NewWallet creates a new wallet with a unique ID, name, and set of tags.
func NewWallet(name string, tags []string) (*Wallet, error) {
	// Generate a new private key.
	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}

	// Create a new wallet with a unique ID, name, and set of tags.
	wallet := &Wallet{
		ID:               uuid.New(),
		PrivateKey:       privateKey,
		PublicKey:        &privateKey.PublicKey,
		Address:          "",
		Encrypted:        false,
		EncryptionParams: NewDefaultEncryptionParams(),
	}
	wallet.SetData("name", name)
	wallet.SetData("tags", tags)
	wallet.SetData("balance", fundWalletAmount)
	wallet.GetAddress()

	fmt.Printf("[%s] Created new Wallet: %+v\n", time.Now().Format(logDateTimeFormat), PrettyPrint(wallet))

	return wallet, nil
}

// SetData sets the data (keypairs) associated with the wallet.
// This wallet allows the user to store arbitrary data (keypairs) in the wallet.
// The data included built-in data such as the wallet name, tags, and balance.
func (w *Wallet) SetData(key string, value interface{}) error {
	if w.Encrypted {
		return errors.New("cannot set data on an encrypted wallet")
	}

	w.data[key] = value

	return nil
}

// GetData returns the data (keypairs) associated with the wallet.
// This wallet allows the user to store arbitrary data (keypairs) in the wallet.
// The data included built-in data such as the wallet name, tags, and balance.
func (w *Wallet) GetData(key string) (interface{}, error) {
	if w.Encrypted {
		return nil, errors.New("cannot get data from an encrypted wallet")
	}

	return w.data[key], nil
}

// GetWalletName returns the wallet name from the data (keypairs) associated with the wallet.
func (w *Wallet) GetWalletName() string {
	if w.Encrypted {
		return ""
	}

	name, err := w.GetData("name")
	if err != nil {
		fmt.Println(err)
		return ""
	}

	return name.(string)
}

// GetBalance returns the wallet balance from the data (keypairs) associated with the wallet.
func (w *Wallet) GetBalance() float64 {
	if w.Encrypted {
		return 0
	}

	balance, err := w.GetData("balance")
	if err != nil {
		fmt.Println(err)
		return 0
	}

	return balance.(float64)
}

// GetTags returns the wallet tags from the data (keypairs) associated with the wallet.
func (w *Wallet) GetTags() []string {
	if w.Encrypted {
		return nil
	}

	tags, err := w.GetData("tags")
	if err != nil {
		fmt.Println(err)
		return nil
	}

	return tags.([]string)
}

// GetAddress generates and returns the wallet address.
func (w *Wallet) GetAddress() string {
	// If the address is already generated, return it.
	if w.Address != "" {
		return w.Address
	}

	// Generate an address by hashing the public key and encoding it in hexadecimal.
	pubBytes, err := w.PublicBytes()
	if err != nil {
		fmt.Printf("[%s] Error getting public key bytes: %s\n", time.Now().Format(logDateTimeFormat), err)
		return ""
	}

	hash := sha256.Sum256(pubBytes)
	w.Address = hex.EncodeToString(hash[:])

	return w.Address
}

// dataToBytes is an internal (private) method that converts the data (keypairs) associated with the wallet to bytes.
// this is used by the wallet to encrypt the data (keypairs) associated with the wallet.
func (w *Wallet) dataToBytes() ([]byte, error) {
	return json.Marshal(w.data)
}

// bytesToData is an internal (private) method that converts the bytes representation of the data (keypairs) associated with the wallet to the data (keypairs) associated with the wallet.
// this is used by the wallet to decrypt the data (keypairs) associated with the wallet.
func (w *Wallet) bytesToData(bytes []byte) error {
	return json.Unmarshal(bytes, &w.data)
}

// PrivateBytes returns the bytes representation of the private key.
func (w *Wallet) PrivateBytes() ([]byte, error) {
	if w.PrivateKey == nil {
		return nil, errors.New("public key is nil")
	}

	bytes, err := x509.MarshalECPrivateKey(w.PrivateKey)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// PublicBytes returns the bytes representation of the public key.
func (w *Wallet) PublicBytes() ([]byte, error) {
	if w.PublicKey == nil {
		return nil, errors.New("public key is nil")
	}

	bytes, err := x509.MarshalPKIXPublicKey(w.PublicKey)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// SendTransaction sends a new transaction from the sender's wallet to the recipient's address.
func (w *Wallet) SendTransaction(to string, tx Transaction, bc *Blockchain) (*Transaction, error) {
	if w.Encrypted {
		return nil, errors.New("cannot send transaction from an encrypted wallet")
	}

	// get the wallets balance
	balance := w.GetBalance()

	// Check if the wallet has enough balance.
	if balance < transactionFee {
		return nil, fmt.Errorf("insufficient funds")
	}

	fmt.Printf("[%s] Sending TX (%s): %+v\n", time.Now().Format(logDateTimeFormat), tx.GetProtocol(), tx)

	// Send the transaction to the network.
	err := tx.Send(bc)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}

	return &tx, nil
}

// SignTransaction signs the given transaction with the wallet's private key.
func (w *Wallet) SignTransaction(tx Transaction) error {

	if w.Encrypted {
		return errors.New("cannot sign transaction with an encrypted wallet")
	}

	// Get the SHA-256 hash of the transaction.
	txHash := sha256.Sum256([]byte(fmt.Sprintf("%v", tx)))

	// Sign the transaction hash.
	r, s, err := ecdsa.Sign(rand.Reader, w.PrivateKey, txHash[:])
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	signature := append(r.Bytes(), s.Bytes()...)
	err = tx.Sign(signature)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	return nil
}

// Lock locks the wallet using the provided passphrase. Basically the wallet's private key & data (keypairs) are encrypted using the passphrase.
func (w *Wallet) Lock(passphrase string) error {
}

// Unlock unlocks the wallet using the provided passphrase. Basically the wallet's private key & data (keypairs) are decrypted using the passphrase.
func (w *Wallet) Unlock(passphrase string) error {

}

// EncryptPrivateKey encrypts the wallet's private key using the provided passphrase.
func (w *Wallet) EncryptPrivateKey(passphrase string) error {
	if w.Encrypted {
		return errors.New("private key is already encrypted")
	}

	// Derive a symmetric encryption key from the passphrase.
	key := w.deriveKey([]byte(passphrase))

	// Generate a random nonce.
	nonce := make([]byte, w.EncryptionParams.NonceSize)
	_, err := rand.Read(nonce)
	if err != nil {
		return fmt.Errorf("failed to generate nonce: %v", err)
	}

	// Create a new AES-GCM cipher block using the derived key.
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher block: %v", err)
	}

	// Encrypt the private key.
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create AES-GCM cipher: %v", err)
	}

	pvtBytes, err := w.PrivateBytes()
	if err != nil {
		return fmt.Errorf("failed to get private key bytes: %v", err)
	}

	ciphertext := aesgcm.Seal(nil, nonce, pvtBytes, nil)

	// Update the wallet with the encrypted private key and encryption parameters.
	w.EncryptedPrivateKey = append(nonce, ciphertext...)
	w.Encrypted = true
	w.EncryptionParams.NonceSize = len(nonce)
	w.EncryptionParams.SaltSize = len(key)

	return nil
}

// DecryptPrivateKey decrypts the wallet's private key using the provided passphrase.
func (w *Wallet) DecryptPrivateKey(passphrase string) error {
	if !w.Encrypted {
		return errors.New("private key is not encrypted")
	}

	// Derive the symmetric encryption key from the passphrase.
	key := w.deriveKey([]byte(passphrase))

	// Create a new AES-GCM cipher block using the derived key.
	block, err := aes.NewCipher(key)
	if err != nil {
		return fmt.Errorf("failed to create cipher block: %v", err)
	}

	// Get the nonce and ciphertext bytes.
	nonceSize := w.EncryptionParams.NonceSize
	if len(w.EncryptedPrivateKey) < nonceSize {
		return errors.New("encrypted private key bytes are incomplete")
	}

	nonceBytes := w.EncryptedPrivateKey[:nonceSize]
	ciphertext := w.EncryptedPrivateKey[nonceSize:]

	// Decrypt the private key.
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return fmt.Errorf("failed to create AES-GCM cipher: %v", err)
	}

	plaintext, err := aesgcm.Open(nil, nonceBytes, ciphertext, nil)
	if err != nil {
		return fmt.Errorf("failed to decrypt private key: %v", err)
	}

	// Parse the decrypted private key.
	parsedPrivateKey, err := x509.ParseECPrivateKey(plaintext)
	if err != nil {
		return fmt.Errorf("failed to parse decrypted private key: %v", err)
	}

	// Update the wallet with the decrypted private key and encryption parameters.
	w.PrivateKey = parsedPrivateKey
	w.Encrypted = false
	w.EncryptionParams = NewDefaultEncryptionParams()

	return nil
}

func (w *Wallet) deriveKey(passphrase []byte) []byte {
	// Use a key derivation function (KDF) to derive a symmetric encryption key from the passphrase.
	// You can use a suitable KDF, such as PBKDF2 or bcrypt, to derive the key. Here's an example using PBKDF2:

	// Generate a salt.
	salt := make([]byte, w.EncryptionParams.SaltSize)
	_, err := rand.Read(salt)
	if err != nil {
		panic(fmt.Errorf("failed to generate salt: %v", err))
	}

	// Derive the key using PBKDF2 with SHA-256.
	key := pbkdf2.Key(passphrase, salt, 100000, 32, sha256.New)

	return key
}
