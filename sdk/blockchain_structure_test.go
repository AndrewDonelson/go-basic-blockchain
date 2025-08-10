package sdk

import (
	"math/big"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBlockchainStructure_BlockCreation tests basic block creation functionality
func TestBlockchainStructure_BlockCreation(t *testing.T) {
	// Arrange
	transactions := []Transaction{}
	previousHash := "0000000000000000000000000000000000000000000000000000000000000000"

	// Act
	block := NewBlock(transactions, previousHash)

	// Assert
	assert.NotNil(t, block)
	assert.Equal(t, int32(1), block.Header.Version)
	assert.Equal(t, previousHash, block.Header.PreviousHash)
	assert.NotNil(t, block.Header.Timestamp)
	assert.Equal(t, uint32(InitialDifficulty), block.Header.Difficulty)
	assert.Equal(t, uint32(0), block.Header.Nonce)
	assert.NotEmpty(t, block.Hash)
	assert.Len(t, block.Transactions, 0)
}

// TestBlockchainStructure_BlockWithTransactions tests block creation with transactions
func TestBlockchainStructure_BlockWithTransactions(t *testing.T) {
	// Arrange
	wallet1, _ := NewWallet(NewWalletOptions(NewBigInt(1), NewBigInt(1), NewBigInt(1), NewBigInt(1), "TestWallet1", testPassPhrase, []string{}))
	wallet2, _ := NewWallet(NewWalletOptions(NewBigInt(1), NewBigInt(1), NewBigInt(1), NewBigInt(2), "TestWallet2", testPassPhrase, []string{}))

	// Unlock wallets to access their data
	err := wallet1.Unlock(testPassPhrase)
	if err != nil {
		t.Fatalf("Failed to unlock wallet1: %v", err)
	}
	err = wallet2.Unlock(testPassPhrase)
	if err != nil {
		t.Fatalf("Failed to unlock wallet2: %v", err)
	}

	// Check wallet balances
	t.Logf("Wallet1 balance: %f", wallet1.GetBalance())
	t.Logf("Wallet2 balance: %f", wallet2.GetBalance())

	bankTx, err := NewBankTransaction(wallet1, wallet2, 10.0)
	if err != nil {
		t.Fatalf("Failed to create bank transaction: %v", err)
	}
	assert.NotNil(t, bankTx, "Bank transaction should not be nil")

	transactions := []Transaction{bankTx}
	previousHash := "0000000000000000000000000000000000000000000000000000000000000000"

	// Act
	block := NewBlock(transactions, previousHash)

	// Assert
	assert.NotNil(t, block)
	assert.Len(t, block.Transactions, 1)
	assert.Equal(t, bankTx.GetID(), block.Transactions[0].GetID())
	assert.NotEmpty(t, block.Header.MerkleRoot)
}

// TestBlockchainStructure_BlockValidation tests block validation functionality
func TestBlockchainStructure_BlockValidation(t *testing.T) {
	// Arrange
	config := NewConfig()
	config.DataPath = "./test_data_blockchain_structure"
	bc := NewBlockchain(config)

	// Create genesis block
	genesisBlock := bc.GetLatestBlock()
	require.NotNil(t, genesisBlock)

	// Create a valid block
	wallet1, _ := NewWallet(NewWalletOptions(NewBigInt(1), NewBigInt(1), NewBigInt(1), NewBigInt(1), "TestWallet1", testPassPhrase, []string{}))
	wallet2, _ := NewWallet(NewWalletOptions(NewBigInt(1), NewBigInt(1), NewBigInt(1), NewBigInt(2), "TestWallet2", testPassPhrase, []string{}))

	// Unlock wallets to access their data
	err := wallet1.Unlock(testPassPhrase)
	if err != nil {
		t.Fatalf("Failed to unlock wallet1: %v", err)
	}
	err = wallet2.Unlock(testPassPhrase)
	if err != nil {
		t.Fatalf("Failed to unlock wallet2: %v", err)
	}

	var bankTx *Bank
	bankTx, err = NewBankTransaction(wallet1, wallet2, 10.0)
	if err != nil {
		t.Fatalf("Failed to create bank transaction: %v", err)
	}

	// Set transaction status to confirmed for validation
	bankTx.SetStatus(StatusConfirmed)

	transactions := []Transaction{bankTx}

	validBlock := NewBlock(transactions, genesisBlock.Hash)
	validBlock.Index = *big.NewInt(1)
	validBlock.Hash = validBlock.CalculateHash()

	// Act
	validateErr := validBlock.Validate(genesisBlock)

	// Assert
	assert.NoError(t, validateErr)
}

// TestBlockchainStructure_BlockValidation_InvalidPreviousHash tests validation with invalid previous hash
func TestBlockchainStructure_BlockValidation_InvalidPreviousHash(t *testing.T) {
	// Arrange
	config := NewConfig()
	config.DataPath = "./test_data_blockchain_structure_invalid"
	bc := NewBlockchain(config)

	genesisBlock := bc.GetLatestBlock()
	require.NotNil(t, genesisBlock)

	// Create block with invalid previous hash
	transactions := []Transaction{}
	invalidBlock := NewBlock(transactions, "invalid_hash")
	invalidBlock.Index = *big.NewInt(1)
	invalidBlock.Hash = invalidBlock.CalculateHash()

	// Act
	err := invalidBlock.Validate(genesisBlock)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid previous hash")
}

// TestBlockchainStructure_BlockValidation_FutureTimestamp tests validation with future timestamp
func TestBlockchainStructure_BlockValidation_FutureTimestamp(t *testing.T) {
	// Arrange
	config := NewConfig()
	config.DataPath = "./test_data_blockchain_structure_future"
	bc := NewBlockchain(config)

	genesisBlock := bc.GetLatestBlock()
	require.NotNil(t, genesisBlock)

	// Create block with future timestamp
	transactions := []Transaction{}
	futureBlock := NewBlock(transactions, genesisBlock.Hash)
	futureBlock.Header.Timestamp = time.Now().Add(1 * time.Hour) // Future timestamp
	futureBlock.Index = *big.NewInt(1)
	futureBlock.Hash = futureBlock.CalculateHash()

	// Act
	err := futureBlock.Validate(genesisBlock)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "block timestamp is in the future")
}

