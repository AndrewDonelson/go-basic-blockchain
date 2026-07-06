// Package sdk is a software development kit for building blockchain applications.
// File sdk/api.go - API for the blockchain
package sdk

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	appservices "github.com/AndrewDonelson/go-basic-blockchain/internal/application/services"
	"github.com/gorilla/mux"
	logging "github.com/op/go-logging"
)

// ErrorResponse represents the structure of an error response.
type ErrorResponse struct {
	Message string `json:"message"`
}

// Blockchain API
//
// This is the API for the blockchain
//
//     Schemes: http
//     Host: localhost
//     BasePath: /
//     Version: 0.1.0
//
//     Consumes:
//     - application/json
//
//     Produces:
//     - application/json
//
// Endpoints:
//     	GET		/														# Home
//     	GET		/version												# Version
//     	GET		/info													# General Chain/Project Info
//     	GET		/health													# Health
//     	POST	/consensus/p2p											# P2P Broadcast Message to 1/3, then 2/3, then all nodes
//     	POST	/consensus/tx											# Incomming TX from another node that needs to be validated and returned
//     	POST	/consensus/block										# Incomming Block from another node that needs to be validated and returned
//     	GET		/blockchain												# Blockchain state
//     	GET		/blockchain/blocks										# Browse all blocks (with pagination)
//     	GET		/blockchain/blocks/{index}								# View a block
//     	GET		/blockchain/blocks/{index}/transactions					# Browse all transactions in a block (with pagination)
//     	GET		/blockchain/blocks/{index}/transactions/{id}			# View a transaction in a block
//		GET		/blockchain/blocks/{index}/transactions/{protocol}		# Browse all transactions in a block by protocol
//	 	GET		/blockchain/wallets										# Browse all wallets (with pagination)
//	 	GET		/blockchain/wallets/new									# Create a new wallet
//	 	GET		/blockchain/wallets/{id}								# View a wallet
//	 	POST	/blockchain/wallets/{id}								# Update a wallet (Name, tags, etc, Owser Only)
//	 	GET		/blockchain/wallets/{id}/balance						# View a wallet balance
//	 	GET		/blockchain/wallets/{id}/transactions					# Browse all transactions for a wallet (with pagination)
//	 	GET		/blockchain/wallets/{id}/transactions/{id}				# View a transaction for a wallet
//	 	GET		/blockchain/wallets/{id}/transactions/{protocol}		# Browse all transactions for a wallet by protocol
//	 	GET		/blockchain/transactions								# Browse all transactions (with pagination)
//	 	GET		/blockchain/transactions/{id}							# View a transaction
//	 	GET		/blockchain/transactions/{protocol}						# Browse all transactions by protocol
//
// This API is a Goroutine that is started by the main() function in main.go if the global constant `EnableAPI` is enabled.
// The API is a struct object and all endpoint methods are defined as methods on the API struct and prepended with 'handle'.
// For example, for the /blockchain endpoint, the method name would be handleBlockchain() and would be called by the API internally.

// API is the blockchain API.
type API struct {
	bc                *Blockchain
	router            *mux.Router
	log               *logging.Logger
	running           bool
	blockchainService *appservices.BlockchainService
	accountStore      AccountStore
}

var publicPaths = []string{
	"/",
	"/version",
	"/info",
	"/health",
	"/account/register",
	"/account/login",
	"/account/verify",
}

// NewAPI creates a new instance of the blockchain API.
func NewAPI(bc *Blockchain) *API {
	// Initialize the Gorilla Mux router
	api := &API{
		bc:                bc,
		log:               logging.MustGetLogger("api"),
		router:            mux.NewRouter(),
		blockchainService: appservices.NewBlockchainService(bc),
		accountStore:      NewFileAccountStore(bc.GetConfig().DataPath),
	}

	LogInfof("Initializing API...")

	// Register the API endpoints
	api.registerRoutes()
	return api
}

