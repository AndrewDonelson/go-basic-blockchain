// Package sdk is a software development kit for building blockchain applications.
// File sdk/wallet.go - Wallet for all Wallet related Protocol based transactions
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
	"path/filepath"
	"strings"
	"sync"

	"golang.org/x/crypto/scrypt"
)

// RequiredWalletProperties is a list of required properties for a wallet.
// This list defines the minimum set of properties that a wallet must have in order to be considered valid.
// The properties include the wallet name, tags, balance, public key, and private key.
var RequiredWalletProperties = []string{
	"name",
	"tags",
	"balance",
	"public_key",
	"private_key",
}

// WalletOptions is a struct that contains the required options for creating a new wallet.
//
// OrganizationID is the ID of the organization creating the wallet.
// AppID is the ID of the app creating the wallet.
// UserID is the ID of the user creating the wallet.
// AssetID is the ID of the asset creating the wallet.
// Name is the string name for the wallet.
// Passphrase is the passphrase for the wallet.
// Tags are the tags associated with the wallet.
type WalletOptions struct {
	OrganizationID *BigInt
	AppID          *BigInt
	UserID         *BigInt
	AssetID        *BigInt
	Name           string
	Passphrase     string
	Tags           []string
}

// NewWalletOptions creates a new WalletOptions struct with the provided parameters.
// The WalletOptions struct contains the necessary properties for creating a new wallet.
// The OrganizationID, AppID, UserID, and AssetID fields are pointers to BigInt values,
// representing the IDs of the organization, app, user, and asset associated with the wallet.
// The Name field is a string representing the name of the wallet.
// The Passphrase field is a string representing the passphrase for the wallet.
// The Tags field is a slice of strings representing the tags associated with the wallet.
func NewWalletOptions(organizationID, appID, userID, assetID *BigInt, name, passphrase string, tags []string) *WalletOptions {
	return &WalletOptions{
		OrganizationID: organizationID,
		AppID:          appID,
		UserID:         userID,
		AssetID:        assetID,
		Name:           name,
		Passphrase:     passphrase,
		Tags:           tags,
	}
}

// Wallet represents a user's wallet. Wallets are persisted to disk as individual files.
// The Wallet struct contains the following fields:
//
// ID: A unique identifier for the wallet.
// Address: The wallet's address.
// Encrypted: A flag indicating whether the private key is encrypted.
// EncryptionParams: The encryption parameters used to encrypt the private key.
// Ciphertext: The encrypted private key data.
// vault: A reference to the wallet's associated vault.
type Wallet struct {
	ID               *PUID
	Address          string
	Encrypted        bool
	EncryptionParams *EncryptionParams
	Ciphertext       []byte
	vault            *Vault
	// mnemonic is the BIP-39 recovery phrase for a wallet created or recovered in
	// this process. It is never serialised: a recovery phrase stored beside the
	// wallet it recovers protects nothing.
	mnemonic string
	// nextNonce is the sequence number the wallet's next transaction will use.
	//
	// It is held in memory rather than in the vault because the vault cannot be
	// written while the wallet is encrypted, and a transaction may be built from
	// a wallet that is only being read. A wallet loaded from disk therefore
	// starts at zero and must be resynchronised from the chain -- see
	// Blockchain.SyncWalletNonce -- or its transactions will be refused as
	// replays of nonces it already used.
	nextNonce uint64
	// mutex guards nextNonce. It was previously declared and unused.
	mutex sync.Mutex
}

// EncryptionParams holds the encryption parameters for the private key.
//
// The scrypt cost parameters are recorded per wallet. They used to be chosen at
// runtime from testing.Testing() and never written down, so a wallet created under
// test could not be opened in production (and vice versa) -- the KDF silently
// derived a different key and decryption failed with no explanation.
type EncryptionParams struct {
	SaltSize  int `json:"salt_size"`  // Size of the salt used for key derivation
	NonceSize int `json:"nonce_size"` // Size of the AES-GCM nonce
	ScryptN   int `json:"scrypt_n"`   // scrypt CPU/memory cost
	ScryptR   int `json:"scrypt_r"`   // scrypt block size
	ScryptP   int `json:"scrypt_p"`   // scrypt parallelisation
}

// scryptParams holds a scrypt cost profile.
type scryptParams struct{ N, R, P int }

