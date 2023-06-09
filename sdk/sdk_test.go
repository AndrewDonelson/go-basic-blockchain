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
	maxDuration := 3 * time.Second
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

	assert := assert.New(t)

	bc := NewBlockchain()
	bc.Run(1)

	// Create wallets and add transactions
	wallets := make([]*Wallet, 5)
	for i := 0; i < len(wallets); i++ {
		wallets[i], err = NewWallet("Wallet"+strconv.Itoa(i), testPassPhrase, []string{"tag1", "tag2"})
		assert.NoError(err)
	}

	// Add transactions
	for numTx := 0; numTx < 5; numTx++ {
		for j := 0; j < transactionQueueSize; j++ {
			// Pick two random, distinct wallets
			var fromWallet, toWallet *Wallet
			for fromWallet == toWallet {
				fromWallet = wallets[rand.Intn(len(wallets))]
				toWallet = wallets[rand.Intn(len(wallets))]
			}

			bankTx, err := NewBankTransaction(fromWallet, toWallet, rand.Float64())
			assert.NoError(err)

			fromWallet.SignTransaction(bankTx)
			assert.NoError(err)
			bc.AddTransaction(bankTx)

			Sleepy()

			msgTx, err := NewMessageTransaction(toWallet, fromWallet, fmt.Sprintf("Thank you %s!", toWallet.GetWalletName()))
			assert.NoError(err)

			toWallet.SignTransaction(msgTx)
			assert.NoError(err)
			bc.AddTransaction(msgTx)

		}
	}

	// Further test cases to be added
}
