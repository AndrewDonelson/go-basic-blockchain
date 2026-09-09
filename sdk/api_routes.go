// Package sdk is a software development kit for building blockchain applications.
// File sdk/api_routes.go - The route table, mounted under /v1 and the legacy paths.
package sdk

import (
	"github.com/gorilla/mux"
)

// apiVersionPrefix is the versioned mount point for every endpoint.
//
// Versioning is additive here: each route is registered twice, once under /v1
// and once at its historic unprefixed path. Moving the endpoints outright would
// break every existing client the day it shipped, to buy nothing until there is
// a second version to distinguish from. New clients should use /v1; the
// unprefixed paths are the compatibility surface and will not gain new
// endpoints.
const apiVersionPrefix = "/v1"

// registerRoutes registers the API routes under both /v1 and the historic
// unprefixed paths.
func (api *API) registerRoutes() {
	// The versioned mount comes first so that, where a path could match either,
	// the explicit version wins.
	api.registerInto(api.router.PathPrefix(apiVersionPrefix).Subrouter())
	api.registerInto(api.router)
}

// registerInto attaches every route to a router, so the same table can be
// mounted at more than one prefix.
func (api *API) registerInto(r *mux.Router) {
	r.HandleFunc("/", api.handleHome).Methods("GET") // same as /info but HTML only
	r.HandleFunc("/version", api.handleVersion).Methods("GET")
	r.HandleFunc("/info", api.handleInfo).Methods("GET") // Same as / but JSON only
	r.HandleFunc("/health", api.handleHealth).Methods("GET")
	r.HandleFunc("/metrics", api.handleMetrics).Methods("GET")

	// Register Public Account Endpoints.
	// register/login are POST: they create state and carry a credential, neither
	// of which belongs in a GET query string.
	r.HandleFunc("/account/register", api.handleAccountRegister).Methods("POST")
	r.HandleFunc("/account/login", api.handleAccountLogin).Methods("POST")
	r.HandleFunc("/account/verify", api.handleAccountVerify).Methods("GET")

	// Register Private Account Endpoints
	// r.HandleFunc("/account", api.handleAccount).Methods("GET")
	// r.HandleFunc("/account/logout", api.handleAccountLogout).Methods("GET")
	// r.HandleFunc("/account/{id}", api.handleAccount).Methods("GET")
	// r.HandleFunc("/account/{id}/balance", api.handleAccountBalance).Methods("GET")
	// r.HandleFunc("/account/{id}/transactions", api.handleAccountTransactions).Methods("GET")
	// r.HandleFunc("/account/{id}/transactions/{id}", api.handleAccountTransaction).Methods("GET")
	// r.HandleFunc("/account/{id}/transactions/{protocol}", api.handleAccountTransactionsByProtocol).Methods("GET")

	// Register the blockchain endpoints
	r.HandleFunc("/blockchain", api.handleBlockchain).Methods("GET")
	r.HandleFunc("/blockchain/blocks", api.handleBrowseBlocks).Methods("GET")
	r.HandleFunc("/blockchain/blocks/{index}", api.handleViewBlock).Methods("GET")
	r.HandleFunc("/blockchain/blocks/{index}/transactions", api.handleBrowseTransactionsInBlock).Methods("GET")
	r.HandleFunc("/blockchain/blocks/{index}/transactions/{id}", api.handleViewTransactionInBlock).Methods("GET")
	r.HandleFunc("/blockchain/wallets", api.handleBrowseWallets).Methods("GET")
	// POST: creating a wallet is not a safe, idempotent, cacheable operation.
	r.HandleFunc("/blockchain/wallets/new", api.handleCreateWallet).Methods("POST")
	r.HandleFunc("/blockchain/wallets/tx", api.handleCreateWalletTransaction).Methods("POST")
	r.HandleFunc("/blockchain/wallets/{id}", api.handleViewWallet).Methods("GET")
	r.HandleFunc("/blockchain/wallets/{id}", api.handleUpdateWallet).Methods("POST")
	r.HandleFunc("/blockchain/wallets/{id}/balance", api.handleViewWalletBalance).Methods("GET")
	r.HandleFunc("/blockchain/wallets/{id}/transactions", api.handleBrowseTransactionsForWallet).Methods("GET")
	// Was "/blockchain/wallets/{id}/transactions/{id}" -- two variables with the
	// same name, of which mux keeps only one. The handler worked around it by
	// re-parsing r.URL.Path by hand; with distinct names mux.Vars is usable again.
	r.HandleFunc("/blockchain/wallets/{id}/transactions/{txid}", api.handleViewTransactionForWallet).Methods("GET")
	r.HandleFunc("/blockchain/transactions", api.handleBrowseTransactions).Methods("GET")
	// A single route handles both an ID and a protocol name; registering the same
	// pattern twice left the second registration permanently unreachable.
	r.HandleFunc("/blockchain/transactions/{id}", api.handleViewTransaction).Methods("GET")

	// The consensus endpoints are only available to other registered/authorised
	// nodes, so they carry their own authentication on top of the API key.
	consensus := r.PathPrefix("/consensus").Subrouter()
	consensus.Use(authenticateNode)
	consensus.HandleFunc("/p2p", api.handleConsensusP2P).Methods("POST")
	consensus.HandleFunc("/tx", api.handleConsensusTx).Methods("POST")
	consensus.HandleFunc("/block", api.handleConsensusBlock).Methods("POST")
}
