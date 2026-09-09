// Package sdk is a software development kit for building blockchain applications.
// File sdk/blockchain_persist.go - Chain persistence: loading, saving and genesis creation.
package sdk

import (
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

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
		if err := bc.TXLookup.Set(data.TXLookup); err != nil {
			log.Printf("Error setting TX lookup: %v", err)
		}
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
	bc.mux.Lock()
	defer bc.mux.Unlock()
	return bc.saveLocked()
}

func (bc *Blockchain) saveLocked() error {
	data := &BlockchainPersistData{
		TXLookup:       bc.TXLookup.index.Get(),
		CurrBlockIndex: &bc.CurrentBlockIndex,
		NextBlockIndex: &bc.NextBlockIndex,
	}

	return localStorage.Set("state", data)
}

// createBlockchain initializes a new blockchain with a genesis block and sets up
// the necessary wallets and transactions. It performs the following steps:
// 1. Initializes blockchain organization, application, admin user, developer asset, and miner asset IDs.
// 2. Creates a developer wallet with a randomly generated password and assigns it to the blockchain configuration.
// 3. Creates a miner wallet with a randomly generated password and assigns it to the blockchain configuration.
// 4. Generates a coinbase transaction to set the initial balance of the developer wallet.
// 5. Generates a bank transaction to fund the miner wallet with a specified amount.
// 6. Generates the genesis block with the created transactions.
//
// Returns an error if any step in the process fails.
func (bc *Blockchain) createBlockchain() error {
	LogInfof("Initializing new blockchain...")
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
		return fmt.Errorf("failed to create dev wallet: %w", err)
	}

	err = devWallet.Close(devWalletPW)
	if err != nil {
		return fmt.Errorf("failed to close dev wallet: %w", err)
	}

	err = devWallet.Open(devWalletPW)
	if err != nil {
		return fmt.Errorf("failed to open dev wallet: %w", err)
	}

	bc.cfg.DevAddress = devWallet.GetAddress()
	LogVerbosef("Dev wallet created: %s (password: %s)", bc.cfg.DevAddress, devWalletPW)

	minerWalletPW, err := GenerateRandomPassword()
	if err != nil {
		return err
	}

	minerWallet, err := NewWallet(NewWalletOptions(ThisBlockchainOrganizationID, ThisBlockchainAppID, ThisBlockchainAdminUserID, ThisBlockchainMinerID, "Miner", minerWalletPW, []string{"blockchain", "node", "miner"}))
	if err != nil {
		return fmt.Errorf("failed to create miner wallet: %w", err)
	}

	if err = minerWallet.Close(minerWalletPW); err != nil {
		return fmt.Errorf("failed to close miner wallet: %w", err)
	}

	if err = minerWallet.Open(minerWalletPW); err != nil {
		return fmt.Errorf("failed to open miner wallet: %w", err)
	}

	bc.cfg.MinerAddress = minerWallet.GetAddress()
	LogVerbosef("Miner wallet created: %s (password: %s)", bc.cfg.MinerAddress, minerWalletPW)

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
	LogVerbosef("Coinbase transaction created: %d tokens allocated", cbTX.TokenCount)

	genesisTxs = append(genesisTxs, cbTX)

	bankTX, err := NewBankTransaction(devWallet, minerWallet, bc.cfg.FundWalletAmount)
	if err != nil {
		return err
	}

	bankTX.Signature, err = bankTX.Sign([]byte(devWallet.PrivatePEM()))
	if err != nil {
		return err
	}
	LogVerbosef("Bank transaction created: %.2f tokens transferred to miner", bankTX.Amount)

	genesisTxs = append(genesisTxs, bankTX)

	bc.GenerateGenesisBlock(genesisTxs)

	return nil
}

// GenerateGenesisBlock generates the genesis block if there are no existing blocks.
func (bc *Blockchain) GenerateGenesisBlock(txs []Transaction) {
	if len(bc.Blocks) == 0 {
		log.Println("Generating Genesis Block...")

		genesisBlock := NewBlock(txs, "")
		genesisBlock.Index = *big.NewInt(0)
		genesisBlock.Header.Difficulty = uint32(genesisDifficulty)

		mined, err := bc.Mine(genesisBlock, 1)
		if err != nil {
			LogInfof("Failed to mine genesis block: %v", err)
			return
		}
		genesisBlock = mined

		err = genesisBlock.save()
		if err != nil {
			log.Printf("Error saving genesis block: %v\n", err)
		}

		bc.Blocks = append(bc.Blocks, genesisBlock)
		bc.mux.Lock()
		if _, err := bc.ensureUTXOSetLocked().ApplyBlock(genesisBlock, bc.feeSplitFor()); err != nil {
			log.Printf("Error applying genesis block to the UTXO set: %v", err)
		}
		bc.mux.Unlock()

		err = bc.TXLookup.Add(genesisBlock)
		if err != nil {
			log.Printf("Error adding block to TXLookup: %v\n", err)
		}

		log.Printf("Genesis Block created with Hash [%s]\n", genesisBlock.Hash)

		err = bc.Save()
		if err != nil {
			log.Printf("Error saving blockchain state: %v\n", err)
		}
	}
}

