// Package sdk is a software development kit for building blockchain applications.
// File sdk/blockchain.go - The main Blockchain file

package sdk

import (
	"crypto/sha512"
	"encoding/hex"
	"fmt"
	"math/big"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// State represents the current state of the blockchain.
type State struct {
	// Add state-related fields here if needed
}

// BlockchainPersistData represents the data that is persisted for a blockchain to disk.
type BlockchainPersistData struct {
	TXLookup       *Index `json:"tx_lookup"`
	CurrBlockIndex *int   `json:"current_block_index"`
	NextBlockIndex *int   `json:"next_block_index"`
}

// String returns a string representation of the BlockchainPersistData.
func (b *BlockchainPersistData) String() string {
	return fmt.Sprintf("TXLookup: %v, CurrBlockIndex: %v, NextBlockIndex: %v", b.TXLookup, b.CurrBlockIndex, b.NextBlockIndex)
}

// Blockchain is the main struct that represents the blockchain.
type Blockchain struct {
	cfg               *Config          // Configuration for the blockchain
	Blocks            []*Block         // Slice of blocks in the blockchain
	TransactionQueue  []Transaction    // Queue of transactions to be added to the blockchain
	TXLookup          *TXLookupManager // Map of Block Number/Index (Key) and Transaction ID (Value)
	mux               sync.Mutex       // Mutex to protect concurrent access to the blockchain
	CurrentBlockIndex int              // Current block index
	NextBlockIndex    int              // Next block index
	AvgTxsPerBlock    float64          // Average number of transactions per block
	State             *State           // Current state of the blockchain
}

// NewBlockchain creates a new instance of the Blockchain struct with the provided configuration.
func NewBlockchain(cfg *Config) *Blockchain {
	bc := &Blockchain{
		cfg:               cfg,
		Blocks:            []*Block{},
		TransactionQueue:  []Transaction{},
		TXLookup:          NewTXLookupManager(),
		CurrentBlockIndex: 0,
		NextBlockIndex:    1,
		AvgTxsPerBlock:    0,
		State:             &State{},
	}

	err := bc.Load()
	if err != nil {
		fmt.Printf("Error loading blockchain...\n%v\n. Creating new blockchain.\n", err)
		bc.createBlockchain()
	}

	return bc
}

// DisplayStatus displays the current status of the blockchain.
func (bc *Blockchain) DisplayStatus() {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	staticBlocksLen := len(bc.Blocks)
	staticTransactionQueueLen := len(bc.TransactionQueue)

	if staticBlocksLen != len(bc.Blocks) || staticTransactionQueueLen != len(bc.TransactionQueue) {
		fmt.Printf("[%s] Blockchain Activity: Blocks: %d, Transaction Queue: %d\n",
			time.Now().Format(logDateTimeFormat), len(bc.Blocks), len(bc.TransactionQueue))
	}
}

// GetConfig returns the configuration used to create the Blockchain instance.
func (bc *Blockchain) GetConfig() *Config {
	return bc.cfg
}

// Load loads the blockchain state from disk.
func (bc *Blockchain) Load() error {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	data := &BlockchainPersistData{}

	err := localStorage.Get("state", data)
	if err != nil {
		return err
	}

	if bc.TXLookup == nil {
		bc.TXLookup = NewTXLookupManager()
	}

	if data.TXLookup != nil {
		bc.TXLookup.Set(data.TXLookup)
	}

	if data.CurrBlockIndex != nil {
		bc.CurrentBlockIndex = *data.CurrBlockIndex
	}

	if data.NextBlockIndex != nil {
		bc.NextBlockIndex = *data.NextBlockIndex
	}

	return nil
}

// Save saves the blockchain state to disk.
func (bc *Blockchain) Save() error {
	data := &BlockchainPersistData{
		TXLookup:       bc.TXLookup.index.Get(),
		CurrBlockIndex: &bc.CurrentBlockIndex,
		NextBlockIndex: &bc.NextBlockIndex,
	}

	bc.mux.Lock()
	defer bc.mux.Unlock()

	return localStorage.Set("state", data)
}

// createBlockchain creates a new blockchain.
func (bc *Blockchain) createBlockchain() error {
	ThisBlockchainOrganizationID = NewBigInt(BlockhainOrganizationID)
	ThisBlockchainAppID = NewBigInt(BlockchainAppID)
	ThisBlockchainAdminUserID = NewBigInt(BlockchainAdminUserID)
	ThisBlockchainDevAssetID = NewBigInt(BlockchainDevAssetID)
	ThisBlockchainMinerID = NewBigInt(BlockchainMinerAssetID)

	genesisTxs := []Transaction{}

	devWalletPW, err := GenerateRandomPassword()
	if err != nil {
		return err
	}

	devWallet, err := NewWallet(NewWalletOptions(ThisBlockchainOrganizationID, ThisBlockchainAppID, ThisBlockchainAdminUserID, ThisBlockchainDevAssetID, "Dev", devWalletPW, []string{"blockchain", "master"}))
	if err != nil {
		return err
	}

	devWallet.Close(devWalletPW)
	devWallet.Open(devWalletPW)
	bc.cfg.DevAddress = devWallet.GetAddress()
	fmt.Printf("A Blockchain project Dev wallet was created for you with address [%s] and password [%s] (you can change this later)\n", bc.cfg.DevAddress, devWalletPW)

	minerWalletPW, err := GenerateRandomPassword()
	if err != nil {
		return err
	}

	minerWallet, err := NewWallet(NewWalletOptions(ThisBlockchainOrganizationID, ThisBlockchainAppID, ThisBlockchainAdminUserID, ThisBlockchainMinerID, "Miner", minerWalletPW, []string{"blockchain", "node", "miner"}))
	if err != nil {
		return err
	}
	minerWallet.Close(minerWalletPW)
	minerWallet.Open(minerWalletPW)
	bc.cfg.MinerAddress = minerWallet.GetAddress()
	fmt.Printf("A Node miner wallet was created for you with address [%s] and password [%s] (you can change this later)\n", bc.cfg.MinerAddress, minerWalletPW)

	cbTX, err := NewCoinbaseTransaction(devWallet, devWallet, bc.cfg)
	if err != nil {
		return err
	}

	err = devWallet.SetData("balance", bc.cfg.TokenCount)
	if err != nil {
		return err
	}

	cbTX.Signature, err = cbTX.Sign([]byte(devWallet.PrivatePEM()))
	if err != nil {
		return err
	}
	fmt.Printf("A Coinbase Transaction was created and set Dev wallet Balance to [%d] tokens)\n", cbTX.TokenCount)

	genesisTxs = append(genesisTxs, cbTX)

	bankTX, err := NewBankTransaction(devWallet, minerWallet, bc.cfg.FundWalletAmount)
	if err != nil {
		return err
	}

	bankTX.Signature, err = bankTX.Sign([]byte(devWallet.PrivatePEM()))
	if err != nil {
		return err
	}
	fmt.Printf("A Bank Transaction was created sent [%0.4f] tokens to the miner wallet)\n", bankTX.Amount)

	genesisTxs = append(genesisTxs, bankTX)

	bc.GenerateGenesisBlock(genesisTxs)

	return nil
}

// GenerateGenesisBlock generates the genesis block if there are no existing blocks.
func (bc *Blockchain) GenerateGenesisBlock(txs []Transaction) {
	if len(bc.Blocks) == 0 {
		fmt.Printf("[%s] Generating Genesis Block...\n", time.Now().Format(logDateTimeFormat))

		genesisBlock := NewBlock([]Transaction{}, "")
		genesisBlock.Index = *big.NewInt(0)

		if len(txs) > 0 {
			genesisBlock.Transactions = txs
			bc.TransactionQueue = nil
		}

		genesisBlock.Hash = bc.generateHash(genesisBlock)

		bc.Mine(genesisBlock, 1)

		bc.Blocks = append(bc.Blocks, genesisBlock)

		err := bc.TXLookup.Add(genesisBlock)
		if err != nil {
			fmt.Printf("[%s] Error adding block to TXLookup: %v\n", time.Now().Format(logDateTimeFormat), err)
		}

		fmt.Printf("[%s] Genesis Block created with Hash [%s]\n", time.Now().Format(logDateTimeFormat), genesisBlock.Hash)

		err = bc.Save()
		if err != nil {
			fmt.Printf("[%s] Error saving blockchain state: %v\n", time.Now().Format(logDateTimeFormat), err)
		}
	}
}

// HasTransaction checks if a transaction with the given ID exists in the blockchain.
func (bc *Blockchain) HasTransaction(id *PUID) bool {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	for _, tx := range bc.TransactionQueue {
		if tx.GetID() == id.String() {
			return true
		}
	}

	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.GetID() == id.String() {
				return true
			}
		}
	}

	return false
}

