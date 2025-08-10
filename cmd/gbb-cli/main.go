package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"
)

type BlockchainClient struct {
	apiURL     string
	httpClient *http.Client
	apiKey     string
}

func NewBlockchainClient() *BlockchainClient {
	return &BlockchainClient{
		apiURL: "http://localhost:8100",
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		apiKey: "69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f",
	}
}

func (bc *BlockchainClient) Connect() error {
	// Test connection to the API
	resp, err := bc.httpClient.Get(bc.apiURL + "/health")
	if err != nil {
		return fmt.Errorf("cannot connect to blockchain API: %v. Please start the blockchain first with: ./bin/release/gbbd", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("blockchain API returned status %d. Please start the blockchain first with: ./bin/release/gbbd", resp.StatusCode)
	}

	return nil
}

func (bc *BlockchainClient) IsConnected() bool {
	// Test connection
	resp, err := bc.httpClient.Get(bc.apiURL + "/health")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK
}

func (bc *BlockchainClient) makeRequest(method, endpoint string) ([]byte, error) {
	req, err := http.NewRequest(method, bc.apiURL+endpoint, nil)
	if err != nil {
		return nil, err
	}

	// Add API key authentication
	req.Header.Set("Authorization", "Bearer "+bc.apiKey)

	resp, err := bc.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	return body, nil
}

func (bc *BlockchainClient) GetBlockchainInfo() (map[string]interface{}, error) {
	body, err := bc.makeRequest("GET", "/blockchain")
	if err != nil {
		return nil, err
	}

	var info map[string]interface{}
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, err
	}

	return info, nil
}

func (bc *BlockchainClient) GetBlocks(page, limit int) ([]interface{}, error) {
	endpoint := fmt.Sprintf("/blockchain/blocks?page=%d&limit=%d", page, limit)
	body, err := bc.makeRequest("GET", endpoint)
	if err != nil {
		return nil, err
	}

	var blocks []interface{}
	if err := json.Unmarshal(body, &blocks); err != nil {
		return nil, err
	}

	return blocks, nil
}

func (bc *BlockchainClient) GetBlock(index int) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/blockchain/blocks/%d", index)
	body, err := bc.makeRequest("GET", endpoint)
	if err != nil {
		return nil, err
	}

	var block map[string]interface{}
	if err := json.Unmarshal(body, &block); err != nil {
		return nil, err
	}

	return block, nil
}

func (bc *BlockchainClient) GetWallets(page, limit int) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/blockchain/wallets?page=%d&limit=%d", page, limit)
	body, err := bc.makeRequest("GET", endpoint)
	if err != nil {
		return nil, err
	}

	var wallets map[string]interface{}
	if err := json.Unmarshal(body, &wallets); err != nil {
		return nil, err
	}

	return wallets, nil
}

func (bc *BlockchainClient) GetWallet(id string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/blockchain/wallets/%s", id)
	body, err := bc.makeRequest("GET", endpoint)
	if err != nil {
		return nil, err
	}

	var wallet map[string]interface{}
	if err := json.Unmarshal(body, &wallet); err != nil {
		return nil, err
	}

	return wallet, nil
}

func (bc *BlockchainClient) GetTransactions(page, limit int) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/blockchain/transactions?page=%d&limit=%d", page, limit)
	body, err := bc.makeRequest("GET", endpoint)
	if err != nil {
		return nil, err
	}

	var transactions map[string]interface{}
	if err := json.Unmarshal(body, &transactions); err != nil {
		return nil, err
	}

	return transactions, nil
}

func (bc *BlockchainClient) GetTransaction(id string) (map[string]interface{}, error) {
	endpoint := fmt.Sprintf("/blockchain/transactions/%s", id)
	body, err := bc.makeRequest("GET", endpoint)
	if err != nil {
		return nil, err
	}

	var transaction map[string]interface{}
	if err := json.Unmarshal(body, &transaction); err != nil {
		return nil, err
	}

	return transaction, nil
}

func main() {
	fmt.Println("🚀 Go Basic Blockchain CLI")
	fmt.Println("==========================")

	client := NewBlockchainClient()

	// Set up signal handler for clean shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		fmt.Println("\nShutting down CLI...")
		os.Exit(0)
	}()

	// Try to connect to the running blockchain
	fmt.Println("Connecting to running blockchain...")
	if err := client.Connect(); err != nil {
		fmt.Printf("❌ Connection failed: %v\n", err)
		fmt.Println("\n💡 To start the blockchain, run:")
		fmt.Println("   ./bin/release/gbbd")
		fmt.Println("\nThen run this CLI again.")
		os.Exit(1)
	}

	// Get blockchain info to show connection status
	info, err := client.GetBlockchainInfo()
	if err != nil {
		fmt.Printf("❌ Failed to get blockchain info: %v\n", err)
		fmt.Println("Note: API may require authentication or the blockchain may not be fully started")
		os.Exit(1)
	}

	if blockCount, ok := info["block_count"].(float64); ok {
		fmt.Printf("✅ Connected to running blockchain with %.0f blocks\n", blockCount)
	} else {
		fmt.Println("✅ Connected to running blockchain")
	}

	// Start interactive CLI
	runCLI(client)
}

