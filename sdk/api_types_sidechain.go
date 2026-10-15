// Package sdk is a software development kit for building blockchain applications.
// File sdk/api_types_sidechain.go - Response shapes for the application-facing endpoints.
package sdk

import "github.com/AndrewDonelson/go-basic-blockchain/sidechain"

// The JSON bodies of the application-facing endpoints are defined once, in the
// dependency-free sidechain package, so the node, this package's callers and
// applications that import only sidechain decode one definition.

// Transaction submission statuses.
const (
	SubmissionPending   = sidechain.SubmissionPending
	SubmissionConfirmed = sidechain.SubmissionConfirmed
	SubmissionRejected  = sidechain.SubmissionRejected
)

type (
	// TransactionSubmission is the reply to POST /v1/transactions.
	TransactionSubmission = sidechain.TransactionSubmission
	// GameInfo is a registered game, as the API reports it.
	GameInfo = sidechain.GameInfo
	// PublisherInfo is a registered publisher, as the API reports it.
	PublisherInfo = sidechain.PublisherInfo
	// AnchorInfo is one anchor recorded on the main chain, as the API reports it.
	AnchorInfo = sidechain.AnchorInfo
	// AnchorList is the reply to GET /v1/sidechains/{publisherID}/{gameID}/anchors.
	AnchorList = sidechain.AnchorList
	// AccountInfo is the reply to GET /v1/accounts/{address}.
	AccountInfo = sidechain.AccountInfo
	// TransactionInfo is the reply to GET /v1/transactions/{id}.
	TransactionInfo = sidechain.TransactionInfo
)

// newPublisherInfo projects a registry record onto the API shape.
func newPublisherInfo(record PublisherRecord) PublisherInfo {
	// Registered keys were parsed on registration, so this cannot fail in
	// practice; an unparseable one is reported with no fingerprint.
	fingerprint, err := PublisherKeyFingerprint(record.PublicKeyPEM)
	if err != nil {
		fingerprint = ""
	}

	games := make([]GameInfo, 0)
	for _, game := range record.Games() {
		games = append(games, GameInfo(game))
	}

	return PublisherInfo{
		PublisherID:        record.PublisherID,
		Name:               record.Name,
		PublicKeyPEM:       record.PublicKeyPEM,
		KeyFingerprint:     fingerprint,
		BondUnits:          record.BondUnits,
		RegisteredAtHeight: record.RegisteredAtHeight,
		Games:              games,
	}
}
