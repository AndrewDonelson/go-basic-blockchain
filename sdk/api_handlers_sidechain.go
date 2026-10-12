// Package sdk is a software development kit for building blockchain applications.
// File sdk/api_handlers_sidechain.go - Application-facing endpoints: submit, publishers, anchors.
package sdk

import (
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
)

// WHY THESE ENDPOINTS EXIST
//
// An application could already build sidechain blocks, anchors and
// registrations in memory, and had no way to get any of it onto the chain or to
// read the result back. /consensus/tx accepts transactions, but only from
// authenticated peer nodes; the publisher registry and the anchor ledger were
// reachable only from inside the process. These endpoints are the missing half:
// submit a signed transaction, then read back what consensus made of it.

// handleSubmitTransaction handles POST /v1/transactions.
//
// It takes a signed transaction in exactly the wire format /consensus/tx takes,
// and runs exactly the same checks (verifyIncomingTransaction) -- the only
// difference is who may call it. It answers:
//
//	202 {"id":..,"status":"pending"}               newly queued, and relayed
//	200 {"id":..,"status":"pending|confirmed","known":true}  already known
//	422 {"id":..,"status":"rejected","reason":..}  refused, with the reason
//	400 error envelope                             the body did not decode
//
// Resubmitting a known id is a success, not an error. An application that timed
// out waiting for the first reply cannot tell whether it arrived, and must be
// able to retry without that retry looking like a failure.
func (api *API) handleSubmitTransaction(w http.ResponseWriter, r *http.Request) {
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

	id := tx.GetID()
	reject := func(reason string) {
		respondJSON(w, http.StatusUnprocessableEntity, TransactionSubmission{
			ID: id, Status: SubmissionRejected, Reason: reason,
		})
	}

	// Refused before anything else: a coinbase mints supply, and no caller of a
	// public endpoint is ever entitled to one. AddTransaction would refuse it
	// too, but saying so explicitly gives the caller a reason instead of a
	// generic refusal.
	if tx.GetProtocol() == CoinbaseProtocolID {
		reject(ErrCoinbaseSubmitted.Error())
		return
	}

	if err := verifyIncomingTransaction(tx); err != nil {
		reject(err.Error())
		return
	}

	if api.bc.HasTransactionID(id) {
		respondJSON(w, http.StatusOK, TransactionSubmission{
			ID: id, Status: api.knownTransactionStatus(id), Known: true,
		})
		return
	}

	if err := api.bc.SubmitTransaction(tx); err != nil {
		// Lost a race with an identical submission between the check above and
		// here: still the idempotent case.
		if errors.Is(err, ErrTransactionAlreadyQueued) {
			respondJSON(w, http.StatusOK, TransactionSubmission{
				ID: id, Status: SubmissionPending, Known: true,
			})
			return
		}
		reject(err.Error())
		return
	}

	respondJSON(w, http.StatusAccepted, TransactionSubmission{ID: id, Status: SubmissionPending})
}

// knownTransactionStatus reports whether a known id is still in the mempool or
// already mined.
func (api *API) knownTransactionStatus(id string) string {
	for _, pending := range api.bc.GetPendingTransactions() {
		if pending != nil && pending.GetID() == id {
			return SubmissionPending
		}
	}
	return SubmissionConfirmed
}

// handleViewPublisher handles GET /v1/publishers/{id}.
func (api *API) handleViewPublisher(w http.ResponseWriter, r *http.Request) {
	publisherID, err := parseRegistryID(mux.Vars(r)["id"])
	if err != nil {
		RespondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid publisher id: %v", err))
		return
	}

	record, ok := api.bc.UTXOSet().Anchors().Registry().Publisher(publisherID)
	if !ok {
		RespondError(w, http.StatusNotFound, fmt.Sprintf("publisher %d is not registered", publisherID))
		return
	}
	respondJSON(w, http.StatusOK, newPublisherInfo(record))
}