func runCLI(client *BlockchainClient) {
	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Println("\n📋 Available Commands:")
		fmt.Println("  status        - Show blockchain status")
		fmt.Println("  blocks        - List all blocks")
		fmt.Println("  block <id>    - View specific block")
		fmt.Println("  wallets       - List all wallets")
		fmt.Println("  wallet <id>   - View specific wallet")
		fmt.Println("  transactions  - List all transactions")
		fmt.Println("  tx <id>       - View specific transaction")
		fmt.Println("  help          - Show this help")
		fmt.Println("  quit          - Exit CLI")
		fmt.Print("\n> ")

		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Printf("Error reading input: %v\n", err)
			continue
		}

		input = strings.TrimSpace(input)
		parts := strings.Fields(input)

		if len(parts) == 0 {
			continue
		}

		command := strings.ToLower(parts[0])

		switch command {
		case "status":
			showStatus(client)

		case "blocks":
			showBlocks(client, 1, 10)

		case "block":
			if len(parts) < 2 {
				fmt.Println("❌ Usage: block <block_index>")
				continue
			}
			index, err := strconv.Atoi(parts[1])
			if err != nil {
				fmt.Println("❌ Invalid block index")
				continue
			}
			showBlock(client, index)

		case "wallets":
			showWallets(client, 1, 10)

		case "wallet":
			if len(parts) < 2 {
				fmt.Println("❌ Usage: wallet <wallet_id>")
				continue
			}
			showWallet(client, parts[1])

		case "transactions":
			showTransactions(client, 1, 10)

		case "tx":
			if len(parts) < 2 {
				fmt.Println("❌ Usage: tx <transaction_id>")
				continue
			}
			showTransaction(client, parts[1])

		case "help":
			showHelp()

		case "quit", "exit":
			fmt.Println("Goodbye!")
			os.Exit(0)

		default:
			fmt.Printf("Unknown command: %s\n", command)
			fmt.Println("Type 'help' for available commands")
		}
	}
}

func showHelp() {
	fmt.Println("\n📖 CLI Help:")
	fmt.Println("============")
	fmt.Println("  status        - Show blockchain status and statistics")
	fmt.Println("  blocks        - List all blocks (paginated)")
	fmt.Println("  block <id>    - View specific block by index")
	fmt.Println("  wallets       - List all wallets (paginated)")
	fmt.Println("  wallet <id>   - View specific wallet by ID")
	fmt.Println("  transactions  - List all transactions (paginated)")
	fmt.Println("  tx <id>       - View specific transaction by ID")
	fmt.Println("  help          - Show this help message")
	fmt.Println("  quit          - Exit the CLI")
	fmt.Println("\n💡 All data is retrieved from the running blockchain via API")
}

func showStatus(client *BlockchainClient) {
	if !client.IsConnected() {
		fmt.Println("❌ Not connected to blockchain")
		return
	}

	fmt.Println("\n📊 Blockchain Status:")

	info, err := client.GetBlockchainInfo()
	if err != nil {
		fmt.Printf("❌ Failed to get status: %v\n", err)
		return
	}

	fmt.Printf("  Connected: %t\n", client.IsConnected())

	if blockCount, ok := info["block_count"].(float64); ok {
		fmt.Printf("  Blocks: %.0f\n", blockCount)
	}

	if difficulty, ok := info["difficulty"].(float64); ok {
		fmt.Printf("  Difficulty: %.0f\n", difficulty)
	}

	if blockTime, ok := info["block_time"].(float64); ok {
		fmt.Printf("  Block Time: %.0f seconds\n", blockTime)
	}

	if latestBlock, ok := info["latest_block"].(map[string]interface{}); ok {
		if hash, ok := latestBlock["hash"].(string); ok {
			fmt.Printf("  Latest Hash: %s\n", hash)
		}
		if index, ok := latestBlock["index"].(float64); ok {
			fmt.Printf("  Latest Block Index: %.0f\n", index)
		}
	}

	fmt.Println("  ✅ Connected to running blockchain via API")
}

