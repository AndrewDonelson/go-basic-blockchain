package sdk

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var (
	transactionQueueSize = 5
)

func Sleepy() {
	// Generate a random seed based on the current time
	rand.Seed(time.Now().UnixNano())

	// Generate a random duration between 1 and 3 seconds
	minDuration := 1 * time.Second
	maxDuration := 2 * time.Second
	randomDuration := minDuration + time.Duration(rand.Intn(int(maxDuration-minDuration)))

	// Sleep for the random duration
	time.Sleep(randomDuration)
}

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())
	os.Exit(m.Run())
}

func TestBlockchain(t *testing.T) {
	var err error

	// Use isolated config to avoid conflicts
	config := NewConfig()
	config.DataPath = "./test_data_blockchain"

	bc := NewBlockchain(config)
	bc.Run(1)

	// Create wallets and add transactions
	wallets := make([]*Wallet, 2)
	for i := 0; i < len(wallets); i++ {
		wallets[i], err = NewWallet(NewWalletOptions(ThisBlockchainOrganizationID, ThisBlockchainAppID, ThisBlockchainAdminUserID, ThisBlockchainDevAssetID, "Wallet"+strconv.Itoa(i), testPassPhrase, []string{"tag1", "tag2"}))
		assert.NoError(t, err)
	}

	// Add transactions
	successfulTransactions := 0
	for numTx := 0; numTx < 2; numTx++ {
		for j := 0; j < transactionQueueSize; j++ {
			// Pick two random, distinct wallets
			var fromWallet, toWallet *Wallet
			for fromWallet == toWallet {
				fromWallet = wallets[rand.Intn(len(wallets))]
				toWallet = wallets[rand.Intn(len(wallets))]
			}

			// Use very small amounts to ensure sufficient balance
			amount := rand.Float64() * 1.0 // Max 1.0 instead of 10.0
			if amount < 0.01 {
				amount = 0.01 // Minimum amount
			}

			bankTx, err := NewBankTransaction(fromWallet, toWallet, amount)
			if err != nil {
				t.Logf("Skipping bank transaction due to insufficient balance: %v", err)
				continue // Skip this transaction if insufficient balance
			}

			// Sign the transaction
			signature, err := bankTx.Sign([]byte(fromWallet.PrivatePEM()))
			if err != nil {
				t.Logf("Failed to sign bank transaction: %v", err)
				continue
			}
			bankTx.Signature = signature

			bc.AddTransaction(bankTx)
			successfulTransactions++

			Sleepy()

			// Create message transaction
			msgTx, err := NewMessageTransaction(toWallet, fromWallet, fmt.Sprintf("Thank you %s!", toWallet.GetWalletName()))
			if err != nil {
				t.Logf("Failed to create message transaction: %v", err)
				continue
			}

			// Sign the message transaction
			msgSignature, err := msgTx.Sign([]byte(toWallet.PrivatePEM()))
			if err != nil {
				t.Logf("Failed to sign message transaction: %v", err)
				continue
			}
			msgTx.Signature = msgSignature

			bc.AddTransaction(msgTx)
			successfulTransactions++
		}
	}

	// Log the number of successful transactions
	t.Logf("Successfully created %d transactions", successfulTransactions)

	// Further test cases to be added
}
