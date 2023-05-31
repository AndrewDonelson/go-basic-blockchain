package sdk

import (
	"crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"io"
	"time"

	"github.com/pborman/uuid"
	"golang.org/x/crypto/pbkdf2"
)

// Wallet represents a user's wallet.
type Wallet struct {
	ID         string
	Name       string
	Tags       []string
	PrivateKey []byte
	PublicKey  []byte
	Address    string
	Balance    float64
}

// NewWallet creates a new wallet with a unique ID, name, and set of tags
func NewWallet(name string, tags []string) (*Wallet, error) {
	// Generate a new private key.
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	// Convert the private key to PEM format
	privateKeyBytes := x509.MarshalPKCS1PrivateKey(privateKey)

	// Generate a new public key.
	publicKeyBytes, err := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err != nil {
		return nil, err
	}

	// Create a new wallet with a unique ID, name, and set of tags.
	wallet := &Wallet{
		ID:         uuid.New(),
		Name:       name,
		Tags:       tags,
		PrivateKey: privateKeyBytes,
		PublicKey:  publicKeyBytes,
		Balance:    fundWalletAmount,
		Address:    "",
	}

	if err != nil {
		return nil, err
	}

	wallet.GetAddress()
	fmt.Printf("[%s] Created new Wallet: %+v\n", time.Now().Format(logDateTimeFormat), PrettyPrint(wallet))

	return wallet, nil
}

// GetAddress generates and returns the wallet address.
func (w *Wallet) GetAddress() string {
	// If the address is already generated, return it.
	if w.Address != "" {
		return w.Address
	}

	// Generate an address by hashing the public key and encoding it in hexadecimal.
	hash := sha256.Sum256(w.PublicKey)
	w.Address = hex.EncodeToString(hash[:])

	return w.Address
}

// SendTransaction sends a new transaction from the sender's wallet to the recipient's address.
func (w *Wallet) SendTransaction(to string, tx Transaction) (*Transaction, error) {
	// Check if the wallet has enough balance
	if w.Balance < transactionFee {
		return nil, fmt.Errorf("insufficient funds")
	}

	fmt.Printf("[%s] Sending TX (%s): %+v\n", time.Now().Format(logDateTimeFormat), tx.GetProtocol(), tx)

	// Send the transaction to the network
	err := tx.Send()
	if err != nil {
		return nil, fmt.Errorf("failed to send transaction: %v", err)
	}

	// // Sign the transaction
	// err := w.SignTransaction(tx)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to sign transaction: %v", err)
	// }

	// // Broadcast the transaction to the network
	// // This function needs to be implemented according to your network protocol
	// err = broadcastTransaction(tx)
	// if err != nil {
	// 	return nil, fmt.Errorf("failed to broadcast transaction: %v", err)
	// }

	// // Deduct the amount from the wallet's balance
	// w.Balance -= amount

	return &tx, nil
}

// SignTransaction signs the given transaction with the wallet's private key.
func (w *Wallet) SignTransaction(tx Transaction) error {
	// Convert the private key to rsa.PrivateKey
	block, _ := pem.Decode(w.PrivateKey)
	if block == nil {
		return fmt.Errorf("failed to parse PEM block containing the private key")
	}

	privKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return fmt.Errorf("failed to parse private key: %v", err)
	}

	// Get the SHA-256 hash of the transaction
	txHash := sha256.Sum256([]byte(fmt.Sprintf("%v", tx)))

	// Sign the transaction hash
	signature, err := rsa.SignPKCS1v15(rand.Reader, privKey, crypto.SHA256, txHash[:])
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	err = tx.Sign(signature)
	if err != nil {
		return fmt.Errorf("failed to sign transaction: %v", err)
	}

	return nil
}

// This function should be implemented according to your network protocol
func broadcastTransaction(tx *Transaction) error {
	// Implement the function body here
	return nil
}

// EncryptPrivateKey encrypts the wallet's private key using the passphrase.
func (w *Wallet) EncryptPrivateKey(passphrase string) error {
	// Generate a new salt
	salt := make([]byte, saltSize)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return err
	}

	// Derive a new key from the passphrase
	key := pbkdf2.Key([]byte(passphrase), salt, 4096, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	// Encrypt the private key
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	// Generate a new nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return err
	}

	// Encrypt the private key and prepend the salt and nonce to it
	w.PrivateKey = append(salt, append(nonce, gcm.Seal(nil, nonce, w.PrivateKey, nil)...)...)
	return nil
}

// DecryptPrivateKey decrypts the wallet's private key using the passphrase.
func (w *Wallet) DecryptPrivateKey(passphrase string) error {
	// Extract the salt from the encrypted private key
	salt := w.PrivateKey[:saltSize]

	// Derive the key from the passphrase
	key := pbkdf2.Key([]byte(passphrase), salt, 4096, 32, sha256.New)
	block, err := aes.NewCipher(key)
	if err != nil {
		return err
	}

	// Create a new GCM instance
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return err
	}

	// Extract the nonce from the encrypted private key
	nonceSize := gcm.NonceSize()

	// Extract the nonce and ciphertext from the encrypted private key
	nonce, ciphertext := w.PrivateKey[saltSize:saltSize+nonceSize], w.PrivateKey[saltSize+nonceSize:]

	// Decrypt the private key
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return err
	}

	// Set the plaintext private key
	w.PrivateKey = plaintext
	return nil
}