func showBlocks(client *BlockchainClient, page, limit int) {
	if !client.IsConnected() {
		fmt.Println("❌ Not connected to blockchain")
		return
	}

	fmt.Printf("\n📦 Blocks (Page %d, Limit %d):\n", page, limit)

	blocks, err := client.GetBlocks(page, limit)
	if err != nil {
		fmt.Printf("❌ Failed to get blocks: %v\n", err)
		return
	}

	if len(blocks) == 0 {
		fmt.Println("  No blocks found")
		return
	}

	for _, block := range blocks {
		if blockMap, ok := block.(map[string]interface{}); ok {
			index := blockMap["index"]
			hash := blockMap["hash"]
			timestamp := blockMap["timestamp"]
			txCount := blockMap["transaction_count"]

			fmt.Printf("  Block #%v: %v (TXs: %v, Time: %v)\n", index, hash, txCount, timestamp)
		}
	}
}

func showBlock(client *BlockchainClient, index int) {
	if !client.IsConnected() {
		fmt.Println("❌ Not connected to blockchain")
		return
	}

	fmt.Printf("\n📦 Block #%d:\n", index)

	block, err := client.GetBlock(index)
	if err != nil {
		fmt.Printf("❌ Failed to get block: %v\n", err)
		return
	}

	// Pretty print the block data
	prettyPrintMap(block, "  ")
}

func showWallets(client *BlockchainClient, page, limit int) {
	if !client.IsConnected() {
		fmt.Println("❌ Not connected to blockchain")
		return
	}

	fmt.Printf("\n👛 Wallets (Page %d, Limit %d):\n", page, limit)

	wallets, err := client.GetWallets(page, limit)
	if err != nil {
		fmt.Printf("❌ Failed to get wallets: %v\n", err)
		return
	}

	if walletsData, ok := wallets["wallets"].([]interface{}); ok {
		if len(walletsData) == 0 {
			fmt.Println("  No wallets found")
			return
		}

		for _, wallet := range walletsData {
			if walletMap, ok := wallet.(map[string]interface{}); ok {
				id := walletMap["id"]
				name := walletMap["name"]
				balance := walletMap["balance"]

				fmt.Printf("  Wallet %v: %v (Balance: %v)\n", id, name, balance)
			}
		}
	} else {
		fmt.Println("  No wallets data available")
	}
}

func showWallet(client *BlockchainClient, id string) {
	if !client.IsConnected() {
		fmt.Println("❌ Not connected to blockchain")
		return
	}

	fmt.Printf("\n👛 Wallet %s:\n", id)

	wallet, err := client.GetWallet(id)
	if err != nil {
		fmt.Printf("❌ Failed to get wallet: %v\n", err)
		return
	}

	// Pretty print the wallet data
	prettyPrintMap(wallet, "  ")
}

func showTransactions(client *BlockchainClient, page, limit int) {
	if !client.IsConnected() {
		fmt.Println("❌ Not connected to blockchain")
		return
	}

	fmt.Printf("\n💸 Transactions (Page %d, Limit %d):\n", page, limit)

	transactions, err := client.GetTransactions(page, limit)
	if err != nil {
		fmt.Printf("❌ Failed to get transactions: %v\n", err)
		return
	}

	if txData, ok := transactions["transactions"].([]interface{}); ok {
		if len(txData) == 0 {
			fmt.Println("  No transactions found")
			return
		}

		for _, tx := range txData {
			if txMap, ok := tx.(map[string]interface{}); ok {
				id := txMap["id"]
				from := txMap["from"]
				to := txMap["to"]
				amount := txMap["amount"]
				status := txMap["status"]

				fmt.Printf("  TX %v: %v -> %v (Amount: %v, Status: %v)\n", id, from, to, amount, status)
			}
		}
	} else {
		fmt.Println("  No transactions data available")
	}
}

func showTransaction(client *BlockchainClient, id string) {
	if !client.IsConnected() {
		fmt.Println("❌ Not connected to blockchain")
		return
	}

	fmt.Printf("\n💸 Transaction %s:\n", id)

	transaction, err := client.GetTransaction(id)
	if err != nil {
		fmt.Printf("❌ Failed to get transaction: %v\n", err)
		return
	}

	// Pretty print the transaction data
	prettyPrintMap(transaction, "  ")
}

func prettyPrintMap(data map[string]interface{}, prefix string) {
	for key, value := range data {
		switch v := value.(type) {
		case map[string]interface{}:
			fmt.Printf("%s%s:\n", prefix, key)
			prettyPrintMap(v, prefix+"  ")
		case []interface{}:
			fmt.Printf("%s%s: [%d items]\n", prefix, key, len(v))
			for i, item := range v {
				if itemMap, ok := item.(map[string]interface{}); ok {
					fmt.Printf("%s  [%d]:\n", prefix, i)
					prettyPrintMap(itemMap, prefix+"    ")
				} else {
					fmt.Printf("%s  [%d]: %v\n", prefix, i, item)
				}
			}
		default:
			fmt.Printf("%s%s: %v\n", prefix, key, value)
		}
	}
}