// LoadExistingBlocks loads any existing blocks from disk and appends them to the blockchain.
func (bc *Blockchain) LoadExistingBlocks() error {
	files, _ := filepath.Glob(fmt.Sprintf("%s/*.json", blockFolder))
	if len(files) == 0 {
		fmt.Printf("[%s] No existing Blocks\n", time.Now().Format(logDateTimeFormat))
		bc.createBlockchain()
		return nil
	}

	fmt.Printf("[%s] Loading Blockchain [%d]...\n", time.Now().Format(logDateTimeFormat), len(files))

	// TODO: Implement block loading logic here

	fmt.Printf("[%s] Done\n", time.Now().Format(logDateTimeFormat))

	return nil
}

// AddTransaction adds a new transaction to the transaction queue.
func (bc *Blockchain) AddTransaction(transaction Transaction) {
	bc.mux.Lock()
	transaction.Hash()
	bc.TransactionQueue = append(bc.TransactionQueue, transaction)
	bc.mux.Unlock()
	fmt.Printf("[%s] Added TX to queue: %v\n", time.Now().Format(logDateTimeFormat), transaction)
}

// Mine attempts to mine a new block for the blockchain.
func (bc *Blockchain) Mine(block *Block, difficulty int) *Block {
	prefix := strings.Repeat("0", difficulty)

	for i := 0; i < maxNonce; i++ {
		block.Header.Nonce = uint32(i)
		block.Hash = block.CalculateHash()

		if strings.HasPrefix(block.Hash, prefix) {
			block.save()
			bc.Blocks = append(bc.Blocks, block)
			bc.TransactionQueue = []Transaction{}

			fmt.Printf("[%s] Mined a new Block with [%d] TXs & Hash [%s]\n", time.Now().Format(logDateTimeFormat), len(block.Transactions), block.Hash)
			break
		}
	}

	return block
}

