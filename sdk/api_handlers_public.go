// Package sdk is a software development kit for building blockchain applications.
// File sdk/api_handlers_public.go - Handlers for the unauthenticated endpoints.
package sdk

import (
	"encoding/json"
	"html/template"
	"net/http"
)

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
		RespondError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	// Render the HTML template with the data
	err = tmpl.Execute(w, info)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Internal Server Error")
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
		RespondError(w, http.StatusInternalServerError, "Internal Server Error")
		return
	}

	w.WriteHeader(http.StatusOK)
	if _, err := w.Write(data); err != nil {
		RespondError(w, http.StatusInternalServerError, "Internal server error")
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
