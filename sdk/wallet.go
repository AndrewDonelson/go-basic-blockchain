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
	"log"
	"path/filepath"
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
	mutex            sync.Mutex
}

// EncryptionParams holds the encryption parameters for the private key.
type EncryptionParams struct {
	SaltSize  int // Size of the salt used for key derivation
	NonceSize int // Size of the nonce used for encryption
}

// NewEncryptionParams creates a new EncryptionParams struct with the specified salt and nonce sizes.
// The salt size and nonce size are used to configure the encryption parameters for a wallet's private key.
func NewEncryptionParams(saltSize, nonceSize int) *EncryptionParams {
	return &EncryptionParams{
		SaltSize:  saltSize,
		NonceSize: nonceSize,
	}
}

// NewDefaultEncryptionParams creates a new EncryptionParams struct with default values.
// The default salt size is 32 bytes and the default nonce size is 12 bytes.
func NewDefaultEncryptionParams() *EncryptionParams {
	return NewEncryptionParams(saltSize, maxNonce)
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
	log.Printf("Creating new Wallet: %s", options.Name)
	wallet := &Wallet{
		ID:               NewPUID(options.OrganizationID, options.AppID, options.UserID, NewBigInt(0)),
		Address:          "",
		Encrypted:        false,
		EncryptionParams: NewDefaultEncryptionParams(),
		vault:            NewVaultWithData(options.Name, options.Tags, float64(fundWalletAmount)),
		Ciphertext:       []byte{},
	}

	// Generate a new private key.
	// err := wallet.vault.NewKeyPair()
	// if err != nil {
	// 	return nil, err
	// }

	// wallet.SetData("name", options.Name)
	// wallet.SetData("tags", options.Tags)
	// wallet.SetData("balance", float64(fundWalletAmount))
	wallet.GetAddress()

	// if verbose {
	// 	log.Printf("Created new Wallet: %+v", PrettyPrint(wallet))
	// } else {
	// 	log.Printf("Created new Wallet: %s", wallet.GetAddress())
	// }

	log.Printf("Created new Wallet: %s", wallet.GetAddress())

	// Save the wallet after creation
	err = wallet.Close(options.Passphrase)
	if err != nil {
		return nil, fmt.Errorf("failed to save wallet: %w", err)
	}

	return wallet, nil
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
		log.Println(err)
		return ""
	}

	return name.(string)
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
		log.Println(err)
		return 0
	}

	convertedBalance, err := ConvertToFloat64(balance)
	if err != nil {
		log.Printf("Error converting balance: %v", err)
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
		log.Println(err)
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
		log.Printf("Error getting public key bytes: %s", err)
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

	if w.vault.Key == nil {
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

	if w.vault.Key == nil {
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

	if w.vault.Key.PublicKey == (ecdsa.PublicKey{}) {
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

	pub := w.vault.Key.Public()
	if pub == nil {
		return nil, errors.New("public key is nil")
	}

	// Diagnostic: log the type
	log.Printf("Public key type: %T", pub)
	if ecdsaPub, ok := pub.(*ecdsa.PublicKey); ok {
		log.Printf("Public key curve: %T", ecdsaPub.Curve)
	} else {
		return nil, errors.New("public key is not of type *ecdsa.PublicKey")
	}

	bytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		log.Printf("x509.MarshalPKIXPublicKey error: %v", err)
		return nil, err
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

	if w.vault.Key.Public() == nil {
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

	log.Printf("Sending TX (%s): %+v", tx.GetProtocol(), tx)

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

	key, err := scrypt.Key(password, salt, 1048576, 8, 1, 32)
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
		log.Printf("Locking wallet [%s]", w.ID)
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
		log.Printf("Wallet [%s] locked", w.ID)
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
			log.Printf("Unlocking wallet [%s]", w.ID)
		}

		// Convert the passphrase to bytes.
		pwAsBytes := []byte(passphrase)

		// Decrypt the wallet's data.
		dataAsBytes, err := w.decrypt(pwAsBytes, w.Ciphertext)
		if err != nil {
			return err
		}

		w.bytesToVault(dataAsBytes)

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
		log.Printf("Wallet [%s] saved to disk", w.ID)
	}

	return nil
}

// Open loads the wallet from disk that was saved as a JSON file.
// It also unlocks the value and restores the wallet.vault object.
func (w *Wallet) Open(passphrase string) error {
	err := localStorage.Set("wallet", w)
	if err != nil {
		return err
	}

	if len(passphrase) >= 12 {
		err = w.Unlock(passphrase)
		if err != nil {
			return fmt.Errorf("failed to load wallet: %v", err)
		}
	}

	if verbose {
		log.Printf("Wallet [%s] loaded (locked: %v) from disk", w.ID, w.Encrypted)
	}

	return nil
}

// LocalWalletList searches the wallet folder for all JSON files, loads each one, and displays the Wallet ID, Name, Address, and Tags.
func LocalWalletList() error {
	walletList := make([]string, 0)

	files, err := filepath.Glob(filepath.Join(walletFolder, "*.json"))
	if err != nil {
		return fmt.Errorf("failed to list wallets: %v", err)
	}

	for _, file := range files {
		// Get the base name of the file without the extension
		address := filepath.Base(file[:len(file)-len(filepath.Ext(file))])

		wallet := &Wallet{Address: address}
		err := wallet.Open("")
		if err != nil {
			log.Printf("Failed to load wallet from file %s: %v", file, err)
			continue
		}

		walletList = append(walletList, fmt.Sprintf("ID: %s, Name: %s, Address: %s, Tags: %v", wallet.ID, wallet.GetWalletName(), wallet.GetAddress(), wallet.GetTags()))
	}

	log.Printf("Wallets in %s: %d", walletFolder, len(walletList))
	if len(walletList) == 0 {
		log.Println("No wallets found")
	} else {
		log.Println(PrettyPrint(walletList))
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