// VerifySignature verifies the signature of the given transaction.
func (bc *Blockchain) VerifySignature(tx Transaction) error {
	_, err := tx.Verify([]byte(tx.GetSenderWallet().PublicPEM()), tx.GetSignature())
	return err
}

// Run is a long-running function that manages the blockchain.
func (bc *Blockchain) Run(difficulty int) {
	statusTicker := time.NewTicker(time.Second)
	blockTicker := time.NewTicker(time.Duration(bc.cfg.BlockTime) * time.Second)

	go func() {
		for range statusTicker.C {
			bc.DisplayStatus()
		}
	}()

	go func() {
		for range blockTicker.C {
			bc.mux.Lock()
			if len(bc.TransactionQueue) == 0 {
				bc.mux.Unlock()
				continue
			}

			index := len(bc.Blocks)
			previousHash := ""
			if index > 0 {
				previousHash = bc.Blocks[index-1].Hash
			}

			block := NewBlock(bc.TransactionQueue, previousHash)
			block.Index = *big.NewInt(int64(index))
			bc.Mine(block, difficulty)

			err := bc.TXLookup.Add(block)
			if err != nil {
				fmt.Printf("[%s] Error adding block to TXLookup: %v\n", time.Now().Format(logDateTimeFormat), err)
			}

			block.save()

			err = bc.Save()
			if err != nil {
				fmt.Printf("[%s] Error saving blockchain state: %v\n", time.Now().Format(logDateTimeFormat), err)
			}

			bc.mux.Unlock()
		}
	}()
}

// generateHash generates a SHA-512 hash for the given block.
func (bc *Blockchain) generateHash(block *Block) string {
	record := block.Index.Text(10) + block.Header.Timestamp.String() + strconv.FormatUint(uint64(block.Header.Nonce), 10) + block.Header.PreviousHash
	h := sha512.New()
	h.Write([]byte(record))
	hashed := h.Sum(nil)
	return hex.EncodeToString(hashed)
}

// GetLatestBlock returns the latest block in the blockchain.
func (bc *Blockchain) GetLatestBlock() *Block {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	if len(bc.Blocks) == 0 {
		return nil
	}
	return bc.Blocks[len(bc.Blocks)-1]
}

// GetBlockByHash returns a block with the given hash.
func (bc *Blockchain) GetBlockByHash(hash string) *Block {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	for _, block := range bc.Blocks {
		if block.Hash == hash {
			return block
		}
	}
	return nil
}

// GetBlockByIndex returns a block at the given index.
func (bc *Blockchain) GetBlockByIndex(index int64) *Block {
	bc.mux.Lock()
	defer bc.mux.Unlock()
	if index < 0 || int(index) >= len(bc.Blocks) {
		return nil
	}
	return bc.Blocks[index]
}

