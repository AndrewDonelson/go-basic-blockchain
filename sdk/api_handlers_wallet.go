// Package sdk is a software development kit for building blockchain applications.
// File sdk/api_handlers_wallet.go - Handlers for wallets and wallet transactions.
package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

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
		RespondError(w, http.StatusInternalServerError, "Internal Server Error")
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
		RespondError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		RespondError(w, http.StatusInternalServerError, "Internal server error")
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
		RespondError(w, http.StatusBadRequest, "Invalid wallet ID")
		return
	}

	wallet, err := api.loadWalletByAddress(address)
	if err != nil {
		RespondError(w, http.StatusNotFound, "Wallet not found")
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
		RespondError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
}

// handleUpdateWallet handles the /blockchain/wallets/{id} endpoint.
func (api *API) handleUpdateWallet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["id"]
	if address == "" {
		RespondError(w, http.StatusBadRequest, "Invalid wallet ID")
		return
	}

	type updateWalletRequest struct {
		Passphrase string   `json:"passphrase"`
		Name       string   `json:"name"`
		Tags       []string `json:"tags"`
	}

	var req updateWalletRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, apiMaxBodyBytes)).Decode(&req); err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Passphrase == "" {
		RespondError(w, http.StatusBadRequest, "Passphrase is required")
		return
	}

	hasNameUpdate := req.Name != ""
	hasTagsUpdate := req.Tags != nil
	if !hasNameUpdate && !hasTagsUpdate {
		RespondError(w, http.StatusBadRequest, "No update fields provided")
		return
	}

	wallet, err := api.loadWalletByAddress(address)
	if err != nil {
		RespondError(w, http.StatusNotFound, "Wallet not found")
		return
	}

	if err := wallet.Unlock(req.Passphrase); err != nil {
		RespondError(w, http.StatusUnauthorized, "Invalid passphrase")
		return
	}

	if hasNameUpdate {
		if err := wallet.SetData("name", req.Name); err != nil {
			RespondError(w, http.StatusInternalServerError, "Failed to update wallet name")
			return
		}
	}

	if hasTagsUpdate {
		if err := wallet.SetData("tags", req.Tags); err != nil {
			RespondError(w, http.StatusInternalServerError, "Failed to update wallet tags")
			return
		}
	}

	if err := wallet.Close(req.Passphrase); err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to persist wallet updates")
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
		RespondError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
}

// handleViewWalletBalance handles the /blockchain/wallets/{id}/balance endpoint.
func (api *API) handleViewWalletBalance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	address := vars["id"]
	if address == "" {
		RespondError(w, http.StatusBadRequest, "Invalid wallet ID")
		return
	}

	_, err := api.loadWalletByAddress(address)
	if err != nil {
		RespondError(w, http.StatusNotFound, "Wallet not found")
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
		RespondError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
}

// handleBrowseTransactionsForWallet handles the /blockchain/wallets/{id}/transactions endpoint.
func (api *API) handleBrowseTransactionsForWallet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	walletID := vars["id"]
	if walletID == "" {
		RespondError(w, http.StatusBadRequest, "Invalid wallet ID")
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
		RespondError(w, http.StatusBadRequest, "Invalid wallet transaction path")
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

	RespondError(w, http.StatusNotFound, "Transaction not found")
}

// handleBrowseTransactionsByProtocolForWallet handles the /blockchain/wallets/{id}/transactions/{protocol} endpoint.
func (api *API) handleBrowseTransactionsByProtocolForWallet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	walletID := vars["id"]
	protocolStr := vars["protocol"]

	if walletID == "" {
		RespondError(w, http.StatusBadRequest, "Invalid wallet ID")
		return
	}

	protocol, ok := normalizeProtocol(protocolStr)
	if !ok {
		RespondError(w, http.StatusBadRequest, "Invalid protocol")
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
