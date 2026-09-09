// Package sdk is a software development kit for building blockchain applications.
// File sdk/api.go - API for the blockchain
package sdk

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	appservices "github.com/AndrewDonelson/go-basic-blockchain/internal/application/services"
	"github.com/gorilla/mux"
	logging "github.com/op/go-logging"
)

// ErrorResponse represents the structure of an error response.
type ErrorResponse struct {
	Message string `json:"message"`
}

// HTTP server hardening. http.ListenAndServe uses a zero-value http.Server, which
// has no timeouts at all: a handful of slow clients can hold every connection open
// indefinitely (Slowloris).
const (
	apiReadHeaderTimeout = 10 * time.Second
	apiReadTimeout       = 30 * time.Second
	apiWriteTimeout      = 30 * time.Second
	apiIdleTimeout       = 120 * time.Second
	apiMaxHeaderBytes    = 1 << 20 // 1 MiB
	apiMaxBodyBytes      = 1 << 20 // 1 MiB
	// defaultPageLimit / maxPageLimit bound paginated collection responses.
	defaultPageLimit = 10
	maxPageLimit     = 1000
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
	runningMu         sync.RWMutex
	server            *http.Server
	blockchainService *appservices.BlockchainService
	accountStore      AccountStore
	accountLimiter    *rateLimiter
}

// respondJSON writes v as a JSON response with the given status code.
//
// Every handler previously repeated the same 12-line marshal/WriteHeader/Write
// block, and several wrote the status code before discovering the payload could
// not be marshalled -- at which point http.Error could no longer set a 500.
func respondJSON(w http.ResponseWriter, status int, v interface{}) {
	data, err := json.Marshal(v)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write(data); err != nil {
		// The status line is already sent; all that is left is to record it.
		LogVerbosef("failed writing response body: %v", err)
	}
}

var publicPaths = []string{
	"/",
	"/version",
	"/info",
	"/health",
	"/metrics",
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
		accountLimiter:    newRateLimiter(authRateLimitAttempts, authRateLimitWindow),
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

// authenticateNode guards the /consensus endpoints.
//
// With no key configured this now refuses every request rather than relying on a
// published fallback key. Comparisons are constant-time and attempts are rate
// limited per source address.
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

	limiter := newRateLimiter(authRateLimitAttempts, authRateLimitWindow)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(decodedAPIKeys) == 0 {
			RespondError(w, http.StatusServiceUnavailable,
				"consensus authentication unavailable: no API key configured")
			return
		}

		host := clientIP(r)
		if !limiter.allow(host) {
			RespondError(w, http.StatusTooManyRequests, "too many authentication attempts")
			return
		}

		apiKey, err := bearerToken(r, cfg.APIKeyHeader)
		if err != nil {
			RespondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		principal, ok := apiKeyIsValid(apiKey, decodedAPIKeys)
		if !ok {
			RespondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		limiter.reset(host)
		next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principal)))
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

// IsRunning returns true if the API is running.
func (api *API) IsRunning() bool {
	api.runningMu.RLock()
	defer api.runningMu.RUnlock()
	return api.running
}

func (api *API) setRunning(running bool) {
	api.runningMu.Lock()
	defer api.runningMu.Unlock()
	api.running = running
}

// Start starts the API and listens for incoming requests.
//
// It returns an error instead of calling log.Fatal: a library must not terminate
// the host process, and the caller needs to know the listener failed. The server
// is configured with explicit timeouts -- http.ListenAndServe's zero-value server
// has none, which leaves it open to Slowloris-style connection exhaustion.
func (api *API) Start() error {
	if api.IsRunning() {
		return nil
	}

	keyCfg := defaultAPIKeyConfig()
	keyCfg.accountStore = api.accountStore

	// Create a logging middleware
	api.router.Use(loggingMiddleware)

	// API key middleware
	apiKeyMiddleware, err := ApiKeyMiddleware(keyCfg, api.log)
	if err != nil {
		return fmt.Errorf("error initializing API key middleware: %w", err)
	}
	api.router.Use(apiKeyMiddleware)

	bindAddr := apiHostname
	if cfg := api.GetConfig(); cfg != nil && cfg.APIHostName != "" {
		bindAddr = cfg.APIHostName
	}

	server := &http.Server{
		Addr:              bindAddr,
		Handler:           api.router,
		ReadHeaderTimeout: apiReadHeaderTimeout,
		ReadTimeout:       apiReadTimeout,
		WriteTimeout:      apiWriteTimeout,
		IdleTimeout:       apiIdleTimeout,
		MaxHeaderBytes:    apiMaxHeaderBytes,
	}

	api.runningMu.Lock()
	api.server = server
	api.running = true
	api.runningMu.Unlock()

	LogInfof("API server starting on %s", bindAddr)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		api.setRunning(false)
		return fmt.Errorf("api server stopped: %w", err)
	}

	api.setRunning(false)
	return nil
}