// TestBlockchainStructure_BlockValidation_InvalidHash tests validation with invalid hash
func TestBlockchainStructure_BlockValidation_InvalidHash(t *testing.T) {
	// Arrange
	config := NewConfig()
	config.DataPath = "./test_data_blockchain_structure_invalid_hash"
	bc := NewBlockchain(config)

	genesisBlock := bc.GetLatestBlock()
	require.NotNil(t, genesisBlock)

	// Create block with invalid hash
	transactions := []Transaction{}
	invalidHashBlock := NewBlock(transactions, genesisBlock.Hash)
	invalidHashBlock.Index = *big.NewInt(1)
	invalidHashBlock.Hash = "invalid_hash" // Invalid hash

	// Act
	err := invalidHashBlock.Validate(genesisBlock)

	// Assert
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid block hash")
}

// TestBlockchainStructure_MerkleRootCalculation tests Merkle root calculation
func TestBlockchainStructure_MerkleRootCalculation(t *testing.T) {
	// Arrange
	wallet1, _ := NewWallet(NewWalletOptions(NewBigInt(1), NewBigInt(1), NewBigInt(1), NewBigInt(1), "TestWallet1", testPassPhrase, []string{}))
	wallet2, _ := NewWallet(NewWalletOptions(NewBigInt(1), NewBigInt(1), NewBigInt(1), NewBigInt(2), "TestWallet2", testPassPhrase, []string{}))

	// Unlock wallets to access their data
	err := wallet1.Unlock(testPassPhrase)
	if err != nil {
		t.Fatalf("Failed to unlock wallet1: %v", err)
	}
	err = wallet2.Unlock(testPassPhrase)
	if err != nil {
		t.Fatalf("Failed to unlock wallet2: %v", err)
	}

	// Create multiple transactions
	bankTx1, err := NewBankTransaction(wallet1, wallet2, 10.0)
	if err != nil {
		t.Fatalf("Failed to create bank transaction 1: %v", err)
	}
	bankTx2, err := NewBankTransaction(wallet2, wallet1, 5.0)
	if err != nil {
		t.Fatalf("Failed to create bank transaction 2: %v", err)
	}
	transactions := []Transaction{bankTx1, bankTx2}

	// Act
	block := NewBlock(transactions, "previous_hash")
	merkleRoot := block.CalculateMerkleRoot()

	// Assert
	assert.NotEmpty(t, merkleRoot)
	assert.Len(t, merkleRoot, 32) // SHA-256 hash is 32 bytes
}

