package sdk

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

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
	router *mux.Router
}

// NewAPI creates a new instance of the blockchain API.
func NewAPI() *API {
	// Initialize the Gorilla Mux router
	api := &API{
		router: mux.NewRouter(),
	}

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

// Start starts the API and listens for incoming requests
func (api *API) Start(addr string) {
	// Start the HTTP server
	fmt.Printf("API listening on %s\n", addr)
	log.Fatal(http.ListenAndServe(addr, api.router))
}

// registerRoutes registers the API routes.
func (api *API) registerRoutes() {
	api.router.HandleFunc("/", api.handleHome).Methods("GET")
	api.router.HandleFunc("/version", api.handleVersion).Methods("GET")
	api.router.HandleFunc("/info", api.handleInfo).Methods("GET")
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

// handleVersion handles the version endpoint.
func (api *API) handleVersion(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleInfo handles the info endpoint.
func (api *API) handleInfo(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
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
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleBrowseBlocks handles the /blockchain/blocks endpoint.
func (api *API) handleBrowseBlocks(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleViewBlock handles the /blockchain/blocks/{index} endpoint.
func (api *API) handleViewBlock(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleBrowseTransactionsInBlock handles the /blockchain/blocks/{index}/transactions endpoint.
func (api *API) handleBrowseTransactionsInBlock(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleViewTransactionInBlock handles the /blockchain/blocks/{index}/transactions/{id} endpoint.
func (api *API) handleViewTransactionInBlock(w http.ResponseWriter, r *http.Request) {
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