// Stop gracefully shuts the API server down.
func (api *API) Stop(ctx context.Context) error {
	api.runningMu.RLock()
	server := api.server
	api.runningMu.RUnlock()

	if server == nil {
		return nil
	}
	return server.Shutdown(ctx)
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
	api.router.HandleFunc("/metrics", api.handleMetrics).Methods("GET")

	// Register Public Account Endpoints.
	// register/login are POST: they create state and carry a credential, neither
	// of which belongs in a GET query string.
	api.router.HandleFunc("/account/register", api.handleAccountRegister).Methods("POST")
	api.router.HandleFunc("/account/login", api.handleAccountLogin).Methods("POST")
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
	api.router.HandleFunc("/blockchain/wallets", api.handleBrowseWallets).Methods("GET")
	// POST: creating a wallet is not a safe, idempotent, cacheable operation.
	api.router.HandleFunc("/blockchain/wallets/new", api.handleCreateWallet).Methods("POST")
	api.router.HandleFunc("/blockchain/wallets/tx", api.handleCreateWalletTransaction).Methods("POST")
	api.router.HandleFunc("/blockchain/wallets/{id}", api.handleViewWallet).Methods("GET")
	api.router.HandleFunc("/blockchain/wallets/{id}", api.handleUpdateWallet).Methods("POST")
	api.router.HandleFunc("/blockchain/wallets/{id}/balance", api.handleViewWalletBalance).Methods("GET")
	api.router.HandleFunc("/blockchain/wallets/{id}/transactions", api.handleBrowseTransactionsForWallet).Methods("GET")
	// Was "/blockchain/wallets/{id}/transactions/{id}" -- two variables with the
	// same name, of which mux keeps only one. The handler worked around it by
	// re-parsing r.URL.Path by hand; with distinct names mux.Vars is usable again.
	api.router.HandleFunc("/blockchain/wallets/{id}/transactions/{txid}", api.handleViewTransactionForWallet).Methods("GET")
	api.router.HandleFunc("/blockchain/transactions", api.handleBrowseTransactions).Methods("GET")
	// A single route handles both an ID and a protocol name; registering the same
	// pattern twice left the second registration permanently unreachable.
	api.router.HandleFunc("/blockchain/transactions/{id}", api.handleViewTransaction).Methods("GET")

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

// handleMetrics exposes node metrics in the Prometheus text exposition format.
//
// Public alongside /health: a metrics endpoint behind authentication is one no
// scraper will be configured for, and nothing here is a secret -- counts,
// timings and a hash rate, no addresses, balances or keys.
//
// Live gauges are passed in rather than mirrored into the recorder, so there is
// one source of truth for each: the mempool knows its own depth, the P2P layer
// knows its peer count, and the syncer already keeps its own stats.
func (api *API) handleMetrics(w http.ResponseWriter, r *http.Request) {
	extra := map[string]float64{}

	if api.bc != nil {
		extra["chain_height"] = float64(api.bc.Height())
		extra["mempool_size"] = float64(api.bc.GetMempoolSize())
		extra["difficulty"] = float64(api.bc.CurrentDifficulty())
		extra["utxo_count"] = float64(api.bc.UTXOSet().Size())
		extra["total_supply"] = api.bc.CalculateTotalSupply()
	}

	if node := GetNode(); node != nil {
		if node.P2P != nil {
			extra["peers"] = float64(node.P2P.PeerCount())
		}
		if node.Syncer != nil {
			stats := node.Syncer.Stats()
			extra["sync_passes"] = float64(stats.Passes)
			extra["sync_blocks_pulled"] = float64(stats.BlocksApplied)
			extra["sync_failures"] = float64(stats.Failures)
		}
	}

	var recorder *Metrics
	if api.bc != nil {
		recorder = api.bc.Metrics()
	}

	body := recorder.Prometheus("gbb", extra)

	w.Header().Set("Content-Type", "text/plain; version=0.0.4; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write([]byte(body)); err != nil {
		LogVerbosef("Failed to write metrics response: %v", err)
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

// handleConsensusTx accepts a transaction from another node.
//
// This used to parse the body into a discarded map and answer {"accepted":true}
// unconditionally -- it validated nothing and stored nothing, while telling the
// caller its transaction had been accepted. It now verifies the signature and
// enqueues the transaction, or reports precisely why it did not.
func (api *API) handleConsensusTx(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, apiMaxBodyBytes))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	tx, err := DecodeTransaction(body)
	if err != nil {
		RespondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid transaction payload: %v", err))
		return
	}

	if err := tx.Validate(); err != nil {
		respondJSON(w, http.StatusUnprocessableEntity, consensusResult{
			Status: "rejected", Accepted: false, Reason: err.Error(),
		})
		return
	}

	sender := tx.GetSenderWallet()
	if sender == nil {
		respondJSON(w, http.StatusUnprocessableEntity, consensusResult{
			Status: "rejected", Accepted: false, Reason: "transaction has no sender wallet",
		})
		return
	}

	valid, err := tx.Verify([]byte(sender.PublicPEM()), tx.GetSignature())
	if err != nil || !valid {
		reason := "signature verification failed"
		if err != nil {
			reason = err.Error()
		}
		respondJSON(w, http.StatusUnprocessableEntity, consensusResult{
			Status: "rejected", Accepted: false, Reason: reason,
		})
		return
	}

	if api.bc.HasTransactionID(tx.GetID()) {
		respondJSON(w, http.StatusOK, consensusResult{
			Status: "duplicate", Accepted: false, Reason: "transaction already known", ID: tx.GetID(),
		})
		return
	}

	api.bc.AddTransaction(tx)

	respondJSON(w, http.StatusAccepted, consensusResult{
		Status: "ok", Accepted: true, ID: tx.GetID(),
	})
}

// consensusResult is the reply shape for the consensus endpoints.
type consensusResult struct {
	Status   string `json:"status"`
	Accepted bool   `json:"accepted"`
	Reason   string `json:"reason,omitempty"`
	ID       string `json:"id,omitempty"`
}

// handleConsensusBlock accepts a block from another node.
//
// Like consensus/tx this previously answered {"accepted":true} without looking at
// the payload. It now validates the block against the current head before
// accepting it. Full fork-choice/reorg handling is still absent (see the design
// notes), so a block that does not extend the current head is explicitly refused
// rather than silently "accepted".
func (api *API) handleConsensusBlock(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(io.LimitReader(r.Body, apiMaxBodyBytes))
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Failed to read request body")
		return
	}

	block := &Block{}
	if err := json.Unmarshal(body, block); err != nil {
		RespondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid block payload: %v", err))
		return
	}

	if err := api.bc.AcceptBlock(block); err != nil {
		respondJSON(w, http.StatusUnprocessableEntity, consensusResult{
			Status: "rejected", Accepted: false, Reason: err.Error(), ID: block.Hash,
		})
		return
	}

	respondJSON(w, http.StatusAccepted, consensusResult{
		Status: "ok", Accepted: true, ID: block.Hash,
	})
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

// parsePagination reads and clamps page/limit query parameters.
func parsePagination(r *http.Request) (page, limit int) {
	page, err := strconv.Atoi(r.URL.Query().Get("page"))
	if err != nil || page < 1 {
		page = 1
	}
	limit, err = strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || limit < 1 {
		limit = defaultPageLimit
	}
	if limit > maxPageLimit {
		limit = maxPageLimit
	}
	return page, limit
}

// handleBrowseBlocks handles the /blockchain/blocks endpoint.
//
// Two defects here: the read went straight at api.bc.Blocks while the miner was
// appending to it (a data race), and on an empty chain the clamp set startIndex to
// -1 and the reslice panicked. Pagination now clamps into [0, len] and reads a
// copy taken under the blockchain's lock.
func (api *API) handleBrowseBlocks(w http.ResponseWriter, r *http.Request) {
	page, limit := parsePagination(r)

	total := api.bc.GetBlockCount()
	startIndex := (page - 1) * limit
	if startIndex > total {
		startIndex = total
	}
	endIndex := startIndex + limit
	if endIndex > total {
		endIndex = total
	}

	respondJSON(w, http.StatusOK, struct {
		Page   int      `json:"page"`
		Limit  int      `json:"limit"`
		Total  int      `json:"total"`
		Blocks []*Block `json:"blocks"`
	}{
		Page:   page,
		Limit:  limit,
		Total:  total,
		Blocks: api.bc.GetBlockRange(startIndex, endIndex),
	})
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

	block := api.bc.GetBlockByIndex(int64(index))
	if block == nil {
		http.Error(w, "Block not found", http.StatusNotFound)
		return
	}

	respondJSON(w, http.StatusOK, block)
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

	block := api.bc.GetBlockByIndex(int64(index))
	if block == nil {
		http.Error(w, "Block index out of range", http.StatusBadRequest)
		return
	}

	respondJSON(w, http.StatusOK, block.Transactions)
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

	block := api.bc.GetBlockByIndex(int64(index))
	if block == nil {
		http.Error(w, "Block not found", http.StatusNotFound)
		return
	}

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

// walletID returns a wallet's identifier, tolerating a wallet loaded from a file
// that predates the ID field.
func walletID(w *Wallet) string {
	if w == nil || w.ID == nil {
		return ""
	}
	return w.ID.String()
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
			WalletID:  walletID(wallet),
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
//
// POST /blockchain/wallets/new  {"name":"...","passphrase":"...","tags":[...]}
//
// The caller supplies the passphrase. This used to be a GET that generated a
// passphrase server-side and returned it in the response body: a GET that creates
// state, and a secret travelling back over a channel the server does not control,
// landing in caches and browser history.
func (api *API) handleCreateWallet(w http.ResponseWriter, r *http.Request) {
	type createWalletRequest struct {
		Name       string   `json:"name"`
		Passphrase string   `json:"passphrase"`
		Tags       []string `json:"tags"`
	}

	var req createWalletRequest
	if r.Body != nil {
		// An empty body is tolerated so the endpoint stays usable for exploration;
		// the passphrase check below still applies.
		_ = json.NewDecoder(io.LimitReader(r.Body, apiMaxBodyBytes)).Decode(&req)
	}

	if err := testPasswordStrength(req.Passphrase); err != nil {
		RespondError(w, http.StatusBadRequest, fmt.Sprintf("Passphrase is not strong enough: %v", err))
		return
	}

	name := req.Name
	if name == "" {
		name = fmt.Sprintf("wallet-%d", time.Now().UnixNano())
	}
	tags := req.Tags
	if tags == nil {
		tags = []string{"api-created"}
	}

	wallet, err := NewWallet(NewWalletOptions(
		NewBigInt(1),
		NewBigInt(1),
		NewBigInt(time.Now().Unix()),
		NewBigInt(time.Now().UnixNano()),
		name,
		req.Passphrase,
		tags,
	))
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to create wallet")
		return
	}

	respondJSON(w, http.StatusCreated, struct {
		WalletID string   `json:"wallet_id"`
		Address  string   `json:"address"`
		Name     string   `json:"name"`
		Tags     []string `json:"tags"`
	}{
		WalletID: wallet.ID.String(),
		Address:  wallet.GetAddress(),
		Name:     name,
		Tags:     tags,
	})
}

// handleCreateWalletTransaction submits a transaction on behalf of a stored wallet.
//
// POST /blockchain/wallets/tx
//
//	{"protocol":"BANK","from":"<addr>","to":"<addr>","amount":1.0,
//	 "passphrase":"<sender passphrase>"}
//
// This endpoint previously validated the request and then returned
// {"status":"ok","accepted":true,...} without creating, signing, queueing or
// persisting anything at all. It now builds, signs and enqueues a real
// transaction, or reports why it could not.
func (api *API) handleCreateWalletTransaction(w http.ResponseWriter, r *http.Request) {
	type createTxRequest struct {
		Protocol   string  `json:"protocol"`
		From       string  `json:"from"`
		To         string  `json:"to"`
		Amount     float64 `json:"amount"`
		Message    string  `json:"message"`
		Passphrase string  `json:"passphrase"`
	}

	var req createTxRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, apiMaxBodyBytes)).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if strings.TrimSpace(req.From) == "" || strings.TrimSpace(req.To) == "" {
		RespondError(w, http.StatusBadRequest, "Missing required transaction fields")
		return
	}

	protocol, ok := normalizeProtocol(req.Protocol)
	if !ok {
		RespondError(w, http.StatusBadRequest, "Invalid protocol")
		return
	}

	if req.Passphrase == "" {
		RespondError(w, http.StatusBadRequest, "Sender passphrase is required to sign the transaction")
		return
	}

	sender, err := OpenWallet(req.From, req.Passphrase)
	if err != nil {
		RespondError(w, http.StatusUnauthorized, "Could not unlock the sender wallet")
		return
	}

	recipient, err := api.loadWalletByAddress(req.To)
	if err != nil {
		RespondError(w, http.StatusNotFound, "Recipient wallet not found")
		return
	}

	tx, err := api.buildWalletTransaction(protocol, sender, recipient, req.Amount, req.Message)
	if err != nil {
		RespondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	signature, err := tx.Sign([]byte(sender.PrivatePEM()))
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to sign transaction")
		return
	}
	setTransactionSignature(tx, signature)

	if err := tx.Send(api.bc); err != nil {
		RespondError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}

	respondJSON(w, http.StatusAccepted, struct {
		Status   string  `json:"status"`
		Accepted bool    `json:"accepted"`
		ID       string  `json:"id"`
		Protocol string  `json:"protocol"`
		From     string  `json:"from"`
		To       string  `json:"to"`
		Amount   float64 `json:"amount,omitempty"`
	}{
		Status:   "ok",
		Accepted: true,
		ID:       tx.GetID(),
		Protocol: protocol,
		From:     sender.GetAddress(),
		To:       recipient.GetAddress(),
		Amount:   req.Amount,
	})
}