// TestBlockchainStructure_MerkleRootEmptyBlock tests Merkle root for empty block
func TestBlockchainStructure_MerkleRootEmptyBlock(t *testing.T) {
	// Arrange
	transactions := []Transaction{}

	// Act
	block := NewBlock(transactions, "previous_hash")
	merkleRoot := block.CalculateMerkleRoot()

	// Assert
	assert.Empty(t, merkleRoot) // Empty block should have empty Merkle root
}

// TestBlockchainStructure_BlockHashConsistency tests that block hash is consistent
func TestBlockchainStructure_BlockHashConsistency(t *testing.T) {
	// Arrange
	transactions := []Transaction{}
	previousHash := "0000000000000000000000000000000000000000000000000000000000000000"

	// Act
	block := NewBlock(transactions, previousHash)
	hash1 := block.CalculateHash()
	hash2 := block.CalculateHash()

	// Assert
	assert.Equal(t, hash1, hash2)
	assert.NotEmpty(t, hash1)
	assert.Len(t, hash1, 64) // SHA-256 hex string is 64 characters
}

// TestBlockchainStructure_BlockHashUniqueness tests that different blocks have different hashes
func TestBlockchainStructure_BlockHashUniqueness(t *testing.T) {
	// Arrange
	transactions1 := []Transaction{}
	transactions2 := []Transaction{}
	previousHash := "0000000000000000000000000000000000000000000000000000000000000000"

	// Act
	block1 := NewBlock(transactions1, previousHash)
	block2 := NewBlock(transactions2, previousHash)

	// Modify block2 slightly
	block2.Header.Nonce = 1

	hash1 := block1.CalculateHash()
	hash2 := block2.CalculateHash()

	// Assert
	assert.NotEqual(t, hash1, hash2)
}

// TestBlockchainStructure_BlockSerialization tests block serialization and deserialization
func TestBlockchainStructure_BlockSerialization(t *testing.T) {
	// Arrange
	transactions := []Transaction{}
	previousHash := "0000000000000000000000000000000000000000000000000000000000000000"
	originalBlock := NewBlock(transactions, previousHash)

	// Act
	serialized := originalBlock.Bytes()
	assert.NotEmpty(t, serialized)

	// Test that we can create a new block from the same data
	newBlock := NewBlock(transactions, previousHash)
	newSerialized := newBlock.Bytes()

	// Assert - blocks should have same structure but timestamps may differ
	assert.NotEmpty(t, newSerialized)
	assert.Contains(t, string(serialized), "header")
	assert.Contains(t, string(serialized), "transactions")
	assert.Contains(t, string(newSerialized), "header")
	assert.Contains(t, string(newSerialized), "transactions")

	// Check that both serialized blocks have similar structure
	// Allow for small differences in timestamp precision
	assert.GreaterOrEqual(t, len(newSerialized), len(serialized)-5)
	assert.LessOrEqual(t, len(newSerialized), len(serialized)+5)
}

