package sdk

import (
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
	transactionQueueSize = 100
	transactionWaitTime  = 25 * time.Second
)

func TestMain(m *testing.M) {
	rand.Seed(time.Now().UnixNano())
	os.Exit(m.Run())
}

func TestWalletMethods(t *testing.T) {
	assert := assert.New(t)

	wallet, err := NewWallet("test", []string{"tag1", "tag2"})
	assert.NoError(err)

	// Test GetAddress()
	address := wallet.GetAddress()
	assert.NotEmpty(address)

	// Test EncryptPrivateKey() and DecryptPrivateKey()
	err = wallet.EncryptPrivateKey("passphrase")
	assert.NoError(err)

	err = wallet.DecryptPrivateKey("passphrase")
	assert.NoError(err)

	// Further test cases to be added
}

func TestBlockchain(t *testing.T) {
	gomega.RegisterTestingT(t) // Register Gomega's fail handler
	assert := assert.New(t)

	bc := NewBlockchain()

	// Create wallets and add transactions
	wallets := make([]*Wallet, 5)
	for i := 0; i < len(wallets); i++ {
		var err error
		wallets[i], err = NewWallet("Wallet"+strconv.Itoa(i), []string{"tag1", "tag2"})
		assert.NoError(err)

		for j := 0; j < transactionQueueSize; j++ {
			toWallet := wallets[(i+1)%len(wallets)]
			transaction, err := NewBankTransaction(wallets[i], toWallet, rand.Float64())
			assert.NoError(err)
			bc.AddTransaction(transaction)
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