var (
	// productionScryptParams is the cost profile used for real wallets.
	productionScryptParams = scryptParams{N: 1048576, R: 8, P: 1} // 2^20
	// fastScryptParams is a deliberately cheap profile used only by the test suite.
	fastScryptParams = scryptParams{N: 16384, R: 8, P: 1} // 2^14
	// defaultScryptParams is what new wallets are created with. Tests lower it via
	// an init() in the test build so the production code no longer imports "testing".
	defaultScryptParams = productionScryptParams
)

// NewEncryptionParams creates a new EncryptionParams struct with the specified salt and nonce sizes.
// The salt size and nonce size are used to configure the encryption parameters for a wallet's private key.
func NewEncryptionParams(saltSize, nonceSize int) *EncryptionParams {
	return &EncryptionParams{
		SaltSize:  saltSize,
		NonceSize: nonceSize,
		ScryptN:   defaultScryptParams.N,
		ScryptR:   defaultScryptParams.R,
		ScryptP:   defaultScryptParams.P,
	}
}

// NewDefaultEncryptionParams creates a new EncryptionParams struct with default values.
// The default salt size is 32 bytes and the default AES-GCM nonce size is 12 bytes.
func NewDefaultEncryptionParams() *EncryptionParams {
	return NewEncryptionParams(saltSize, gcmNonceSize)
}

// scrypt returns the cost profile recorded in these params, falling back to the
// production profile for wallets written before the parameters were persisted.
func (e *EncryptionParams) scrypt() scryptParams {
	if e == nil || e.ScryptN == 0 {
		return productionScryptParams
	}
	return scryptParams{N: e.ScryptN, R: e.ScryptR, P: e.ScryptP}
}

// NewWallet creates a new wallet with a unique ID, name, and set of tags.
// The wallet is initialized with a new private key and default encryption parameters.
// The wallet must be closed to save it to disk.
func NewWallet(options *WalletOptions) (*Wallet, error) {
	var err error

	if options == nil {
		return nil, errors.New("options cannot be nil")
	}

	// Check if the passphrase is strong enough.
	if testPasswordStrength(options.Passphrase) != nil {
		return nil, errors.New("password is too weak")
	}

	// Create a new wallet with a unique ID, name, and set of tags.
	LogInfof("Creating wallet: %s", options.Name)
	wallet := &Wallet{
		// options.AssetID was previously discarded and replaced with 0, so every
		// wallet in the system shared the identity <org>:<app>:<user>:0.
		ID:               NewPUID(options.OrganizationID, options.AppID, options.UserID, options.AssetID),
		Address:          "",
		Encrypted:        false,
		EncryptionParams: NewDefaultEncryptionParams(),
		// Balance starts at zero. Seeding it with fundWalletAmount created tokens
		// out of nothing on every wallet creation, and NewBankTransaction then
		// checked affordability against that fabricated number.
		vault:      NewVaultWithData(options.Name, options.Tags, 0),
		Ciphertext: []byte{},
	}

	// Generate a new private key from a BIP-39 recovery phrase, so every wallet is
	// recoverable by default.
	//
	// The key used to be raw ecdsa.GenerateKey output with no way to reproduce it,
	// which made a lost wallet file final. sdk/mnemonic.go existed but nothing
	// called it -- and could not have, since it derives secp256k1 material while
	// wallets are P-256. Call Wallet.Mnemonic() to show the phrase to the user
	// before the wallet goes out of scope; it is never written to disk.
	mnemonic, err := GenerateMnemonic()
	if err != nil {
		return nil, fmt.Errorf("generate recovery phrase: %w", err)
	}
	seed, err := SeedFromMnemonic(mnemonic, "")
	if err != nil {
		return nil, fmt.Errorf("derive seed: %w", err)
	}
	key, err := deriveP256Key(seed)
	if err != nil {
		return nil, fmt.Errorf("derive wallet key: %w", err)
	}
	wallet.vault.Key = key
	wallet.vault.Pem = NewPEM(key)
	wallet.mnemonic = mnemonic

	wallet.GetAddress()

	// if verbose {
	// 	LogVerbosef("Created new Wallet: %+v", PrettyPrint(wallet))
	// } else {
	// 	LogVerbosef("Created new Wallet: %s", wallet.GetAddress())
	// }

	LogVerbosef("Wallet created: %s", wallet.GetAddress())

	// Save the wallet after creation
	err = wallet.Close(options.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to save wallet: %w", err)
	}

	return wallet, nil
}