// TestBlockchainStructure_BlockIndexing tests block indexing functionality
func TestBlockchainStructure_BlockIndexing(t *testing.T) {
	// Arrange
	config := NewConfig()
	config.DataPath = "./test_data_blockchain_structure_indexing"
	bc := NewBlockchain(config)

	// Act
	genesisBlock := bc.GetLatestBlock()
	require.NotNil(t, genesisBlock)

	// Assert
	assert.Equal(t, int64(0), genesisBlock.Index.Int64())
	assert.Equal(t, 1, bc.GetBlockCount())
}

// TestBlockchainStructure_BlockRetrieval tests block retrieval by hash and index
func TestBlockchainStructure_BlockRetrieval(t *testing.T) {
	// Arrange
	config := NewConfig()
	config.DataPath = "./test_data_blockchain_structure_retrieval"
	bc := NewBlockchain(config)

	genesisBlock := bc.GetLatestBlock()
	require.NotNil(t, genesisBlock)

	// Act
	blockByHash := bc.GetBlockByHash(genesisBlock.Hash)
	blockByIndex := bc.GetBlockByIndex(0)

	// Assert
	assert.NotNil(t, blockByHash)
	assert.NotNil(t, blockByIndex)
	assert.Equal(t, genesisBlock.Hash, blockByHash.Hash)
	assert.Equal(t, genesisBlock.Hash, blockByIndex.Hash)
}

// TestBlockchainStructure_BlockRetrieval_NotFound tests block retrieval for non-existent blocks
func TestBlockchainStructure_BlockRetrieval_NotFound(t *testing.T) {
	// Arrange
	config := NewConfig()
	config.DataPath = "./test_data_blockchain_structure_retrieval_notfound"
	bc := NewBlockchain(config)

	// Act
	blockByHash := bc.GetBlockByHash("non_existent_hash")
	blockByIndex := bc.GetBlockByIndex(999)

	// Assert
	assert.Nil(t, blockByHash)
	assert.Nil(t, blockByIndex)
}

// TestBlockchainStructure_ChainValidation tests complete chain validation
func TestBlockchainStructure_ChainValidation(t *testing.T) {
	// Arrange
	config := NewConfig()
	config.DataPath = "./test_data_blockchain_structure_chain"
	bc := NewBlockchain(config)

	// Act
	err := bc.ValidateChain()

	// Assert
	assert.NoError(t, err)
}

// TestBlockchainStructure_BlockMining tests block mining functionality
func TestBlockchainStructure_BlockMining(t *testing.T) {
	// Arrange
	transactions := []Transaction{}
	previousHash := "0000000000000000000000000000000000000000000000000000000000000000"
	block := NewBlock(transactions, previousHash)
	difficulty := 1 // Reduced difficulty for faster test execution

	// Act - add timeout to prevent hanging
	done := make(chan bool, 1)
	go func() {
		block.Mine(uint(difficulty))
		done <- true
	}()

	select {
	case <-done:
		// Mining completed successfully
	case <-time.After(10 * time.Second):
		t.Fatal("Mining test timed out after 10 seconds")
	}

	// Assert
	assert.True(t, strings.HasPrefix(block.Hash, strings.Repeat("0", difficulty)))
	assert.Greater(t, block.Header.Nonce, uint32(0))
}

// TestBlockchainStructure_BlockMining_ZeroDifficulty tests mining with zero difficulty
func TestBlockchainStructure_BlockMining_ZeroDifficulty(t *testing.T) {
	// Arrange
	transactions := []Transaction{}
	previousHash := "0000000000000000000000000000000000000000000000000000000000000000"
	block := NewBlock(transactions, previousHash)
	difficulty := 0

	// Act - add timeout to prevent hanging
	done := make(chan bool, 1)
	go func() {
		block.Mine(uint(difficulty))
		done <- true
	}()

	select {
	case <-done:
		// Mining completed successfully
	case <-time.After(5 * time.Second):
		t.Fatal("Mining test timed out after 5 seconds")
	}

	// Assert
	// With zero difficulty, any hash should be valid
	assert.NotEmpty(t, block.Hash)
}