// RespondError sends an error response with the given status code and message.
func RespondError(w http.ResponseWriter, statusCode int, message string) {
	// Set the Content-Type header and status code
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Create an ErrorResponse instance
	errorResponse := ErrorResponse{Message: message}

	// Encode and send the error message as JSON
	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func isPublicPath(path string) bool {
	for _, publicPath := range publicPaths {
		if path == publicPath {
			return true
		}
	}
	return false
}

func authenticateNode(next http.Handler) http.Handler {
	cfg := defaultAPIKeyConfig()
	decodedAPIKeys := make(map[string][]byte)

	for name, value := range cfg.APIKeys {
		decodedKey, err := hex.DecodeString(value)
		if err != nil {
			LogInfof("invalid consensus API key configuration for %s: %v", name, err)
			continue
		}

		decodedAPIKeys[name] = decodedKey
	}

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(decodedAPIKeys) == 0 {
			http.Error(w, "Consensus authentication unavailable", http.StatusInternalServerError)
			return
		}

		apiKey, err := bearerToken(r, cfg.APIKeyHeader)
		if err != nil {
			RespondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		if _, ok := apiKeyIsValid(apiKey, decodedAPIKeys); !ok {
			RespondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Logging middleware logs the request and response details
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Get the user's remote IP address
		ip := GetUserIP(r)

		// Log the request details
		cacheReqStr := fmt.Sprintf("[%s] Request: %s %s %s", time.Now().Format(logDateTimeFormat), ip, r.Method, r.URL.Path)

		// Create a response writer wrapper to capture the response status code
		ww := &responseWriterWrapper{ResponseWriter: w}

		// Call the next handler
		next.ServeHTTP(ww, r)

		// Log the response details
		//api.logger.Printf("%s -> Response: %d %d bytes", cacheReqStr, ww.statusCode, ww.bytesWritten)
		LogVerbosef("%s -> Response: %d %d bytes", cacheReqStr, ww.statusCode, ww.bytesWritten)
	})
}

// responseWriterWrapper is a wrapper around http.ResponseWriter to capture the response status code.
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (ww *responseWriterWrapper) WriteHeader(code int) {
	ww.statusCode = code
	ww.ResponseWriter.WriteHeader(code)
}

func (ww *responseWriterWrapper) Write(data []byte) (int, error) {
	n, err := ww.ResponseWriter.Write(data)
	ww.bytesWritten += n
	return n, err
}

// IsRunning returns true if the API is running
func (api *API) IsRunning() bool {
	return api.running
}

// Start starts the API and listens for incoming requests
func (api *API) Start() {

	if api.IsRunning() {
		return
	}

	// Create a logging middleware
	api.router.Use(loggingMiddleware)

	// API key middleware
	apiKeyMiddleware, err := ApiKeyMiddleware(defaultAPIKeyConfig(), api.log)
	if err != nil {
		api.log.Fatal("Error initializing API key middleware:", err)
	}
	api.router.Use(apiKeyMiddleware)

	bindAddr := apiHostname
	if cfg := api.GetConfig(); cfg != nil && cfg.APIHostName != "" {
		bindAddr = cfg.APIHostName
	}

	// Start the HTTP server
	LogInfof("API server starting on %s", bindAddr)
	api.running = true
	api.log.Fatal(http.ListenAndServe(bindAddr, api.router))
}

// GetConfig returns the configuration used to create the API instance.
func (api *API) GetConfig() *Config {
	return api.bc.GetConfig()
}

// GetRouter returns the router for testing purposes
func (api *API) GetRouter() *mux.Router {
	return api.router
}

// registerRoutes registers the API routes.
func (api *API) registerRoutes() {
	api.router.HandleFunc("/", api.handleHome).Methods("GET") // same as /info but HTML only
	api.router.HandleFunc("/version", api.handleVersion).Methods("GET")
	api.router.HandleFunc("/info", api.handleInfo).Methods("GET") // Same as / but JSON only
	api.router.HandleFunc("/health", api.handleHealth).Methods("GET")

	// Register Public Account Endpoints
	api.router.HandleFunc("/account/register", api.handleAccountRegister).Methods("GET")
	api.router.HandleFunc("/account/login", api.handleAccountLogin).Methods("GET")
	api.router.HandleFunc("/account/verify", api.handleAccountVerify).Methods("GET")

	// Register Private Account Endpoints
	// api.router.HandleFunc("/account", api.handleAccount).Methods("GET")
	// api.router.HandleFunc("/account/logout", api.handleAccountLogout).Methods("GET")
	// api.router.HandleFunc("/account/{id}", api.handleAccount).Methods("GET")
	// api.router.HandleFunc("/account/{id}/balance", api.handleAccountBalance).Methods("GET")
	// api.router.HandleFunc("/account/{id}/transactions", api.handleAccountTransactions).Methods("GET")
	// api.router.HandleFunc("/account/{id}/transactions/{id}", api.handleAccountTransaction).Methods("GET")
	// api.router.HandleFunc("/account/{id}/transactions/{protocol}", api.handleAccountTransactionsByProtocol).Methods("GET")

	// Register the blockchain endpoints
	api.router.HandleFunc("/blockchain", api.handleBlockchain).Methods("GET")
	api.router.HandleFunc("/blockchain/blocks", api.handleBrowseBlocks).Methods("GET")
	api.router.HandleFunc("/blockchain/blocks/{index}", api.handleViewBlock).Methods("GET")
	api.router.HandleFunc("/blockchain/blocks/{index}/transactions", api.handleBrowseTransactionsInBlock).Methods("GET")
	api.router.HandleFunc("/blockchain/blocks/{index}/transactions/{id}", api.handleViewTransactionInBlock).Methods("GET")
	api.router.HandleFunc("/blockchain/blocks/{index}/transactions/{protocol}", api.handleBrowseTransactionsByProtocolInBlock).Methods("GET")
	api.router.HandleFunc("/blockchain/wallets", api.handleBrowseWallets).Methods("GET")
	api.router.HandleFunc("/blockchain/wallets/new", api.handleCreateWallet).Methods("GET")
	api.router.HandleFunc("/blockchain/wallets/tx", api.handleCreateWalletTransaction).Methods("POST")
	api.router.HandleFunc("/blockchain/wallets/{id}", api.handleViewWallet).Methods("GET")
	api.router.HandleFunc("/blockchain/wallets/{id}", api.handleUpdateWallet).Methods("POST")
	api.router.HandleFunc("/blockchain/wallets/{id}/balance", api.handleViewWalletBalance).Methods("GET")
	api.router.HandleFunc("/blockchain/wallets/{id}/transactions", api.handleBrowseTransactionsForWallet).Methods("GET")
	api.router.HandleFunc("/blockchain/wallets/{id}/transactions/{id}", api.handleViewTransactionForWallet).Methods("GET")
	api.router.HandleFunc("/blockchain/wallets/{id}/transactions/{protocol}", api.handleBrowseTransactionsByProtocolForWallet).Methods("GET")
	api.router.HandleFunc("/blockchain/transactions", api.handleBrowseTransactions).Methods("GET")
	api.router.HandleFunc("/blockchain/transactions/{id}", api.handleViewTransaction).Methods("GET")
	api.router.HandleFunc("/blockchain/transactions/{protocol}", api.handleBrowseTransactionsByProtocol).Methods("GET")

	// Create a subrouter for the consensus endpoints
	// This is only available to other regsitered/authorized nodes
	consensusRouter := mux.NewRouter().PathPrefix("/consensus").Subrouter()
	consensusRouter.Use(authenticateNode)

	consensusRouter.HandleFunc("/p2p", api.handleConsensusP2P).Methods("POST")
	consensusRouter.HandleFunc("/tx", api.handleConsensusTx).Methods("POST")
	consensusRouter.HandleFunc("/block", api.handleConsensusBlock).Methods("POST")

	// Add the consensusRouter to the main router
	api.router.PathPrefix("/consensus").Handler(consensusRouter)
}

// handleHome handles the home endpoint.
func (api *API) handleHome(w http.ResponseWriter, r *http.Request) {
	info := BlockchainInfo{
		Version:    BlockchainVersion,
		Name:       BlockchainName,
		Symbol:     BlockchainSymbol,
		BlockTime:  blockTimeInSec,
		Difficulty: proofOfWorkDifficulty,
		Fee:        transactionFee,
	}

	// Define the HTML template
	const homeTemplate = `
		<!DOCTYPE html>
		<html>
		<head>
			<title>Blockchain Info</title>
			<link rel="stylesheet" href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/css/bootstrap.min.css">
			<link rel="stylesheet" href="https://cdnjs.cloudflare.com/ajax/libs/font-awesome/5.15.4/css/all.min.css">
		</head>
		<body>
			<div class="container">
				<h1>Blockchain Info</h1>
				<table class="table">
					<tr>
						<th>Version</th>
						<td>{{.Version}}</td>
					</tr>
					<tr>
						<th>Name</th>
						<td>{{.Name}}</td>
					</tr>
					<tr>
						<th>Symbol</th>
						<td>{{.Symbol}}</td>
					</tr>
					<tr>
						<th>Block Time (s)</th>
						<td>{{.BlockTime}}</td>
					</tr>
					<tr>
						<th>Difficulty</th>
						<td>{{.Difficulty}}</td>
					</tr>
					<tr>
						<th>Transaction Fee</th>
						<td>{{.Fee}}</td>
					</tr>
				</table>
			</div>
			<script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.0/dist/js/bootstrap.bundle.min.js"></script>
		</body>
		</html>
	`

	// Parse the HTML template
	tmpl, err := template.New("home").Parse(homeTemplate)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Render the HTML template with the data
	err = tmpl.Execute(w, info)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

// handleVersion handles the version endpoint.
func (api *API) handleVersion(w http.ResponseWriter, r *http.Request) {
	info := BlockchainInfo{
		Version: BlockchainVersion,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Marshal the info struct to JSON
	data, err := json.Marshal(info)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Write the JSON response
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleInfo handles the info endpoint.
func (api *API) handleInfo(w http.ResponseWriter, r *http.Request) {
	info := BlockchainInfo{
		Version:    BlockchainVersion,
		Name:       BlockchainName,
		Symbol:     BlockchainSymbol,
		BlockTime:  blockTimeInSec,
		Difficulty: proofOfWorkDifficulty,
		Fee:        transactionFee,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Marshal the info struct to JSON
	data, err := json.Marshal(info)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Write the JSON response
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleHealth handles the health endpoint.
func (api *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	status := api.blockchainService.Status()

	response := struct {
		Status      string `json:"status"`
		BlockCount  int    `json:"block_count"`
		MempoolSize int    `json:"mempool_size"`
	}{
		Status:      "ok",
		BlockCount:  status.BlockCount,
		MempoolSize: status.MempoolSize,
	}

	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleConsensusP2P handles the consensus/P2P endpoint. This is used to recieve and process a Broadcast a Message to 1/3. Upon validation it is then
// broadcast to 2/3 of all nodes. Finally upon validation it is Broadcast to all nodes
// 1. get the post data and unmarshal it into a P2PTransaction
// 2. Add the transaction to the P2P queue
func (api *API) handleConsensusP2P(w http.ResponseWriter, r *http.Request) {
	// get the post data and unmarshal it into a P2PTransaction
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var tx P2PTransaction
	err = json.Unmarshal(data, &tx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if node == nil || node.P2P == nil {
		http.Error(w, "Consensus service unavailable", http.StatusServiceUnavailable)
		return
	}

	// 2. Add the transaction to the P2P queue
	node.P2P.AddTransaction(tx)

	// Return a 201 response to indicate the transaction was queued successfully
	w.WriteHeader(http.StatusCreated)
}

// handleConsensusTx handles the consensus/tx endpoint.
func (api *API) handleConsensusTx(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	response := struct {
		Status   string `json:"status"`
		Accepted bool   `json:"accepted"`
	}{
		Status:   "ok",
		Accepted: true,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleConsensusBlock handles the consensus/block endpoint.
func (api *API) handleConsensusBlock(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		http.Error(w, "Failed to read request body", http.StatusBadRequest)
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "Invalid JSON payload", http.StatusBadRequest)
		return
	}

	response := struct {
		Status   string `json:"status"`
		Accepted bool   `json:"accepted"`
	}{
		Status:   "ok",
		Accepted: true,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleBlockchain handles the blockchain endpoint.
func (api *API) handleBlockchain(w http.ResponseWriter, r *http.Request) {
	status := api.blockchainService.Status()

	// Create a response struct
	response := struct {
		NumBlocks              int `json:"num_blocks"`
		NumTransactionsInQueue int `json:"num_transactions_in_queue"`
	}{
		NumBlocks:              status.BlockCount,
		NumTransactionsInQueue: status.MempoolSize,
	}

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Marshal the response struct to JSON
	data, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Write the JSON response
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleBrowseBlocks handles the /blockchain/blocks endpoint.
func (api *API) handleBrowseBlocks(w http.ResponseWriter, r *http.Request) {

	// Parse the query parameters
	queryParams := r.URL.Query()
	page, err := strconv.Atoi(queryParams.Get("page"))
	if err != nil {
		page = 1
	}
	limit, err := strconv.Atoi(queryParams.Get("limit"))
	if err != nil {
		limit = 10
	}

	// Calculate the start and end indices for pagination
	startIndex := (page - 1) * limit
	endIndex := startIndex + limit
	if startIndex >= len(api.bc.Blocks) {
		startIndex = len(api.bc.Blocks) - 1
	}
	if endIndex >= len(api.bc.Blocks) {
		endIndex = len(api.bc.Blocks)
	}

	// Get the requested blocks based on the pagination
	requestedBlocks := api.bc.Blocks[startIndex:endIndex]

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Marshal the requested blocks to JSON
	data, err := json.Marshal(requestedBlocks)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Write the JSON response
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleViewBlock handles the /blockchain/blocks/{index} endpoint.
func (api *API) handleViewBlock(w http.ResponseWriter, r *http.Request) {
	// Get the block index from the request URL path parameters
	vars := mux.Vars(r)
	indexStr := vars["index"]
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		http.Error(w, "Invalid block index", http.StatusBadRequest)
		return
	}

	// Check if the requested block index is valid
	if index < 0 || index >= len(api.bc.Blocks) {
		http.Error(w, "Block not found", http.StatusNotFound)
		return
	}

	// Get the requested block
	block := api.bc.Blocks[index]

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Marshal the block to JSON
	data, err := json.Marshal(block)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Write the JSON response
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleBrowseTransactionsInBlock handles the /blockchain/blocks/{index}/transactions endpoint.
func (api *API) handleBrowseTransactionsInBlock(w http.ResponseWriter, r *http.Request) {
	// Get the block index from the path parameters
	vars := mux.Vars(r)
	indexStr := vars["index"]
	index, err := strconv.Atoi(indexStr)
	if err != nil {
		http.Error(w, "Invalid block index", http.StatusBadRequest)
		return
	}

	// Check if the requested block index is valid
	if index < 0 || index >= len(api.bc.Blocks) {
		http.Error(w, "Block index out of range", http.StatusBadRequest)
		return
	}

	// Get the transactions of the requested block
	transactions := api.bc.Blocks[index].Transactions

	// Set response headers
	w.Header().Set("Content-Type", "application/json")

	// Marshal the transactions to JSON
	data, err := json.Marshal(transactions)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Write the JSON response
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleViewTransactionInBlock handles the /blockchain/blocks/{index}/transactions/{id} endpoint.
func (api *API) handleViewTransactionInBlock(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	indexStr := vars["index"]
	idOrProtocol := vars["id"]

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		http.Error(w, "Invalid block index", http.StatusBadRequest)
		return
	}

	if index < 0 || index >= len(api.bc.Blocks) {
		http.Error(w, "Block not found", http.StatusNotFound)
		return
	}

	block := api.bc.Blocks[index]

	// Keep legacy route compatibility: `/.../transactions/{id}` and
	// `/.../transactions/{protocol}` share the same path pattern.
	if protocol, ok := normalizeProtocol(idOrProtocol); ok {
		api.respondTransactionsByProtocol(w, block, protocol)
		return
	}

	for _, tx := range block.Transactions {
		if tx.GetID() == idOrProtocol {
			w.Header().Set("Content-Type", "application/json")
			data, err := json.Marshal(tx)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				return
			}

			w.WriteHeader(http.StatusOK)
			if _, err := w.Write(data); err != nil {
				http.Error(w, "Internal server error", http.StatusInternalServerError)
				return
			}
			return
		}
	}

	http.Error(w, "Transaction not found", http.StatusNotFound)
}

// handleBrowseTransactionsByProtocolInBlock handles the /blockchain/blocks/{index}/transactions/{protocol} endpoint.
func (api *API) handleBrowseTransactionsByProtocolInBlock(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	indexStr := vars["index"]
	protocolStr := vars["protocol"]

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		http.Error(w, "Invalid block index", http.StatusBadRequest)
		return
	}

	if index < 0 || index >= len(api.bc.Blocks) {
		http.Error(w, "Block not found", http.StatusNotFound)
		return
	}

	protocol, ok := normalizeProtocol(protocolStr)
	if !ok {
		http.Error(w, "Invalid protocol", http.StatusBadRequest)
		return
	}

	api.respondTransactionsByProtocol(w, api.bc.Blocks[index], protocol)
}

func normalizeProtocol(candidate string) (string, bool) {
	for _, protocol := range AvailableProtocols {
		if strings.EqualFold(candidate, protocol) {
			return protocol, true
		}
	}
	return "", false
}

func (api *API) respondTransactionsByProtocol(w http.ResponseWriter, block *Block, protocol string) {
	filtered := make([]Transaction, 0)
	for _, tx := range block.Transactions {
		if strings.EqualFold(tx.GetProtocol(), protocol) {
			filtered = append(filtered, tx)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(filtered)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (api *API) ensureLocalStorageReady() error {
	if LocalStorageAvailable() {
		return nil
	}

	dataPath := ""
	if cfg := api.GetConfig(); cfg != nil {
		dataPath = cfg.DataPath
	}

	return NewLocalStorage(dataPath)
}

func (api *API) listWalletAddresses() ([]string, error) {
	if err := api.ensureLocalStorageReady(); err != nil {
		return nil, err
	}

	ls, err := GetLocalStorage()
	if err != nil {
		return nil, err
	}

	paths, err := filepath.Glob(filepath.Join(ls.dataPath, "wallets", "*.json"))
	if err != nil {
		return nil, err
	}

	addresses := make([]string, 0, len(paths))
	for _, p := range paths {
		name := filepath.Base(p)
		if strings.HasSuffix(name, ".json") {
			addresses = append(addresses, strings.TrimSuffix(name, ".json"))
		}
	}

	return addresses, nil
}

func (api *API) loadWalletByAddress(address string) (*Wallet, error) {
	if err := api.ensureLocalStorageReady(); err != nil {
		return nil, err
	}

	ls, err := GetLocalStorage()
	if err != nil {
		return nil, err
	}

	w := &Wallet{Address: address}
	if err := ls.Get("wallet", w); err != nil {
		return nil, err
	}

	return w, nil
}

// handleBrowseWallets handles the /blockchain/wallets endpoint.
func (api *API) handleBrowseWallets(w http.ResponseWriter, r *http.Request) {
	addresses, err := api.listWalletAddresses()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	type walletSummary struct {
		WalletID  string  `json:"wallet_id"`
		Address   string  `json:"address"`
		Encrypted bool    `json:"encrypted"`
		Balance   float64 `json:"balance"`
	}

	result := make([]walletSummary, 0, len(addresses))
	for _, addr := range addresses {
		wallet, err := api.loadWalletByAddress(addr)
		if err != nil {
			continue
		}

		result = append(result, walletSummary{
			WalletID:  wallet.ID.String(),
			Address:   wallet.GetAddress(),
			Encrypted: wallet.Encrypted,
			Balance:   api.bc.GetBalance(wallet.GetAddress()),
		})
	}

	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(result)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleCreateWallet handles the /blockchain/wallets/new endpoint.
func (api *API) handleCreateWallet(w http.ResponseWriter, r *http.Request) {
	passphrase, err := GenerateRandomPassword()
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	name := fmt.Sprintf("wallet-%d", time.Now().UnixNano())
	wallet, err := NewWallet(NewWalletOptions(
		NewBigInt(1),
		NewBigInt(1),
		NewBigInt(time.Now().Unix()),
		NewBigInt(0),
		name,
		passphrase,
		[]string{"api-created"},
	))
	if err != nil {
		http.Error(w, "Failed to create wallet", http.StatusInternalServerError)
		return
	}

	response := struct {
		WalletID   string `json:"wallet_id"`
		Address    string `json:"address"`
		Name       string `json:"name"`
		Passphrase string `json:"passphrase"`
	}{
		WalletID:   wallet.ID.String(),
		Address:    wallet.GetAddress(),
		Name:       name,
		Passphrase: passphrase,
	}

	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleCreateWalletTransaction handles /blockchain/wallets/tx for externally submitted transaction payloads.
func (api *API) handleCreateWalletTransaction(w http.ResponseWriter, r *http.Request) {
	type createTxRequest struct {
		Protocol string  `json:"protocol"`
		From     string  `json:"from"`
		To       string  `json:"to"`
		Amount   float64 `json:"amount"`
	}

	var req createTxRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if strings.TrimSpace(req.Protocol) == "" || strings.TrimSpace(req.From) == "" || strings.TrimSpace(req.To) == "" {
		http.Error(w, "Missing required transaction fields", http.StatusBadRequest)
		return
	}

	if req.Amount <= 0 {
		http.Error(w, "Amount must be greater than zero", http.StatusBadRequest)
		return
	}

	if _, ok := normalizeProtocol(req.Protocol); !ok {
		http.Error(w, "Invalid protocol", http.StatusBadRequest)
		return
	}

	response := struct {
		Status     string  `json:"status"`
		Accepted   bool    `json:"accepted"`
		Protocol   string  `json:"protocol"`
		From       string  `json:"from"`
		To         string  `json:"to"`
		Amount     float64 `json:"amount"`
		ExternalID string  `json:"external_id"`
	}{
		Status:     "ok",
		Accepted:   true,
		Protocol:   strings.ToUpper(req.Protocol),
		From:       req.From,
		To:         req.To,
		Amount:     req.Amount,
		ExternalID: generateRandomToken(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleViewWallet handles the /blockchain/wallets/{id} endpoint.
func (api *API) handleViewWallet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["id"]
	if address == "" {
		http.Error(w, "Invalid wallet ID", http.StatusBadRequest)
		return
	}

	wallet, err := api.loadWalletByAddress(address)
	if err != nil {
		http.Error(w, "Wallet not found", http.StatusNotFound)
		return
	}

	response := struct {
		WalletID  string  `json:"wallet_id"`
		Address   string  `json:"address"`
		Encrypted bool    `json:"encrypted"`
		Balance   float64 `json:"balance"`
	}{
		WalletID:  wallet.ID.String(),
		Address:   wallet.GetAddress(),
		Encrypted: wallet.Encrypted,
		Balance:   api.bc.GetBalance(wallet.GetAddress()),
	}

	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleUpdateWallet handles the /blockchain/wallets/{id} endpoint.
func (api *API) handleUpdateWallet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["id"]
	if address == "" {
		http.Error(w, "Invalid wallet ID", http.StatusBadRequest)
		return
	}

	type updateWalletRequest struct {
		Passphrase string   `json:"passphrase"`
		Name       string   `json:"name"`
		Tags       []string `json:"tags"`
	}

	var req updateWalletRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Passphrase == "" {
		http.Error(w, "Passphrase is required", http.StatusBadRequest)
		return
	}

	hasNameUpdate := req.Name != ""
	hasTagsUpdate := req.Tags != nil
	if !hasNameUpdate && !hasTagsUpdate {
		http.Error(w, "No update fields provided", http.StatusBadRequest)
		return
	}

	wallet, err := api.loadWalletByAddress(address)
	if err != nil {
		http.Error(w, "Wallet not found", http.StatusNotFound)
		return
	}

	if err := wallet.Unlock(req.Passphrase); err != nil {
		http.Error(w, "Invalid passphrase", http.StatusUnauthorized)
		return
	}

	if hasNameUpdate {
		if err := wallet.SetData("name", req.Name); err != nil {
			http.Error(w, "Failed to update wallet name", http.StatusInternalServerError)
			return
		}
	}

	if hasTagsUpdate {
		if err := wallet.SetData("tags", req.Tags); err != nil {
			http.Error(w, "Failed to update wallet tags", http.StatusInternalServerError)
			return
		}
	}

	if err := wallet.Close(req.Passphrase); err != nil {
		http.Error(w, "Failed to persist wallet updates", http.StatusInternalServerError)
		return
	}

	response := struct {
		WalletID string   `json:"wallet_id"`
		Address  string   `json:"address"`
		Name     string   `json:"name"`
		Tags     []string `json:"tags"`
		Updated  bool     `json:"updated"`
	}{
		WalletID: wallet.ID.String(),
		Address:  wallet.GetAddress(),
		Name:     req.Name,
		Tags:     req.Tags,
		Updated:  true,
	}

	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleViewWalletBalance handles the /blockchain/wallets/{id}/balance endpoint.
func (api *API) handleViewWalletBalance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["id"]
	if address == "" {
		http.Error(w, "Invalid wallet ID", http.StatusBadRequest)
		return
	}

	_, err := api.loadWalletByAddress(address)
	if err != nil {
		http.Error(w, "Wallet not found", http.StatusNotFound)
		return
	}

	response := struct {
		Address string  `json:"address"`
		Balance float64 `json:"balance"`
	}{
		Address: address,
		Balance: api.bc.GetBalance(address),
	}

	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(response)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleBrowseTransactionsForWallet handles the /blockchain/wallets/{id}/transactions endpoint.
func (api *API) handleBrowseTransactionsForWallet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	walletID := vars["id"]
	if walletID == "" {
		http.Error(w, "Invalid wallet ID", http.StatusBadRequest)
		return
	}

	history := api.bc.GetTransactionHistory(walletID)
	api.respondTransactionList(w, history)
}

// handleViewTransactionForWallet handles the /blockchain/wallets/{id}/transactions/{id} endpoint.
func (api *API) handleViewTransactionForWallet(w http.ResponseWriter, r *http.Request) {
	walletID, idOrProtocol, ok := parseWalletTransactionPath(r.URL.Path)
	if !ok {
		http.Error(w, "Invalid wallet transaction path", http.StatusBadRequest)
		return
	}

	history := api.bc.GetTransactionHistory(walletID)

	// Keep legacy route compatibility: `/.../transactions/{id}` and
	// `/.../transactions/{protocol}` share the same path pattern.
	if protocol, isProtocol := normalizeProtocol(idOrProtocol); isProtocol {
		api.respondTransactionsForWalletByProtocol(w, history, protocol)
		return
	}

	for _, tx := range history {
		if tx.GetID() == idOrProtocol {
			api.respondTransaction(w, tx)
			return
		}
	}

	http.Error(w, "Transaction not found", http.StatusNotFound)
}

// handleBrowseTransactionsByProtocolForWallet handles the /blockchain/wallets/{id}/transactions/{protocol} endpoint.
func (api *API) handleBrowseTransactionsByProtocolForWallet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	walletID := vars["id"]
	protocolStr := vars["protocol"]

	if walletID == "" {
		http.Error(w, "Invalid wallet ID", http.StatusBadRequest)
		return
	}

	protocol, ok := normalizeProtocol(protocolStr)
	if !ok {
		http.Error(w, "Invalid protocol", http.StatusBadRequest)
		return
	}

	history := api.bc.GetTransactionHistory(walletID)
	api.respondTransactionsForWalletByProtocol(w, history, protocol)
}

func parseWalletTransactionPath(path string) (string, string, bool) {
	const prefix = "/blockchain/wallets/"
	if !strings.HasPrefix(path, prefix) {
		return "", "", false
	}

	trimmed := strings.TrimPrefix(path, prefix)
	parts := strings.Split(trimmed, "/")
	if len(parts) != 3 || parts[1] != "transactions" || parts[0] == "" || parts[2] == "" {
		return "", "", false
	}

	return parts[0], parts[2], true
}

func (api *API) respondTransactionsForWalletByProtocol(w http.ResponseWriter, txs []Transaction, protocol string) {
	filtered := make([]Transaction, 0)
	for _, tx := range txs {
		if strings.EqualFold(tx.GetProtocol(), protocol) {
			filtered = append(filtered, tx)
		}
	}

	api.respondTransactionList(w, filtered)
}

func (api *API) respondTransaction(w http.ResponseWriter, tx Transaction) {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(tx)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (api *API) respondTransactionList(w http.ResponseWriter, txs []Transaction) {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(txs)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

func (api *API) collectAllTransactions() []Transaction {
	all := make([]Transaction, 0)
	for _, block := range api.bc.Blocks {
		all = append(all, block.Transactions...)
	}
	all = append(all, api.bc.GetPendingTransactions()...)
	return all
}

// handleBrowseTransactions handles the /blockchain/transactions endpoint.
func (api *API) handleBrowseTransactions(w http.ResponseWriter, r *http.Request) {
	api.respondTransactionList(w, api.collectAllTransactions())
}

// handleViewTransaction handles the /blockchain/transactions/{id} endpoint.
func (api *API) handleViewTransaction(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	idOrProtocol := vars["id"]
	if idOrProtocol == "" {
		http.Error(w, "Invalid transaction identifier", http.StatusBadRequest)
		return
	}

	txs := api.collectAllTransactions()

	// Keep legacy route compatibility: `/.../transactions/{id}` and
	// `/.../transactions/{protocol}` share the same path pattern.
	if protocol, ok := normalizeProtocol(idOrProtocol); ok {
		filtered := make([]Transaction, 0)
		for _, tx := range txs {
			if strings.EqualFold(tx.GetProtocol(), protocol) {
				filtered = append(filtered, tx)
			}
		}
		api.respondTransactionList(w, filtered)
		return
	}

	for _, tx := range txs {
		if tx.GetID() == idOrProtocol {
			api.respondTransaction(w, tx)
			return
		}
	}

	http.Error(w, "Transaction not found", http.StatusNotFound)
}

// handleBrowseTransactionsByProtocol handles the /blockchain/transactions/{protocol} endpoint.
func (api *API) handleBrowseTransactionsByProtocol(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	protocolStr := vars["protocol"]

	protocol, ok := normalizeProtocol(protocolStr)
	if !ok {
		http.Error(w, "Invalid protocol", http.StatusBadRequest)
		return
	}

	filtered := make([]Transaction, 0)
	for _, tx := range api.collectAllTransactions() {
		if strings.EqualFold(tx.GetProtocol(), protocol) {
			filtered = append(filtered, tx)
		}
	}

	api.respondTransactionList(w, filtered)
}