// ReserveNonce returns the nonce for the wallet's next transaction and advances
// the counter.
//
// The nonce is part of the signing payload, so it has to be settled before the
// transaction is signed -- which is why it is assigned at construction rather
// than at submission.
func (w *Wallet) ReserveNonce() uint64 {
	if w == nil {
		return 0
	}
	w.mutex.Lock()
	defer w.mutex.Unlock()

	nonce := w.nextNonce
	w.nextNonce++
	return nonce
}

// NextNonce reports the nonce the wallet's next transaction will use.
func (w *Wallet) NextNonce() uint64 {
	if w == nil {
		return 0
	}
	w.mutex.Lock()
	defer w.mutex.Unlock()
	return w.nextNonce
}

// SetNextNonce resets the counter, normally from chain state.
func (w *Wallet) SetNextNonce(nonce uint64) {
	if w == nil {
		return
	}
	w.mutex.Lock()
	defer w.mutex.Unlock()
	w.nextNonce = nonce
}

// SetData sets the data (keypairs) associated with the wallet.
// This method allows the user to store arbitrary data (keypairs) in the wallet.
// If the wallet is encrypted, this method will return an error.
func (w *Wallet) SetData(key string, value interface{}) error {
	if w.Encrypted {
		return errors.New("cannot set data on an encrypted wallet")
	}

	if key == "balance" {
		convertedValue, err := ConvertToFloat64(value)
		if err != nil {
			return fmt.Errorf("error converting balance: %v", err)
		}
		value = convertedValue
	}

	return w.vault.SetData(key, value)
}

// GetData returns the data (keypairs) associated with the wallet.
// This wallet allows the user to store arbitrary data (keypairs) in the wallet.
// The data included built-in data such as the wallet name, tags, and balance.
// If the wallet is encrypted, this method will return an error.
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
// If the wallet is encrypted, an empty string is returned. If there is an error
// retrieving the wallet name, an empty string is also returned.
func (w *Wallet) GetWalletName() string {
	if w.Encrypted {
		return ""
	}

	name, err := w.GetData("name")
	if err != nil {
		LogInfof("error reading wallet name: %v", err)
		return ""
	}

	str, ok := name.(string)
	if !ok {
		return ""
	}
	return str
}

// GetBalance returns the wallet balance from the data (keypairs) associated with the wallet.
// If the wallet is encrypted, this function will return 0.
// Otherwise, it will retrieve the "balance" key from the wallet data and return it as a float64.
// If there is an error retrieving the balance, it will log the error and return 0.
func (w *Wallet) GetBalance() float64 {
	if w.Encrypted {
		return 0
	}

	balance, err := w.GetData("balance")
	if err != nil {
		LogVerbosef("wallet data error: %v", err)
		return 0
	}

	convertedBalance, err := ConvertToFloat64(balance)
	if err != nil {
		LogVerbosef("Error converting balance: %v", err)
		return 0
	}
	return convertedBalance
}

// GetTags returns the wallet tags from the data (keypairs) associated with the wallet.
// If the wallet is encrypted, this function will return nil.
// Otherwise, it will return the tags stored in the wallet data, or nil if there is an error retrieving the tags.
func (w *Wallet) GetTags() []string {
	if w.Encrypted {
		return nil
	}
	tags, err := w.GetData("tags")
	if err != nil {
		LogVerbosef("wallet data error: %v", err)
		return nil
	}
	// Handle both []string and []interface{} (from JSON)
	switch v := tags.(type) {
	case []string:
		return v
	case []interface{}:
		strs := make([]string, len(v))
		for i, val := range v {
			str, ok := val.(string)
			if ok {
				strs[i] = str
			}
		}
		return strs
	default:
		return nil
	}
}

// GetAddress generates and returns the wallet address.
//
// If the address is already generated, it returns the cached address.
// Otherwise, it generates a new address by hashing the public key and encoding it in hexadecimal.
func (w *Wallet) GetAddress() string {
	// If the address is already generated, return it.
	if w.Address != "" {
		return w.Address
	}

	// Generate an address by hashing the public key and encoding it in hexadecimal.
	pubBytes, err := w.PublicBytes()
	if err != nil {
		LogVerbosef("Error getting public key bytes: %s", err)
		return ""
	}

	hash := sha256.Sum256(pubBytes)
	w.Address = hex.EncodeToString(hash[:])

	return w.Address
}