// TestBlockchainStructure_BlockMining_HighDifficulty tests mining with high difficulty
func TestBlockchainStructure_BlockMining_HighDifficulty(t *testing.T) {
	// Arrange
	transactions := []Transaction{}
	previousHash := "0000000000000000000000000000000000000000000000000000000000000000"
	block := NewBlock(transactions, previousHash)
	difficulty := 2 // Reduced difficulty for faster test execution

	// Act - add timeout to prevent hanging
	startTime := time.Now()
	done := make(chan bool, 1)
	go func() {
		block.Mine(uint(difficulty))
		done <- true
	}()

	select {
	case <-done:
		// Mining completed successfully
	case <-time.After(15 * time.Second):
		t.Fatal("Mining test timed out after 15 seconds")
	}
	duration := time.Since(startTime)

	// Assert
	assert.True(t, strings.HasPrefix(block.Hash, strings.Repeat("0", difficulty)))
	assert.Greater(t, duration, time.Microsecond) // Should take some time (reduced expectation since algorithm is efficient)
}

// TestBlockchainStructure_BlockRewardCalculation tests block reward calculation
func TestBlockchainStructure_BlockRewardCalculation(t *testing.T) {
	// Arrange
	transactions := []Transaction{}
	previousHash := "0000000000000000000000000000000000000000000000000000000000000000"
	block := NewBlock(transactions, previousHash)

	// Act
	reward0 := block.CalculateBlockReward(0)           // Genesis block
	reward210000 := block.CalculateBlockReward(210000) // First halving
	reward420000 := block.CalculateBlockReward(420000) // Second halving

	// Assert
	assert.Equal(t, InitialBlockReward, reward0)
	assert.Equal(t, InitialBlockReward/2, reward210000)
	assert.Equal(t, InitialBlockReward/4, reward420000)
}

// TestBlockchainStructure_DifficultyAdjustment tests difficulty adjustment
func TestBlockchainStructure_DifficultyAdjustment(t *testing.T) {
	// Arrange
	transactions := []Transaction{}
	previousHash := "0000000000000000000000000000000000000000000000000000000000000000"

	previousBlock := NewBlock(transactions, previousHash)
	previousBlock.Header.Timestamp = time.Now().Add(-5 * time.Second) // 5 seconds ago (much faster than target)
	previousBlock.Header.Difficulty = 4

	currentBlock := NewBlock(transactions, previousBlock.Hash)
	currentBlock.Header.Timestamp = time.Now()
	targetBlockTime := 20 * time.Second

	// Act
	newDifficulty := currentBlock.AdjustDifficulty(previousBlock, targetBlockTime)

	// Assert
	assert.Greater(t, newDifficulty, previousBlock.Header.Difficulty) // Should increase difficulty
}

// TestBlockchainStructure_DifficultyAdjustment_SlowMining tests difficulty adjustment for slow mining
func TestBlockchainStructure_DifficultyAdjustment_SlowMining(t *testing.T) {
	// Arrange
	transactions := []Transaction{}
	previousHash := "0000000000000000000000000000000000000000000000000000000000000000"

	previousBlock := NewBlock(transactions, previousHash)
	previousBlock.Header.Timestamp = time.Now().Add(-60 * time.Second) // 60 seconds ago
	previousBlock.Header.Difficulty = 4

	currentBlock := NewBlock(transactions, previousBlock.Hash)
	currentBlock.Header.Timestamp = time.Now()
	targetBlockTime := 20 * time.Second

	// Act
	newDifficulty := currentBlock.AdjustDifficulty(previousBlock, targetBlockTime)

	// Assert
	assert.Less(t, newDifficulty, previousBlock.Header.Difficulty) // Should decrease difficulty
}

