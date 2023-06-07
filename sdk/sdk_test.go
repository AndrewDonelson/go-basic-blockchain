package sdk

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/onsi/gomega"
	"github.com/stretchr/testify/assert"
)

var (
	transactionQueueSize = 5
	transactionWaitTime  = 25 * time.Second
)

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())
	os.Exit(m.Run())
}

func TestBlockchain(t *testing.T) {
	var err error

	gomega.RegisterTestingT(t) // Register Gomega's fail handler
	assert := assert.New(t)

	bc := NewBlockchain()
	bc.Run(1)

	// Create wallets and add transactions
	wallets := make([]*Wallet, 5)
	for i := 0; i < len(wallets); i++ {
		wallets[i], err = NewWallet("Wallet"+strconv.Itoa(i), []string{"tag1", "tag2"})
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
			bc.AddTransaction(bankTx)

			msgTx, err := NewMessageTransaction(toWallet, fromWallet, fmt.Sprintf("Thank you %s!", toWallet.GetWalletName()))
			assert.NoError(err)
			bc.AddTransaction(msgTx)

		}
	}

	// Test adding transactions concurrently to simulate high load
	wg := sync.WaitGroup{}
	for i := 0; i < len(wallets); i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()

			for j := 0; j < transactionQueueSize; j++ {
				toWallet := wallets[(i+1)%len(wallets)]
				transaction, err := NewBankTransaction(wallets[i], toWallet, rand.Float64())
				assert.NoError(err)
				bc.AddTransaction(transaction)
			}
		}(i)
	}
	wg.Wait()

	gomega.Eventually(func() int {
		return len(bc.TransactionQueue)
	}, transactionWaitTime, 50*time.Millisecond).Should(gomega.Equal(0))

	// Further test cases to be added
}
