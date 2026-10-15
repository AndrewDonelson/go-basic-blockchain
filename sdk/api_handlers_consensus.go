// Package sdk is a software development kit for building blockchain applications.
// File sdk/api_handlers_consensus.go - Handlers for the node-to-node consensus endpoints.
package sdk

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// handleConsensusP2P handles the consensus/P2P endpoint. This is used to receive and process a Broadcast a Message to 1/3. Upon validation it is then
// broadcast to 2/3 of all nodes. Finally upon validation it is Broadcast to all nodes
// 1. get the post data and unmarshal it into a P2PTransaction
// 2. Add the transaction to the P2P queue
func (api *API) handleConsensusP2P(w http.ResponseWriter, r *http.Request) {
	// get the post data and unmarshal it into a P2PTransaction
	data, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	var tx P2PTransaction
	err = json.Unmarshal(data, &tx)
	if err != nil {
		RespondError(w, http.StatusBadRequest, err.Error())
		return
	}

	if node == nil || node.P2P == nil {
		RespondError(w, http.StatusServiceUnavailable, "Consensus service unavailable")
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

	if err := verifyIncomingTransaction(tx); err != nil {
		respondJSON(w, http.StatusUnprocessableEntity, consensusResult{
			Status: "rejected", Accepted: false, Reason: err.Error(),
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

// verifyIncomingTransaction is the check every transaction arriving over the API
// passes before it is queued, whether it comes from a peer (/consensus/tx) or
// from an application (/v1/transactions).
//
// It is one function on purpose. Two copies of "validate, find the sender, check
// the signature" would drift, and the endpoint that drifted looser would become
// the way to get past the other one. The error text is the reason reported to
// the caller, unchanged from what /consensus/tx has always said -- with one
// added check, that the sender's address belongs to the key the signature is
// verified against, without which the signature proves nothing about who may
// spend from that address.
func verifyIncomingTransaction(tx Transaction) error {
	if err := tx.Validate(); err != nil {
		return err
	}

	if err := VerifyTransactionAuthority(tx); err != nil {
		return err
	}
	return nil
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
