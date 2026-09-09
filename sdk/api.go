// Package sdk is a software development kit for building blockchain applications.
// File sdk/api.go - API for the blockchain
package sdk

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	appservices "github.com/AndrewDonelson/go-basic-blockchain/internal/application/services"
	"github.com/gorilla/mux"
	logging "github.com/op/go-logging"
)

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

	// Install the middleware with the routes, not with the listener.
	//
	// Authentication used to be attached in Start(), so api.router on its own was
	// an unauthenticated API. Anything that serves the router directly -- an
	// embedder, or a test harness wiring it into its own http.Server -- silently
	// got no authentication at all. The router is now safe to serve as it is.
	if err := api.installMiddleware(); err != nil {
		LogInfof("Failed to install API middleware: %v", err)
		return nil
	}

	return api
}

// NewAPIWithError is NewAPI, reporting why construction failed.
//
// NewAPI returns a bare nil, so the caller could say no more than "failed to
// create API" while the actual reason -- a non-hexadecimal API key, say -- went
// to a log line somewhere above it. Anything that wants to tell a user what to
// fix should call this.
func NewAPIWithError(bc *Blockchain) (*API, error) {
	if bc == nil {
		return nil, errors.New("cannot create an API without a blockchain")
	}

	api := &API{
		bc:                bc,
		log:               logging.MustGetLogger("api"),
		router:            mux.NewRouter(),
		blockchainService: appservices.NewBlockchainService(bc),
		accountStore:      NewFileAccountStore(bc.GetConfig().DataPath),
		accountLimiter:    newRateLimiter(authRateLimitAttempts, authRateLimitWindow),
	}

	LogInfof("Initializing API...")
	api.registerRoutes()

	if err := api.installMiddleware(); err != nil {
		return nil, err
	}
	return api, nil
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

	// Middleware is installed by NewAPI, alongside the routes it protects.

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

	cfg := api.GetConfig()
	useTLS := cfg != nil && cfg.APITLSEnabled
	if useTLS {
		server.TLSConfig = apiTLSConfig()
	}

	api.runningMu.Lock()
	api.server = server
	api.running = true
	api.runningMu.Unlock()

	var err error
	if useTLS {
		LogInfof("API server starting on %s (TLS)", bindAddr)
		// The certificate and key were already loaded once by Config.Validate, so
		// a failure here is a file that changed underneath us rather than a
		// configuration mistake.
		err = server.ListenAndServeTLS(cfg.APITLSCertFile, cfg.APITLSKeyFile)
	} else {
		LogInfof("API server starting on %s (plaintext HTTP -- set API_TLS_ENABLED "+
			"to serve HTTPS, or terminate TLS at a proxy)", bindAddr)
		err = server.ListenAndServe()
	}

	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		api.setRunning(false)
		return fmt.Errorf("api server stopped: %w", err)
	}

	api.setRunning(false)
	return nil
}

// apiTLSConfig returns the TLS settings the API server is served with.
//
// TLS 1.2 is the floor: 1.0 and 1.1 have been deprecated for years and Go still
// permits them unless a minimum is set. The cipher suites are the AEAD suites
// with forward secrecy -- the same property the P2P handshake provides through
// ephemeral ECDH. CBC and RSA key-exchange suites are left out: the first has a
// long history of padding-oracle attacks, and the second offers no forward
// secrecy, so recording traffic today and stealing the key later decrypts it.
//
// Go chooses TLS 1.3 cipher suites itself and ignores the list below for them,
// which is why only the 1.2 suites are named.
func apiTLSConfig() *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		CipherSuites: []uint16{
			tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
			tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
			tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
		},
	}
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
