// Package sdk is a software development kit for building blockchain applications.
// File sdk/api_respond.go - Response helpers: the error envelope, pagination, serialisation.
package sdk

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// ErrorResponse represents the structure of an error response.
// ErrorResponse is the single shape every API error takes.
//
// Handlers used to mix this JSON envelope with http.Error's plain text, so a
// client could not parse an error without first guessing which of the two it had
// received -- and the choice varied by handler, not by error kind. Status is
// included in the body because a client that only has the payload (a log line, a
// queued webhook) otherwise cannot tell a 400 from a 500.
type ErrorResponse struct {
	Message string `json:"message"`
	Status  int    `json:"status"`
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

// RespondError sends an error response with the given status code and message.
func RespondError(w http.ResponseWriter, statusCode int, message string) {
	// Set the Content-Type header and status code
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Create an ErrorResponse instance
	errorResponse := ErrorResponse{Message: message, Status: statusCode}

	// Encode and send the error message as JSON
	if err := json.NewEncoder(w).Encode(errorResponse); err != nil {
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
		RespondError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
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
		RespondError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		RespondError(w, http.StatusInternalServerError, "Internal server error")
		return
	}
}

func (api *API) respondTransactionList(w http.ResponseWriter, txs []Transaction) {
	w.Header().Set("Content-Type", "application/json")
	data, err := json.Marshal(txs)
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
