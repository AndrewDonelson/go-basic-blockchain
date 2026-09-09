// Package sdk is a software development kit for building blockchain applications.
// File sdk/api_handlers_chain.go - Handlers for blocks and transactions.
package sdk

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

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
		RespondError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	// Write the JSON response
	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
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
		RespondError(w, http.StatusBadRequest, "Invalid block index")
		return
	}

	block := api.bc.GetBlockByIndex(int64(index))
	if block == nil {
		RespondError(w, http.StatusNotFound, "Block not found")
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
		RespondError(w, http.StatusBadRequest, "Invalid block index")
		return
	}

	block := api.bc.GetBlockByIndex(int64(index))
	if block == nil {
		RespondError(w, http.StatusBadRequest, "Block index out of range")
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
		RespondError(w, http.StatusBadRequest, "Invalid block index")
		return
	}

	block := api.bc.GetBlockByIndex(int64(index))
	if block == nil {
		RespondError(w, http.StatusNotFound, "Block not found")
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
				RespondError(w, http.StatusInternalServerError, "Internal Server Error")
				return
			}

			w.WriteHeader(http.StatusOK)
			if _, err := w.Write(data); err != nil {
				RespondError(w, http.StatusInternalServerError, "Internal server error")
				return
			}
			return
		}
	}

	RespondError(w, http.StatusNotFound, "Transaction not found")
}

// handleBrowseTransactionsByProtocolInBlock handles the /blockchain/blocks/{index}/transactions/{protocol} endpoint.
func (api *API) handleBrowseTransactionsByProtocolInBlock(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	indexStr := vars["index"]
	protocolStr := vars["protocol"]

	index, err := strconv.Atoi(indexStr)
	if err != nil {
		RespondError(w, http.StatusBadRequest, "Invalid block index")
		return
	}

	if index < 0 || index >= len(api.bc.Blocks) {
		RespondError(w, http.StatusNotFound, "Block not found")
		return
	}

	protocol, ok := normalizeProtocol(protocolStr)
	if !ok {
		RespondError(w, http.StatusBadRequest, "Invalid protocol")
		return
	}

	api.respondTransactionsByProtocol(w, api.bc.Blocks[index], protocol)
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
		RespondError(w, http.StatusBadRequest, "Invalid transaction identifier")
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

	RespondError(w, http.StatusNotFound, "Transaction not found")
}

// handleBrowseTransactionsByProtocol handles the /blockchain/transactions/{protocol} endpoint.
func (api *API) handleBrowseTransactionsByProtocol(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	protocolStr := vars["protocol"]

	protocol, ok := normalizeProtocol(protocolStr)
	if !ok {
		RespondError(w, http.StatusBadRequest, "Invalid protocol")
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