// handleFindPublisher handles GET /v1/publishers?key={fingerprint}.
//
// The key is the identity key's fingerprint (PublisherKeyFingerprint), not the
// PEM. A PEM does not fit in a query string without escaping every newline, and
// it is not canonical anyway -- two encodings of one key would look up
// differently.
func (api *API) handleFindPublisher(w http.ResponseWriter, r *http.Request) {
	fingerprint := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("key")))
	if fingerprint == "" {
		RespondError(w, http.StatusBadRequest,
			"the key query parameter is required: the publisher key's SHA-256 fingerprint")
		return
	}
	if decoded, err := hex.DecodeString(fingerprint); err != nil || len(decoded) != 32 {
		RespondError(w, http.StatusBadRequest,
			"key must be 64 hex characters: the SHA-256 of the identity key's PKIX DER encoding")
		return
	}

	record, ok := api.bc.UTXOSet().Anchors().Registry().PublisherByKeyFingerprint(fingerprint)
	if !ok {
		RespondError(w, http.StatusNotFound, "no publisher is registered with that key")
		return
	}
	respondJSON(w, http.StatusOK, newPublisherInfo(record))
}

// handleSidechainAnchors handles GET /v1/sidechains/{publisherID}/{gameID}/anchors.
//
// The anchors are listed oldest first and paginated like every other collection
// (page, limit). Each carries the main-chain block that holds it, which is what
// a verifier needs to confirm the anchor is really on the chain rather than
// merely reported by this node.
func (api *API) handleSidechainAnchors(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	publisherID, err := parseRegistryID(vars["publisherID"])
	if err != nil {
		RespondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid publisher id: %v", err))
		return
	}
	gameID, err := parseRegistryID(vars["gameID"])
	if err != nil {
		RespondError(w, http.StatusBadRequest, fmt.Sprintf("Invalid game id: %v", err))
		return
	}
	id := SidechainID{PublisherID: publisherID, GameID: gameID}

	ledger := api.bc.UTXOSet().Anchors()

	// An unregistered sidechain is a 404 rather than an empty list: "nothing
	// anchored yet" and "no such sidechain" are different answers, and an
	// application polling for its first anchor needs to tell them apart.
	if _, err := ledger.Registry().AuthoriseSidechain(id); err != nil {
		RespondError(w, http.StatusNotFound, err.Error())
		return
	}

	records := ledger.Anchors(id)
	page, limit := parsePagination(r)

	total := len(records)
	start := (page - 1) * limit
	if start > total {
		start = total
	}
	end := start + limit
	if end > total {
		end = total
	}

	anchors := make([]AnchorInfo, 0, end-start)
	for _, record := range records[start:end] {
		anchors = append(anchors, api.anchorInfo(record))
	}

	respondJSON(w, http.StatusOK, AnchorList{
		PublisherID: publisherID,
		GameID:      gameID,
		Delinquent:  ledger.IsDelinquent(id),
		Page:        page,
		Limit:       limit,
		Total:       total,
		Anchors:     anchors,
	})
}

// anchorInfo projects a ledger record onto the API shape, locating the block
// and transaction that carry it.
func (api *API) anchorInfo(record AnchorRecord) AnchorInfo {
	anchor := record.Anchor
	info := AnchorInfo{
		PublisherID:    anchor.PublisherID,
		GameID:         anchor.GameID,
		FromHeight:     anchor.FromHeight,
		ToHeight:       anchor.ToHeight,
		TipHash:        hex.EncodeToString(anchor.TipHash),
		HeaderRoot:     hex.EncodeToString(anchor.HeaderRoot),
		PayloadCount:   anchor.PayloadCount,
		MainBlockIndex: record.MainHeight,
	}

	block := api.bc.GetBlockByIndex(record.MainHeight)
	if block == nil {
		return info
	}
	info.MainBlockHash = block.Hash

	// A sidechain's anchors are contiguous, so (sidechain, FromHeight) names
	// exactly one of them.
	for _, tx := range block.Transactions {
		carrier, ok := tx.(*AnchorTx)
		if !ok {
			continue
		}
		if carrier.Anchor.SidechainID().Equal(anchor.SidechainID()) &&
			carrier.Anchor.FromHeight == anchor.FromHeight {
			info.TransactionID = carrier.GetID()
			break
		}
	}
	return info
}

// parseRegistryID parses a publisher or game id from a path segment.
//
// Zero is refused for the reason it is refused everywhere else: it is what an
// omitted protobuf field decodes to, and it names nothing.
func parseRegistryID(raw string) (uint64, error) {
	id, err := strconv.ParseUint(raw, 10, 64)
	if err != nil {
		return 0, errors.New("must be a positive decimal integer")
	}
	if id == 0 {
		return 0, errors.New("must not be zero")
	}
	return id, nil
}