// vaultToBytes is an internal (private) method that converts the wallet's vault data (keypairs) to bytes.
// This is used by the wallet to encrypt the data (keypairs) associated with the wallet.
func (w *Wallet) vaultToBytes() ([]byte, error) {
	return json.Marshal(w.vault)
}

// bytesToData is an internal (private) method that converts the bytes representation of the data (keypairs) associated with the wallet to the data (keypairs) associated with the wallet.
// this is used by the wallet to decrypt the data (keypairs) associated with the wallet.

func (w *Wallet) bytesToVault(bytes []byte) error {
	err := json.Unmarshal(bytes, &w.vault)
	if err != nil {
		return err
	}
	// Restore the key from PEM after loading
	return w.vault.RestoreKeyFromPEM()
}

// / PrivateKey returns the private key from the vault associated with the wallet.
// / If the wallet is encrypted, this method will return an error.
func (w *Wallet) PrivateKey() (*ecdsa.PrivateKey, error) {
	if w.Encrypted {
		return nil, errors.New("cannot get private key from an encrypted wallet")
	}

	return w.vault.Key, nil
}

// PrivateBytes returns the bytes representation of the private key associated with the wallet.
// If the wallet is encrypted, this method will return an error. If the private key is nil,
// this method will also return an error.
func (w *Wallet) PrivateBytes() ([]byte, error) {
	if w.Encrypted {
		return nil, errors.New("cannot get private key from an encrypted wallet")
	}

	if w.vault == nil || w.vault.Key == nil {
		return nil, errors.New("private key is nil")
	}

	bytes, err := x509.MarshalECPrivateKey(w.vault.Key)
	if err != nil {
		return nil, err
	}

	return bytes, nil
}

// PrivatePEM returns the PEM representation of the private key associated with the wallet.
// If the wallet is encrypted, this method will return an empty string.
// If the private key is nil, this method will also return an empty string.
func (w *Wallet) PrivatePEM() string {
	if w.Encrypted {
		return ""
	}

	if w.vault == nil || w.vault.Key == nil || w.vault.Pem == nil {
		return ""
	}

	return w.vault.PrivatePEM()
}

// PublicKey returns the public key from the data (keypairs) associated with the wallet.
// If the wallet is encrypted, this method will return an error. If the public key is nil,
// this method will also return an error.
func (w *Wallet) PublicKey() (*ecdsa.PublicKey, error) {
	if w.Encrypted {
		return nil, errors.New("cannot get public key from an encrypted wallet")
	}

	if w.vault == nil || w.vault.Key == nil || w.vault.Key.Curve == nil {
		return nil, errors.New("public key is nil")
	}

	return &w.vault.Key.PublicKey, nil
}

// PublicBytes returns the bytes representation of the public key.
// If the wallet is encrypted, this returns an error.
// If the public key is nil, this returns an error.
// Otherwise, this returns the bytes representation of the public key.
func (w *Wallet) PublicBytes() ([]byte, error) {
	if w.Encrypted {
		return nil, errors.New("cannot get public key from an encrypted wallet")
	}

	if w.vault == nil || w.vault.Key == nil {
		return nil, errors.New("public key is nil")
	}

	pub := w.vault.Key.Public()
	if pub == nil {
		return nil, errors.New("public key is nil")
	}

	if _, ok := pub.(*ecdsa.PublicKey); !ok {
		return nil, errors.New("public key is not of type *ecdsa.PublicKey")
	}

	bytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal public key: %w", err)
	}

	return bytes, nil
}

// PublicPEM returns the PEM representation of the public key.
// If the wallet is encrypted, this returns an empty string.
// If the public key is nil, this also returns an empty string.
// Otherwise, it returns the PEM representation of the public key.
func (w *Wallet) PublicPEM() string {
	if w.Encrypted {
		return ""
	}

	// Only the PEM is required: a wallet reconstructed from a block carries the
	// public key for signature verification but has no private key.
	if w.vault == nil || w.vault.Pem == nil {
		return ""
	}

	return w.vault.PublicPEM()
}