// LoadExistingBlocks loads any existing blocks from disk and appends them to the
// blockchain, in block-index order, verifying the hash chain as it goes.
//
// Two problems with the previous version: it ordered files with sort.Strings, so
// "10.json" sorted before "2.json" and blocks were appended in lexical order --
// bc.Blocks[i] was not block i, and the previousHash chain was scrambled. And it
// logged and skipped every decode failure, which (given Block could not be
// decoded at all) silently discarded the entire history on every restart.
func (bc *Blockchain) LoadExistingBlocks() error {
	blocksPath := filepath.Join(bc.cfg.DataPath, "blocks")

	// Look for both .json and .jso files (in case of truncated names)
	// Glob only fails on a malformed pattern, and both patterns are literals, so
	// the error cannot occur here.
	jsonFiles, _ := filepath.Glob(filepath.Join(blocksPath, "*.json")) //nolint:errcheck // pattern is a literal
	jsoFiles, _ := filepath.Glob(filepath.Join(blocksPath, "*.jso"))   //nolint:errcheck // pattern is a literal
	files := append(jsonFiles, jsoFiles...)
	if len(files) == 0 {
		LogVerbosef("No existing blocks found in %s", blocksPath)
		return nil
	}

	LogInfof("Loading %d existing blocks...", len(files))

	type blockFile struct {
		index int
		path  string
	}

	ordered := make([]blockFile, 0, len(files))
	for _, file := range files {
		filename := filepath.Base(file)
		blockIndexStr := strings.TrimSuffix(strings.TrimSuffix(filename, ".json"), ".jso")
		blockIndex, err := strconv.Atoi(blockIndexStr)
		if err != nil {
			LogInfof("Skipping file with non-numeric block index: %s", filename)
			continue
		}
		ordered = append(ordered, blockFile{index: blockIndex, path: file})
	}

	// Sort numerically, not lexically.
	sort.Slice(ordered, func(i, j int) bool { return ordered[i].index < ordered[j].index })

	loaded := make([]*Block, 0, len(ordered))
	for _, bf := range ordered {
		fileData, err := os.ReadFile(bf.path)
		if err != nil {
			return fmt.Errorf("error reading block file %s: %w", bf.path, err)
		}

		block := &Block{}
		if err := json.Unmarshal(fileData, block); err != nil {
			// A corrupt block is fatal, not skippable: continuing would silently
			// serve a chain with a hole in it.
			return fmt.Errorf("error parsing block %d (%s): %w", bf.index, bf.path, err)
		}

		if got := int(block.Index.Int64()); got != bf.index {
			return fmt.Errorf("block file %s contains block %d", bf.path, got)
		}

		// Verify the chain links as we load, so a gap or a tampered block is
		// detected at startup rather than at some arbitrary later point.
		if len(loaded) > 0 {
			previous := loaded[len(loaded)-1]
			if block.Header.PreviousHash != previous.Hash {
				return fmt.Errorf("block %d does not link to block %d (previous hash mismatch)",
					bf.index, int(previous.Index.Int64()))
			}
		}

		loaded = append(loaded, block)
		if err := bc.TXLookup.Add(block); err != nil {
			LogInfof("Error adding block %d to TXLookup: %v", bf.index, err)
		}
	}

	bc.mux.Lock()
	defer bc.mux.Unlock()

	bc.Blocks = loaded
	LogInfof("Successfully loaded %d blocks", len(bc.Blocks))

	if len(bc.Blocks) > 0 {
		lastBlockIndex := int(bc.Blocks[len(bc.Blocks)-1].Index.Int64())
		bc.CurrentBlockIndex = lastBlockIndex
		bc.NextBlockIndex = lastBlockIndex + 1
		LogVerbosef("Updated blockchain state: current_block_index=%d, next_block_index=%d",
			bc.CurrentBlockIndex, bc.NextBlockIndex)
	}

	return nil
}
