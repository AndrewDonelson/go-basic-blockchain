// file: sdk/wallet.go
// package: sdk
// description: Wallet represents a user's wallet.
package sdk

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/pborman/uuid"
	"golang.org/x/crypto/scrypt"
)

var RequiredWalletProperties = []string{
	"name",
	"tags",
	"balance",
	"public_key",
	"private_key",
}

// Wallet represents a user's wallet.
type Wallet struct {
	ID               string
	Address          string
	Encrypted        bool                   // Flag to indicate if the private key is encrypted
	EncryptionParams *EncryptionParams      // Encryption parameters for the private key
	data             map[string]interface{} // Data (keypairs) associated with the wallet
	ciphertext       []byte                 // Encrypted data
	vault            *Vault
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
func NewWallet(name string, passphrase string, tags []string) (*Wallet, error) {

	// Create a new wallet with a unique ID, name, and set of tags.
	wallet := &Wallet{
		ID:               uuid.New(),
		Address:          "",
		Encrypted:        false,
		EncryptionParams: NewDefaultEncryptionParams(),
		vault:            NewVault(),
	}

	// Generate a new private key.
	err := wallet.vault.NewKeyPair()
	if err != nil {
		return nil, err
	}

	wallet.SetData("name", name)
	wallet.SetData("tags", tags)
	wallet.SetData("balance", fundWalletAmount)
	wallet.GetAddress()

	fmt.Printf("[%s] Created new Wallet: %+v\n", time.Now().Format(logDateTimeFormat), PrettyPrint(wallet))

	// close & save the new wallet
	wallet.Close(passphrase)

	// open & load the new wallet
	wallet.Open(passphrase)

	return wallet, nil
}

// SetData sets the data (keypairs) associated with the wallet.
// This wallet allows the user to store arbitrary data (keypairs) in the wallet.
// The data included built-in data such as the wallet name, tags, and balance.
func (w *Wallet) SetData(key string, value interface{}) error {
	if w.Encrypted {
		return errors.New("cannot set data on an encrypted wallet")
	}

	err := w.vault.SetData(key, value)
	if err != nil {
		return err
	}

	return nil
}

// GetData returns the data (keypairs) associated with the wallet.
// This wallet allows the user to store arbitrary data (keypairs) in the wallet.
// The data included built-in data such as the wallet name, tags, and balance.
func (w *Wallet) GetData(key string) (interface{}, error) {
	if w.Encrypted {
		return nil, errors.New("cannot get data from an encrypted wallet")
	}

	value, err := w.vault.GetData(key)
	if err != nil {
		return nil, err
	}

	return value, nil
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
func (w *Wallet) vaultToBytes() ([]byte, error) {
	return json.Marshal(w.vault)
}

// bytesToData is an internal (private) method that converts the bytes representation of the data (keypairs) associated with the wallet to the data (keypairs) associated with the wallet.
// this is used by the wallet to decrypt the data (keypairs) associated with the wallet.
func (w *Wallet) bytesToVault(bytes []byte) error {
	return json.Unmarshal(bytes, &w.vault)
}

// PrivateKey returns the private key from the vault associated with the wallet.
func (w *Wallet) PrivateKey() (*ecdsa.PrivateKey, error) {
	if w.Encrypted {
		return nil, errors.New("cannot get private key from an encrypted wallet")
	}

	return w.vault.Key, nil
}

// PrivateBytes returns the bytes representation of the private key.
func (w *Wallet) PrivateBytes() ([]byte, error) {
	if w.Encrypted {
		return nil, errors.New("cannot get private key from an encrypted wallet")
	}

	if w.vault.Key == nil {
		return nil, errors.New("private key is nil")
	}

	bytes, err := x509.MarshalECPrivateKey(w.vault.Key)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// PublicKey returns the public key from the data (keypairs) associated with the wallet.
func (w *Wallet) PublicKey() (*ecdsa.PublicKey, error) {
	if w.Encrypted {
		return nil, errors.New("cannot get public key from an encrypted wallet")
	}

	if w.vault.Key.PublicKey == (ecdsa.PublicKey{}) {
		return nil, errors.New("public key is nil")
	}

	return &w.vault.Key.PublicKey, nil
}

// PublicBytes returns the bytes representation of the public key.
func (w *Wallet) PublicBytes() ([]byte, error) {
	if w.Encrypted {
		return nil, errors.New("cannot get public key from an encrypted wallet")
	}

	if w.vault.Key.Public() == nil {
		return nil, errors.New("public key is nil")
	}

	bytes, err := x509.MarshalPKIXPublicKey(w.vault.Key.Public())
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

	fmt.Printf("[%s] %s Signing %s-TX : %v\n", time.Now().Format(logDateTimeFormat), w.GetWalletName(), tx.GetProtocol(), tx)

	// Get the SHA-256 hash of the transaction.
	txHash := sha256.Sum256([]byte(fmt.Sprintf("%v", tx)))

	// key the Private key from the wallet's data (keypairs).
	key, err := w.PrivateKey()
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	if key == nil {
		return fmt.Errorf("failed to sign transaction: private key is nil")
	}

	// Sign the transaction hash.
	r, s, err := ecdsa.Sign(rand.Reader, key, txHash[:])
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

// encrypt is a private internal method that encrypts the data (keypairs) associated with the wallet.
func (w *Wallet) encrypt(key, data []byte) ([]byte, error) {
	key, salt, err := w.deriveKey(key, nil)
	if err != nil {
		return nil, err
	}

	blockCipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(blockCipher)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nonce, nonce, data, nil)

	ciphertext = append(ciphertext, salt...)

	return ciphertext, nil
}

// decrypt is a private internal method that decrypts the data (keypairs) associated with the wallet.
func (w *Wallet) decrypt(key, data []byte) ([]byte, error) {
	salt, data := data[len(data)-32:], data[:len(data)-32]

	key, _, err := w.deriveKey(key, salt)
	if err != nil {
		return nil, err
	}

	blockCipher, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(blockCipher)
	if err != nil {
		return nil, err
	}

	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

// deriveKey is a private internal method that derives a key from the provided password and salt.
func (w *Wallet) deriveKey(password, salt []byte) ([]byte, []byte, error) {
	if salt == nil {
		salt = make([]byte, 32)
		if _, err := rand.Read(salt); err != nil {
			return nil, nil, err
		}
	}

	key, err := scrypt.Key(password, salt, 1048576, 8, 1, 32)
	if err != nil {
		return nil, nil, err
	}

	return key, salt, nil
}

// Lock locks the wallet using the provided passphrase. Basically the wallet's data (keypairs), including the private key are
// encrypted using the passphrase.
func (w *Wallet) Lock(passphrase string) error {

	// Check if the wallet is already encrypted.
	if w.Encrypted {
		return errors.New("wallet is already encrypted")
	}

	// Check if the passphrase is strong enough.
	if testPasswordStrength(passphrase) != nil {
		return errors.New("password is too weak")
	}
	// Convert the passphrase to bytes.
	pwAsBytes := []byte(passphrase)

	// Get the wallet's data as bytes.
	dataAsbytes, err := w.vaultToBytes()
	if err != nil {
		return err
	}

	// Encrypt the wallet's data.
	w.ciphertext, err = w.encrypt(pwAsBytes, dataAsbytes)
	if err != nil {
		return err
	}

	w.data = nil
	w.Encrypted = true

	return nil
}

// Unlock unlocks the wallet using the provided passphrase. Basically the wallet's data (keypairs), including the private key are
// decrypted using the passphrase.
func (w *Wallet) Unlock(passphrase string) error {

	// Check if the wallet is already decrypted.
	if !w.Encrypted {
		return errors.New("wallet is already decrypted")
	}

	// Convert the passphrase to bytes.
	pwAsBytes := []byte(passphrase)

	// Decrypt the wallet's data.
	dataAsBytes, err := w.decrypt(pwAsBytes, w.ciphertext)
	if err != nil {
		return err
	}

	// Convert the wallet's data to a map.
	err = w.bytesToVault(dataAsBytes)
	if err != nil {
		return err
	}

	// Set the wallet's data.
	w.ciphertext = nil
	w.Encrypted = false

	return nil
}

// save encrypts and saves the wallet to disk as a JSON file.
func (w *Wallet) Close(passphrase string) error {
	if w.Encrypted {
		return errors.New("cannot save an already encrypted wallet")
	}

	err := w.Lock(passphrase)
	if err != nil {
		return fmt.Errorf("failed to save wallet: %v", err)
	}

	filename := fmt.Sprintf("%s/%s.json", dataFolder, w.GetAddress())
	file, _ := json.MarshalIndent(w, "", " ")

	err = ioutil.WriteFile(filename, file, 0644)
	if err != nil {
		return fmt.Errorf("failed to save wallet: %v", err)
	}

	fmt.Printf("[%s] Wallet saved to disk: %s\n", time.Now().Format(logDateTimeFormat), filename)

	return nil
}

// load loads and decrypts the wallet from disk.
func (w *Wallet) Open(passphrase string) error {
	filename := fmt.Sprintf("%s/%s.json", dataFolder, w.GetAddress())

	fileData, err := ioutil.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("failed to load wallet: %v", err)
	}

	err = json.Unmarshal(fileData, w)
	if err != nil {
		return fmt.Errorf("failed to load wallet: %v", err)
	}

	err = w.Unlock(passphrase)
	if err != nil {
		return fmt.Errorf("failed to load wallet: %v", err)
	}

	fmt.Printf("[%s] Wallet loaded from disk: %s\n", time.Now().Format(logDateTimeFormat), filename)

	return nil
}
