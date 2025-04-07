package sdk

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

const (
	baseURL = "http://localhost:8100"
	apiKey  = "69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f"
)

// createAuthorizedRequest creates an HTTP request with the API key
func createAuthorizedRequest(method, url string, body []byte) (*http.Request, error) {
	req, err := http.NewRequest(method, url, bytes.NewBuffer(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")
	return req, nil
}

// executeAuthorizedRequest executes an authorized HTTP request
func executeAuthorizedRequest(method, url string, body []byte) (*http.Response, error) {
	req, err := createAuthorizedRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

// Helper function to initialize test node if not already initialized
func initializeTestNode(t *testing.T) {
	if GetNode() == nil {
		t.Log("Initializing test node...")

		// Prepare node options
		nodeOpts := DefaultNodeOptions()
		nodeOpts.IsSeed = true
		nodeOpts.DataPath = "./test_data"

		// Ensure data path exists
		err := os.MkdirAll(nodeOpts.DataPath, 0755)
		if err != nil {
			t.Fatalf("Failed to create test data path: %v", err)
		}

		// Create a new configuration if not exists
		if nodeOpts.Config == nil {
			nodeOpts.Config = NewConfig()
		}
		nodeOpts.Config.DataPath = nodeOpts.DataPath

		// Create the node
		err = NewNode(nodeOpts)
		if err != nil {
			t.Fatalf("Failed to create test node: %v", err)
		}
	}
}

func TestBlockchainAPI(t *testing.T) {
	// Initialize test node before running API tests
	initializeTestNode(t)

	// Test suite for Blockchain API endpoints
	t.Run("Version Endpoint", testVersionEndpoint)
	t.Run("Info Endpoint", testInfoEndpoint)
	t.Run("Health Endpoint", testHealthEndpoint)
	t.Run("Blockchain Endpoint", testBlockchainEndpoint)
	t.Run("Blocks Endpoints", testBlocksEndpoints)
	t.Run("Wallet Creation", testWalletCreation)
	t.Run("Transaction Creation", testTransactionCreation)
}

func testVersionEndpoint(t *testing.T) {
	resp, err := http.Get(baseURL + "/version")
	if err != nil {
		t.Fatalf("Failed to get version: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}

	var versionInfo BlockchainInfo
	err = json.NewDecoder(resp.Body).Decode(&versionInfo)
	if err != nil {
		t.Fatalf("Failed to decode version info: %v", err)
	}

	if versionInfo.Version == "" {
		t.Error("Version should not be empty")
	}
}

func testInfoEndpoint(t *testing.T) {
	resp, err := http.Get(baseURL + "/info")
	if err != nil {
		t.Fatalf("Failed to get info: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}

	var blockchainInfo BlockchainInfo
	err = json.NewDecoder(resp.Body).Decode(&blockchainInfo)
	if err != nil {
		t.Fatalf("Failed to decode blockchain info: %v", err)
	}

	// Validate blockchain info fields
	if blockchainInfo.Name == "" {
		t.Error("Blockchain name should not be empty")
	}
	if blockchainInfo.Symbol == "" {
		t.Error("Blockchain symbol should not be empty")
	}
	if blockchainInfo.BlockTime <= 0 {
		t.Error("Block time should be positive")
	}
}

func testHealthEndpoint(t *testing.T) {
	resp, err := http.Get(baseURL + "/health")
	if err != nil {
		t.Fatalf("Failed to get health: %v", err)
	}
	defer resp.Body.Close()

	// Currently the health endpoint returns "Not Yet Implemented"
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}
}

func testBlockchainEndpoint(t *testing.T) {
	resp, err := executeAuthorizedRequest("GET", baseURL+"/blockchain", nil)
	if err != nil {
		t.Fatalf("Failed to get blockchain status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}

	var blockchainStatus struct {
		NumBlocks              int `json:"num_blocks"`
		NumTransactionsInQueue int `json:"num_transactions_in_queue"`
	}
	err = json.NewDecoder(resp.Body).Decode(&blockchainStatus)
	if err != nil {
		t.Fatalf("Failed to decode blockchain status: %v", err)
	}

	if blockchainStatus.NumBlocks < 0 {
		t.Error("Number of blocks should not be negative")
	}
}

func testBlocksEndpoints(t *testing.T) {
	// Test list of blocks
	resp, err := executeAuthorizedRequest("GET", baseURL+"/blockchain/blocks", nil)
	if err != nil {
		t.Fatalf("Failed to get blocks: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}

	// Use a dynamic type to handle JSON unmarshaling
	var blocks []map[string]interface{}
	err = json.NewDecoder(resp.Body).Decode(&blocks)
	if err != nil {
		t.Fatalf("Failed to decode blocks: %v", err)
	}

	if len(blocks) == 0 {
		t.Error("There should be at least one block (genesis block)")
	}

	// Optional: Print block details for debugging
	for i, block := range blocks {
		t.Logf("Block %d: %+v", i, block)
	}
}

func testWalletCreation(t *testing.T) {
	// Test wallet creation endpoint
	resp, err := executeAuthorizedRequest("GET", baseURL+"/blockchain/wallets/new", nil)
	if err != nil {
		t.Fatalf("Failed to create wallet: %v", err)
	}
	defer resp.Body.Close()

	// Currently returns "Not Yet Implemented"
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}
}

func testTransactionCreation(t *testing.T) {
	// This is a mock transaction creation test
	// In a real scenario, you'd need to create wallets first
	transaction := map[string]interface{}{
		"protocol": "BANK",
		"from":     "sender_address",
		"to":       "recipient_address",
		"amount":   100.0,
	}

	jsonData, _ := json.Marshal(transaction)
	resp, err := executeAuthorizedRequest("POST", baseURL+"/blockchain/wallets/tx", jsonData)
	if err != nil {
		t.Fatalf("Failed to create transaction: %v", err)
	}
	defer resp.Body.Close()

	// Currently returns "Not Yet Implemented"
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status OK, got %v", resp.Status)
	}
}

func TestBankTransaction(t *testing.T) {
	// Ensure node is initialized
	initializeTestNode(t)

	// Get the node
	node := GetNode()
	if node == nil {
		t.Fatal("Failed to get node instance")
	}

	// Generate strong passwords
	pass1, err := GenerateRandomPassword()
	if err != nil {
		t.Fatalf("Failed to generate password for wallet 1: %v", err)
	}

	pass2, err := GenerateRandomPassword()
	if err != nil {
		t.Fatalf("Failed to generate password for wallet 2: %v", err)
	}

	// Create two wallets
	wallet1Opts := NewWalletOptions(
		NewBigInt(1),     // OrganizationID
		NewBigInt(1),     // AppID
		NewBigInt(1),     // UserID
		NewBigInt(0),     // AssetID
		"TestWallet1",    // Name
		pass1,            // Passphrase (generated strong password)
		[]string{"test"}, // Tags
	)

	wallet2Opts := NewWalletOptions(
		NewBigInt(1),     // OrganizationID
		NewBigInt(1),     // AppID
		NewBigInt(2),     // UserID
		NewBigInt(0),     // AssetID
		"TestWallet2",    // Name
		pass2,            // Passphrase (generated strong password)
		[]string{"test"}, // Tags
	)

	wallet1, err := NewWallet(wallet1Opts)
	if err != nil {
		t.Fatalf("Failed to create wallet 1: %v", err)
	}

	wallet2, err := NewWallet(wallet2Opts)
	if err != nil {
		t.Fatalf("Failed to create wallet 2: %v", err)
	}

	// Open/Unlock wallets
	err = wallet1.Open(pass1)
	if err != nil {
		t.Fatalf("Failed to open wallet 1: %v", err)
	}

	err = wallet2.Open(pass2)
	if err != nil {
		t.Fatalf("Failed to open wallet 2: %v", err)
	}

	// Manually set wallet balances
	err = wallet1.SetData("balance", 100.0)
	if err != nil {
		t.Fatalf("Failed to set balance for wallet 1: %v", err)
	}

	err = wallet2.SetData("balance", 50.0)
	if err != nil {
		t.Fatalf("Failed to set balance for wallet 2: %v", err)
	}

	// Verify wallet balances
	balance1 := wallet1.GetBalance()
	balance2 := wallet2.GetBalance()
	t.Logf("Wallet 1 Balance: %.2f", balance1)
	t.Logf("Wallet 2 Balance: %.2f", balance2)

	// Ensure sufficient balance for transaction
	transactionAmount := 10.0

	// Create a bank transaction
	bankTx, err := NewBankTransaction(wallet1, wallet2, transactionAmount)
	if err != nil {
		t.Fatalf("Failed to create bank transaction: %v", err)
	}

	// Sign the transaction
	signature, err := bankTx.Sign([]byte(wallet1.PrivatePEM()))
	if err != nil {
		t.Fatalf("Failed to sign transaction: %v", err)
	}
	bankTx.Signature = signature

	// Send the transaction to the blockchain
	err = bankTx.Send(node.Blockchain)
	if err != nil {
		t.Fatalf("Failed to send transaction: %v", err)
	}

	// Wait a bit for the transaction to be processed
	time.Sleep(2 * time.Second)

	// Verify the transaction was added to the transaction queue
	pendingTxs := node.Blockchain.GetPendingTransactions()
	found := false
	for _, tx := range pendingTxs {
		if tx.GetID() == bankTx.GetID() {
			found = true
			break
		}
	}

	if !found {
		t.Error("Transaction was not added to the transaction queue")
	}

	// Optional: Verify wallet balances after transaction
	t.Logf("Wallet 1 Balance After Tx: %.2f", wallet1.GetBalance())
	t.Logf("Wallet 2 Balance After Tx: %.2f", wallet2.GetBalance())
}