// GetTransactionByID returns a transaction with the given ID.
func (bc *Blockchain) GetTransactionByID(id string) Transaction {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	// First, check the transaction queue
	for _, tx := range bc.TransactionQueue {
		if tx.GetID() == id {
			return tx
		}
	}

	// Then, check all blocks
	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.GetID() == id {
				return tx
			}
		}
	}

	return nil
}

// GetBalance returns the balance of a given wallet address.
func (bc *Blockchain) GetBalance(address string) float64 {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	balance := 0.0
	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.GetSenderWallet().GetAddress() == address {
				balance -= tx.GetFee()
				if bankTx, ok := tx.(*Bank); ok {
					balance -= bankTx.Amount
				}
			}
			if tx.GetProtocol() == BankProtocolID {
				if bankTx, ok := tx.(*Bank); ok {
					if bankTx.To.GetAddress() == address {
						balance += bankTx.Amount
					}
				}
			}
		}
	}
	return balance
}

// CalculateTotalSupply calculates the total supply of tokens in the blockchain.
func (bc *Blockchain) CalculateTotalSupply() float64 {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	totalSupply := 0.0
	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.GetProtocol() == CoinbaseProtocolID {
				if coinbaseTx, ok := tx.(*Coinbase); ok {
					totalSupply += float64(coinbaseTx.TokenCount)
				}
			}
		}
	}
	return totalSupply
}

// ValidateChain validates the entire blockchain.
func (bc *Blockchain) ValidateChain() error {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	for i := 1; i < len(bc.Blocks); i++ {
		currentBlock := bc.Blocks[i]
		previousBlock := bc.Blocks[i-1]

		if currentBlock.Header.PreviousHash != previousBlock.Hash {
			return fmt.Errorf("invalid previous hash at block %d", i)
		}

		if currentBlock.Hash != currentBlock.CalculateHash() {
			return fmt.Errorf("invalid hash at block %d", i)
		}

		if err := currentBlock.Validate(previousBlock); err != nil {
			return fmt.Errorf("invalid block at index %d: %v", i, err)
		}

		for _, tx := range currentBlock.Transactions {
			if err := tx.Validate(); err != nil {
				return fmt.Errorf("invalid transaction %s in block %d: %v", tx.GetID(), i, err)
			}
		}
	}

	return nil
}

// GetTransactionHistory returns the transaction history for a given wallet address.
func (bc *Blockchain) GetTransactionHistory(address string) []Transaction {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	var history []Transaction

	for _, block := range bc.Blocks {
		for _, tx := range block.Transactions {
			if tx.GetSenderWallet().GetAddress() == address || (tx.GetProtocol() == BankProtocolID && tx.(*Bank).To.GetAddress() == address) {
				history = append(history, tx)
			}
		}
	}

	return history
}

// GetPendingTransactions returns all pending transactions in the queue.
func (bc *Blockchain) GetPendingTransactions() []Transaction {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	return bc.TransactionQueue
}

// RemoveTransaction removes a transaction from the pending queue.
func (bc *Blockchain) RemoveTransaction(id string) bool {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	for i, tx := range bc.TransactionQueue {
		if tx.GetID() == id {
			bc.TransactionQueue = append(bc.TransactionQueue[:i], bc.TransactionQueue[i+1:]...)
			return true
		}
	}

	return false
}

// UpdateConfig updates the blockchain configuration.
func (bc *Blockchain) UpdateConfig(newConfig *Config) error {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	// Validate the new configuration
	if err := newConfig.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %v", err)
	}

	// Update the configuration
	bc.cfg = newConfig

	// Save the updated configuration
	return bc.Save()
}

// GetBlockchainInfo returns general information about the blockchain.
func (bc *Blockchain) GetBlockchainInfo() BlockchainInfo {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	return BlockchainInfo{
		Version:    BlockchainVersion,
		Name:       bc.cfg.BlockchainName,
		Symbol:     bc.cfg.BlockchainSymbol,
		BlockTime:  bc.cfg.BlockTime,
		Difficulty: bc.cfg.Difficulty,
		Fee:        bc.cfg.TransactionFee,
	}
}

// GetMempoolSize returns the number of transactions in the mempool (transaction queue).
func (bc *Blockchain) GetMempoolSize() int {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	return len(bc.TransactionQueue)
}

// GetBlockCount returns the total number of blocks in the blockchain.
func (bc *Blockchain) GetBlockCount() int {
	bc.mux.Lock()
	defer bc.mux.Unlock()

	return len(bc.Blocks)
}