// buildWalletTransaction constructs a concrete transaction for the given protocol.
func (api *API) buildWalletTransaction(protocol string, from, to *Wallet, amount float64, message string) (Transaction, error) {
	switch protocol {
	case BankProtocolID:
		if amount <= 0 {
			return nil, errors.New("amount must be greater than zero")
		}
		return NewBankTransaction(from, to, amount)

	case MessageProtocolID:
		if strings.TrimSpace(message) == "" {
			return nil, errors.New("message must not be empty")
		}
		return NewMessageTransaction(from, to, message)

	default:
		return nil, fmt.Errorf("protocol %s cannot be submitted through this endpoint", protocol)
	}
}

// setTransactionSignature stores a signature on a concrete transaction.
func setTransactionSignature(tx Transaction, signature string) {
	switch concrete := tx.(type) {
	case *Bank:
		concrete.Signature = signature
	case *Message:
		concrete.Signature = signature
	case *Coinbase:
		concrete.Signature = signature
	case *Persist:
		concrete.Signature = signature
	case *Tx:
		concrete.Signature = signature
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
		WalletID:  walletID(wallet),
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
	if err := json.NewDecoder(io.LimitReader(r.Body, apiMaxBodyBytes)).Decode(&req); err != nil {
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
		WalletID: walletID(wallet),
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
	// The route now uses distinct variable names ({id} and {txid}), so mux.Vars
	// works and the hand-rolled path parser this used to need is gone.
	vars := mux.Vars(r)
	walletID := vars["id"]
	idOrProtocol := vars["txid"]

	if walletID == "" || idOrProtocol == "" {
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

// collectAllTransactions returns every transaction, read under the chain's lock.
func (api *API) collectAllTransactions() []Transaction {
	return api.bc.GetAllTransactions()
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