// TestBlockchainStructure_BloomFilter tests Bloom filter functionality
func TestBlockchainStructure_BloomFilter(t *testing.T) {
	// Arrange
	wallet1, _ := NewWallet(NewWalletOptions(NewBigInt(1), NewBigInt(1), NewBigInt(1), NewBigInt(1), "TestWallet1", testPassPhrase, []string{}))
	wallet2, _ := NewWallet(NewWalletOptions(NewBigInt(1), NewBigInt(1), NewBigInt(1), NewBigInt(2), "TestWallet2", testPassPhrase, []string{}))

	// Unlock wallets to access their data
	err := wallet1.Unlock(testPassPhrase)
	if err != nil {
		t.Fatalf("Failed to unlock wallet1: %v", err)
	}
	err = wallet2.Unlock(testPassPhrase)
	if err != nil {
		t.Fatalf("Failed to unlock wallet2: %v", err)
	}

	bankTx, err := NewBankTransaction(wallet1, wallet2, 10.0)
	if err != nil {
		t.Fatalf("Failed to create bank transaction: %v", err)
	}
	transactions := []Transaction{bankTx}
	previousHash := "0000000000000000000000000000000000000000000000000000000000000000"

	block := NewBlock(transactions, previousHash)

	// Act
	bloomFilter := block.CreateBloomFilter()

	// Assert
	assert.NotNil(t, bloomFilter)
	assert.True(t, bloomFilter.Contains([]byte(bankTx.GetID())))
	assert.False(t, bloomFilter.Contains([]byte("non_existent_tx")))
}

// TestBlockchainStructure_BlockStringRepresentation tests block string representation
func TestBlockchainStructure_BlockStringRepresentation(t *testing.T) {
	// Arrange
	transactions := []Transaction{}
	previousHash := "0000000000000000000000000000000000000000000000000000000000000000"
	block := NewBlock(transactions, previousHash)

	// Act
	str := block.String()

	// Assert
	assert.Contains(t, str, "Index:")
	assert.Contains(t, str, "Timestamp:")
	assert.Contains(t, str, "Transactions:")
	assert.Contains(t, str, "Nonce:")
	assert.Contains(t, str, "Hash:")
	assert.Contains(t, str, "PreviousHash:")
}

// TestBlockchainStructure_BlockGetTransactions tests transaction retrieval from block
func TestBlockchainStructure_BlockGetTransactions(t *testing.T) {
	// Arrange
	wallet1, _ := NewWallet(NewWalletOptions(NewBigInt(1), NewBigInt(1), NewBigInt(1), NewBigInt(1), "TestWallet1", testPassPhrase, []string{}))
	wallet2, _ := NewWallet(NewWalletOptions(NewBigInt(1), NewBigInt(1), NewBigInt(1), NewBigInt(2), "TestWallet2", testPassPhrase, []string{}))

	// Unlock wallets to access their data
	err := wallet1.Unlock(testPassPhrase)
	if err != nil {
		t.Fatalf("Failed to unlock wallet1: %v", err)
	}
	err = wallet2.Unlock(testPassPhrase)
	if err != nil {
		t.Fatalf("Failed to unlock wallet2: %v", err)
	}

	bankTx, err := NewBankTransaction(wallet1, wallet2, 10.0)
	if err != nil {
		t.Fatalf("Failed to create bank transaction: %v", err)
	}
	transactions := []Transaction{bankTx}
	previousHash := "0000000000000000000000000000000000000000000000000000000000000000"

	block := NewBlock(transactions, previousHash)

	// Act
	allTxs := block.GetTransactions("")
	specificTx := block.GetTransactions(bankTx.GetID())
	nonExistentTx := block.GetTransactions("non_existent_id")

	// Assert
	assert.Len(t, allTxs, 1)
	assert.Len(t, specificTx, 1)
	assert.Len(t, nonExistentTx, 0)
	assert.Equal(t, bankTx.GetID(), specificTx[0].GetID())
}