// SendTransaction sends a transaction from the wallet to the specified address on the blockchain.
// It first checks if the wallet is encrypted, and returns an error if it is.
// It then gets the wallet's balance, and checks if it has enough funds to cover the transaction fee.
// If the wallet has sufficient funds, it prints a log message and sends the transaction to the blockchain.
// If the transaction is successfully sent, it returns the transaction.
// If there is an error sending the transaction, it returns the error.
func (w *Wallet) SendTransaction(tx Transaction, bc *Blockchain) (*Transaction, error) {
	if w.Encrypted {
		return nil, errors.New("cannot send transaction from an encrypted wallet")
	}

	// get the wallets balance
	balance := w.GetBalance()

	// Check if the wallet has enough balance.
	if balance < transactionFee {
		return nil, fmt.Errorf("insufficient funds")
	}

	LogVerbosef("Sending TX (%s): %s", tx.GetProtocol(), tx.GetID())

	// Send the transaction to the network.
	err := tx.Send(bc)
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}

	return &tx, nil
}

// encrypt is a private internal method that encrypts the data (keypairs) associated with the wallet.
// It derives a key from the provided key and salt, creates an AES-GCM cipher, generates a random nonce,
// and then seals the data using the cipher. The resulting ciphertext is appended with the salt and returned.
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
// It takes the encryption key and the encrypted data as input, and returns the decrypted plaintext.
// The method first extracts the salt from the end of the encrypted data, then derives the encryption key
// using the provided key and the extracted salt. It then uses the derived key to decrypt the ciphertext
// using AES-GCM. The decrypted plaintext is returned.
func (w *Wallet) decrypt(key, data []byte) ([]byte, error) {
	// The salt is appended to the ciphertext by encrypt(). Slicing it off without
	// a length check panicked on any short or corrupt payload.
	if len(data) <= saltSize {
		return nil, errors.New("ciphertext is too short to contain a salt")
	}
	salt, data := data[len(data)-saltSize:], data[:len(data)-saltSize]

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
// It uses the scrypt key derivation function to derive a 32-byte key from the password and salt.
// If the salt is nil, a new random 32-byte salt is generated.
// The derived key and the salt are returned.
func (w *Wallet) deriveKey(password, salt []byte) ([]byte, []byte, error) {
	if salt == nil {
		salt = make([]byte, 32)
		if _, err := rand.Read(salt); err != nil {
			return nil, nil, err
		}
	}

	// Use the cost profile recorded on this wallet, so a wallet always decrypts
	// with the same parameters it was encrypted with.
	sp := w.EncryptionParams.scrypt()

	key, err := scrypt.Key(password, salt, sp.N, sp.R, sp.P, 32)
	if err != nil {
		return nil, nil, err
	}

	return key, salt, nil
}

// Lock locks the wallet using the provided passphrase. Basically the wallet's data (keypairs), including the private key are
// encrypted using the passphrase.
//
// If the wallet is already encrypted, this method will return an error. If the provided passphrase is too weak, this method
// will also return an error.
//
// This method first converts the passphrase to bytes, then gets the wallet's data as bytes using the vaultToBytes method.
// It then encrypts the wallet's data using the encrypt method and stores the ciphertext in the Ciphertext field.
// Finally, it sets the vault field to nil and the Encrypted field to true.
func (w *Wallet) Lock(passphrase string) error {

	// Check if the wallet is already encrypted.
	if w.Encrypted {
		return errors.New("wallet is already encrypted")
	}

	// Check if the passphrase is strong enough.
	if testPasswordStrength(passphrase) != nil {
		return errors.New("password is too weak")
	}

	if verbose {
		LogVerbosef("Locking wallet [%s]", w.ID)
	}

	// Convert the passphrase to bytes.
	pwAsBytes := []byte(passphrase)

	// Get the wallet's data as bytes.
	dataAsbytes, err := w.vaultToBytes()
	if err != nil {
		return err
	}

	// Encrypt the wallet's data.
	w.Ciphertext, err = w.encrypt(pwAsBytes, dataAsbytes)
	if err != nil {
		return err
	}

	w.vault = nil
	w.Encrypted = true

	if verbose {
		LogVerbosef("Wallet [%s] locked", w.ID)
	}

	return nil
}

// Unlock unlocks the wallet using the provided passphrase. Basically the wallet's data (keypairs), including the private key are
// decrypted using the passphrase.
//
// If the wallet is already decrypted, this method will return an error. If the provided passphrase is incorrect, this method
// will also return an error.
//
// This method first converts the passphrase to bytes, then decrypts the wallet's data using the decrypt method. It then
// sets the wallet's data by calling the bytesToVault method. Finally, it sets the Ciphertext field to an empty slice and
// the Encrypted field to false.
func (w *Wallet) Unlock(passphrase string) error {

	// Check if the wallet is already decrypted.
	if w.Encrypted {
		if verbose {
			LogVerbosef("Unlocking wallet [%s]", w.ID)
		}

		// Convert the passphrase to bytes.
		pwAsBytes := []byte(passphrase)

		// Decrypt the wallet's data.
		dataAsBytes, err := w.decrypt(pwAsBytes, w.Ciphertext)
		if err != nil {
			return err
		}

		// Swallowing this error used to leave Encrypted=false with a nil vault,
		// so the next PrivatePEM()/PublicPEM() call nil-dereferenced.
		if err := w.bytesToVault(dataAsBytes); err != nil {
			return fmt.Errorf("failed to restore wallet vault: %w", err)
		}

		// Set the wallet's data.
		w.Ciphertext = []byte{}
		w.Encrypted = false
	}

	return nil
}

// Close encrypts and saves the wallet to disk as a JSON file. If the wallet is already encrypted, this method will return an error.
// This method first locks the wallet using the provided passphrase, then saves the encrypted wallet to disk using the localStorage.Set method.
// If any errors occur during the locking or saving process, this method will return an error.
func (w *Wallet) Close(passphrase string) error {
	if !w.Encrypted {
		err := w.Lock(passphrase)
		if err != nil {
			return fmt.Errorf("failed to save wallet: %v", err)
		}

		err = localStorage.Set("wallet", w)
		if err != nil {
			return fmt.Errorf("failed to save wallet: %v", err)
		}

	}

	if verbose {
		LogVerbosef("Wallet [%s] saved to disk", w.ID)
	}

	return nil
}

// Open loads the wallet from disk that was saved as a JSON file and, when a
// passphrase is supplied, unlocks it so wallet.vault is usable.
//
// This previously called localStorage.Set, i.e. it *overwrote* the stored wallet
// with whatever was in memory. LocalWalletList builds an empty Wallet shell per
// file and calls Open on it, so listing wallets destroyed every private key on
// disk. Loading must never write.
//
// An empty passphrase loads metadata only and leaves the wallet locked; any
// non-empty passphrase must successfully unlock or Open reports an error rather
// than silently returning a still-locked wallet.
func (w *Wallet) Open(passphrase string) error {
	if w.Address == "" {
		return errors.New("cannot open a wallet without an address")
	}

	if err := localStorage.Get("wallet", w); err != nil {
		return fmt.Errorf("failed to load wallet: %w", err)
	}

	if passphrase != "" {
		if err := w.Unlock(passphrase); err != nil {
			return fmt.Errorf("failed to unlock wallet: %w", err)
		}
	}

	LogVerbosef("Wallet [%s] loaded (locked: %v) from disk", w.ID, w.Encrypted)

	return nil
}

// ListWalletAddresses returns the addresses of every wallet on disk.
//
// It reads filenames only. LocalWalletList used to load each wallet through
// Wallet.Open, which wrote instead of read and so destroyed every key file it
// touched.
func ListWalletAddresses() ([]string, error) {
	ls, err := GetLocalStorage()
	if err != nil {
		return nil, err
	}

	files, err := filepath.Glob(filepath.Join(ls.dataPath, "wallets", "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to list wallets: %w", err)
	}

	addresses := make([]string, 0, len(files))
	for _, file := range files {
		name := filepath.Base(file)
		addresses = append(addresses, strings.TrimSuffix(name, filepath.Ext(name)))
	}

	return addresses, nil
}

// OpenWallet loads a wallet from disk and unlocks it with the given passphrase.
func OpenWallet(address, passphrase string) (*Wallet, error) {
	if address == "" {
		return nil, errors.New("wallet address is required")
	}

	wallet := &Wallet{Address: address}
	if err := wallet.Open(passphrase); err != nil {
		return nil, err
	}
	return wallet, nil
}

// LocalWalletList searches the wallet folder for all JSON files, loads each one, and displays the Wallet ID, Name, Address, and Tags.
func LocalWalletList() error {
	addresses, err := ListWalletAddresses()
	if err != nil {
		return err
	}

	LogInfof("Wallets found: %d", len(addresses))
	for _, address := range addresses {
		LogInfof("  %s", address)
	}

	return nil
}

// LocalWalletCount returns the number of wallets in the wallet folder.
func LocalWalletCount() (count int, err error) {
	files, err := filepath.Glob(filepath.Join(walletFolder, "*.json"))
	if err != nil {
		return 0, fmt.Errorf("failed to list wallets: %v", err)
	}

	return len(files), nil
}
