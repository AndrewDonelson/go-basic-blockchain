// Package sdk is a software development kit for building blockchain applications.
// File sdk/api.go - API for the blockchain
package sdk

import (
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

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
	bc      *Blockchain
	router  *mux.Router
	running bool
}

// NewAPI creates a new instance of the blockchain API.
func NewAPI(bc *Blockchain) *API {
	// Initialize the Gorilla Mux router
	api := &API{
		bc:     bc,
		router: mux.NewRouter(),
	}

	fmt.Printf("Initializing API...\n")

	// Register the API endpoints
	api.registerRoutes()
	return api
}

func authenticateNode(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Perform authentication logic here
		// Check if the node is registered and has valid credentials
		// If authentication fails, return an error or redirect to an error page

		// Example: Assume authentication fails for demonstration
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
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
		fmt.Printf("%s -> Response: %d %d bytes\n", cacheReqStr, ww.statusCode, ww.bytesWritten)
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

	// Start the HTTP server
	fmt.Printf("API listening on %s\n", apiHostname)
	api.running = true
	log.Fatal(http.ListenAndServe(apiHostname, api.router))
}

// registerRoutes registers the API routes.
func (api *API) registerRoutes() {
	api.router.HandleFunc("/", api.handleHome).Methods("GET") // same as /info but HTML only
	api.router.HandleFunc("/version", api.handleVersion).Methods("GET")
	api.router.HandleFunc("/info", api.handleInfo).Methods("GET") // Same as / but JSON only
	api.router.HandleFunc("/health", api.handleHealth).Methods("GET")
	api.router.HandleFunc("/blockchain", api.handleBlockchain).Methods("GET")
	api.router.HandleFunc("/blockchain/blocks", api.handleBrowseBlocks).Methods("GET")
	api.router.HandleFunc("/blockchain/blocks/{index}", api.handleViewBlock).Methods("GET")
	api.router.HandleFunc("/blockchain/blocks/{index}/transactions", api.handleBrowseTransactionsInBlock).Methods("GET")
	api.router.HandleFunc("/blockchain/blocks/{index}/transactions/{id}", api.handleViewTransactionInBlock).Methods("GET")
	api.router.HandleFunc("/blockchain/blocks/{index}/transactions/{protocol}", api.handleBrowseTransactionsByProtocolInBlock).Methods("GET")
	api.router.HandleFunc("/blockchain/wallets", api.handleBrowseWallets).Methods("GET")
	api.router.HandleFunc("/blockchain/wallets/new", api.handleCreateWallet).Methods("GET")
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
	w.Write(data)
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
	w.Write(data)
}

// handleHealth handles the health endpoint.
func (api *API) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleConsensusTx handles the consensus/tx endpoint.
func (api *API) handleConsensusTx(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleConsensusBlock handles the consensus/block endpoint.
func (api *API) handleConsensusBlock(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleBlockchain handles the blockchain endpoint.
func (api *API) handleBlockchain(w http.ResponseWriter, r *http.Request) {

	// Create a response struct
	response := struct {
		NumBlocks              int `json:"num_blocks"`
		NumTransactionsInQueue int `json:"num_transactions_in_queue"`
	}{
		NumBlocks:              len(api.bc.Blocks),
		NumTransactionsInQueue: len(api.bc.TransactionQueue),
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
	w.Write(data)
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
	w.Write(data)
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
	w.Write(data)
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
	w.Write(data)
}

// handleViewTransactionInBlock handles the /blockchain/blocks/{index}/transactions/{id} endpoint.
func (api *API) handleViewTransactionInBlock(w http.ResponseWriter, r *http.Request) {
	// 	// Get the index and transaction ID from the URL path parameters
	// 	vars := mux.Vars(r)
	// 	indexStr := vars["index"]
	// 	idStr := vars["id"]

	// 	// Parse the index and transaction ID
	// 	index, err := strconv.Atoi(indexStr)
	// 	if err != nil {
	// 		http.Error(w, "Invalid block index", http.StatusBadRequest)
	// 		return
	// 	}

	// 	// Check if the block index is valid
	// 	if index < 0 || index >= len(api.bc.Blocks) {
	// 		http.Error(w, "Block not found", http.StatusNotFound)
	// 		return
	// 	}

	// 	// Get the block at the specified index
	// 	block := api.bc.Blocks[index]

	// 	// Find the transaction with the specified ID in the block
	// 	var transaction *Transaction
	// 	for _, tx := range block.Transactions {
	// 		if tx.ID == idStr {
	// 			transaction = tx
	// 			break
	// 		}
	// 	}

	// 	// Check if the transaction was found
	// 	if transaction == nil {
	// 		http.Error(w, "Transaction not found", http.StatusNotFound)
	// 		return
	// 	}

	// 	// Set response headers
	// 	w.Header().Set("Content-Type", "application/json")

	// 	// Marshal the transaction to JSON
	// 	data, err := json.Marshal(transaction)
	// 	if err != nil {
	// 		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	// 		return
	// 	}

	// 	// Write the JSON response
	// 	w.WriteHeader(http.StatusOK)
	// 	w.Write(data)

	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))

}

// handleBrowseTransactionsByProtocolInBlock handles the /blockchain/blocks/{index}/transactions/{protocol} endpoint.
func (api *API) handleBrowseTransactionsByProtocolInBlock(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleBrowseWallets handles the /blockchain/wallets endpoint.
func (api *API) handleBrowseWallets(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleCreateWallet handles the /blockchain/wallets/new endpoint.
func (api *API) handleCreateWallet(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleViewWallet handles the /blockchain/wallets/{id} endpoint.
func (api *API) handleViewWallet(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleUpdateWallet handles the /blockchain/wallets/{id} endpoint.
func (api *API) handleUpdateWallet(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleViewWalletBalance handles the /blockchain/wallets/{id}/balance endpoint.
func (api *API) handleViewWalletBalance(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleBrowseTransactionsForWallet handles the /blockchain/wallets/{id}/transactions endpoint.
func (api *API) handleBrowseTransactionsForWallet(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleViewTransactionForWallet handles the /blockchain/wallets/{id}/transactions/{id} endpoint.
func (api *API) handleViewTransactionForWallet(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleBrowseTransactionsByProtocolForWallet handles the /blockchain/wallets/{id}/transactions/{protocol} endpoint.
func (api *API) handleBrowseTransactionsByProtocolForWallet(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleBrowseTransactions handles the /blockchain/transactions endpoint.
func (api *API) handleBrowseTransactions(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleViewTransaction handles the /blockchain/transactions/{id} endpoint.
func (api *API) handleViewTransaction(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleBrowseTransactionsByProtocol handles the /blockchain/transactions/{protocol} endpoint.
func (api *API) handleBrowseTransactionsByProtocol(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}
