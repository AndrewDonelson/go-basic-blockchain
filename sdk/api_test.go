package sdk

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
	"context"
	"sync"
	"github.com/gorilla/mux"
)

const (
	baseURL = "http://localhost:8100"
	apiKey  = "69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f"
)

// testServer holds the test server instance
type testServer struct {
	server *http.Server
	wg     sync.WaitGroup
}

var (
	testServerInstance *testServer
	serverMutex        sync.Mutex
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

// startTestServer starts the API server for testing
func startTestServer(t *testing.T) {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	if testServerInstance != nil {
		return // Server already running
	}

	// Initialize test node if not already done
	initializeTestNode(t)

	// Get the node and its API
	node := GetNode()
	if node == nil {
		t.Fatal("Failed to get node instance")
	}

	if node.API == nil {
		t.Fatal("Node API is nil")
	}

	// Ensure the API is properly initialized with middleware
	api := node.API
	
	// Create a new router with middleware for testing
	router := mux.NewRouter()
	
	// Add logging middleware
	router.Use(loggingMiddleware)
	
	// Add API key middleware
	apiKeyMiddleware, err := ApiKeyMiddleware(defaulAPIKeytConfig, api.log)
	if err != nil {
		t.Fatalf("Error initializing API key middleware: %v", err)
	}
	router.Use(apiKeyMiddleware)
	
	// Register all the same routes as the original API
	router.HandleFunc("/", api.handleHome).Methods("GET")
	router.HandleFunc("/version", api.handleVersion).Methods("GET")
	router.HandleFunc("/info", api.handleInfo).Methods("GET")
	router.HandleFunc("/health", api.handleHealth).Methods("GET")
	router.HandleFunc("/account/register", api.handleAccountRegister).Methods("GET")
	router.HandleFunc("/account/login", api.handleAccountLogin).Methods("GET")
	router.HandleFunc("/account/verify", api.handleAccountVerify).Methods("GET")
	router.HandleFunc("/blockchain", api.handleBlockchain).Methods("GET")
	router.HandleFunc("/blockchain/blocks", api.handleBrowseBlocks).Methods("GET")
	router.HandleFunc("/blockchain/blocks/{index}", api.handleViewBlock).Methods("GET")
	router.HandleFunc("/blockchain/blocks/{index}/transactions", api.handleBrowseTransactionsInBlock).Methods("GET")
	router.HandleFunc("/blockchain/blocks/{index}/transactions/{id}", api.handleViewTransactionInBlock).Methods("GET")
	router.HandleFunc("/blockchain/blocks/{index}/transactions/{protocol}", api.handleBrowseTransactionsByProtocolInBlock).Methods("GET")
	router.HandleFunc("/blockchain/wallets", api.handleBrowseWallets).Methods("GET")
	router.HandleFunc("/blockchain/wallets/new", api.handleCreateWallet).Methods("GET")
	router.HandleFunc("/blockchain/wallets/{id}", api.handleViewWallet).Methods("GET")
	router.HandleFunc("/blockchain/wallets/{id}", api.handleUpdateWallet).Methods("POST")
	router.HandleFunc("/blockchain/wallets/{id}/balance", api.handleViewWalletBalance).Methods("GET")
	router.HandleFunc("/blockchain/wallets/{id}/transactions", api.handleBrowseTransactionsForWallet).Methods("GET")
	router.HandleFunc("/blockchain/wallets/{id}/transactions/{id}", api.handleViewTransactionForWallet).Methods("GET")
	router.HandleFunc("/blockchain/wallets/{id}/transactions/{protocol}", api.handleBrowseTransactionsByProtocolForWallet).Methods("GET")
	router.HandleFunc("/blockchain/transactions", api.handleBrowseTransactions).Methods("GET")
	router.HandleFunc("/blockchain/transactions/{id}", api.handleViewTransaction).Methods("GET")
	router.HandleFunc("/blockchain/transactions/{protocol}", api.handleBrowseTransactionsByProtocol).Methods("GET")

	// Create a new server instance for testing
	server := &http.Server{
		Addr:    ":8100",
		Handler: router,
	}

	testServerInstance = &testServer{
		server: server,
	}

	// Start the server in a goroutine
	testServerInstance.wg.Add(1)
	go func() {
		defer testServerInstance.wg.Done()
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Logf("Server error: %v", err)
		}
	}()

	// Wait a moment for the server to start
	time.Sleep(100 * time.Millisecond)
}

// stopTestServer stops the test API server
func stopTestServer() {
	serverMutex.Lock()
	defer serverMutex.Unlock()

	if testServerInstance != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		
		testServerInstance.server.Shutdown(ctx)
		testServerInstance.wg.Wait()
		testServerInstance = nil
	}
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
	// Start the test server
	startTestServer(t)
	defer stopTestServer()

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
