// Package sdk is a software development kit for building blockchain applications.
// File sdk/api_types_sidechain.go - Response shapes for the application-facing endpoints.
package sdk

import (
	"encoding/hex"
	"fmt"
)

// These types are the JSON bodies of POST /v1/transactions and the publisher and
// sidechain read endpoints. They are exported so the server and every Go client
// (see the client package) decode one definition: a client that kept its own
// copy would be one renamed field away from silently reading zeroes.

// Transaction submission statuses.
const (
	// SubmissionPending means the transaction is in the mempool, waiting to be
	// mined.
	SubmissionPending = "pending"
	// SubmissionConfirmed means a transaction with this id is already in a
	// block on this node's chain.
	SubmissionConfirmed = "confirmed"
	// SubmissionRejected means the transaction was refused; Reason says why.
	SubmissionRejected = "rejected"
)

// TransactionSubmission is the reply to POST /v1/transactions.
type TransactionSubmission struct {
	// ID is the transaction's id, present whenever the body decoded.
	ID string `json:"id,omitempty"`
	// Status is one of SubmissionPending, SubmissionConfirmed, SubmissionRejected.
	Status string `json:"status"`
	// Known is true when the node already had this id. Submitting the same
	// transaction twice is therefore safe: the second call reports what
	// happened to the first.
	Known bool `json:"known,omitempty"`
	// Reason explains a rejection.
	Reason string `json:"reason,omitempty"`
}

// GameInfo is a registered game, as the API reports it.
type GameInfo struct {
	GameID             uint64 `json:"game_id"`
	Name               string `json:"name"`
	RegisteredAtHeight int64  `json:"registered_at_height"`
}

// PublisherInfo is a registered publisher, as the API reports it.
type PublisherInfo struct {
	PublisherID uint64 `json:"publisher_id"`
	Name        string `json:"name"`
	// PublicKeyPEM is the identity key that signs this publisher's anchors and
	// game registrations.
	PublicKeyPEM string `json:"public_key_pem"`
	// KeyFingerprint is PublisherKeyFingerprint(PublicKeyPEM): the value
	// GET /v1/publishers?key= looks a publisher up by.
	KeyFingerprint     string     `json:"key_fingerprint"`
	BondUnits          int64      `json:"bond_units"`
	RegisteredAtHeight int64      `json:"registered_at_height"`
	Games              []GameInfo `json:"games"`
}

// GamesNamed returns the publisher's games registered under name, in id order.
// Game names are not unique, so every match is returned.
func (p PublisherInfo) GamesNamed(name string) []GameInfo {
	var out []GameInfo
	for _, game := range p.Games {
		if game.Name == name {
			out = append(out, game)
		}
	}
	return out
}

// newPublisherInfo projects a registry record onto the API shape.
func newPublisherInfo(record PublisherRecord) PublisherInfo {
	fingerprint, _ := PublisherKeyFingerprint(record.PublicKeyPEM)

	games := make([]GameInfo, 0)
	for _, game := range record.Games() {
		games = append(games, GameInfo{
			GameID:             game.GameID,
			Name:               game.Name,
			RegisteredAtHeight: game.RegisteredAtHeight,
		})
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

// AnchorInfo is one anchor recorded on the main chain, as the API reports it.
type AnchorInfo struct {
	PublisherID  uint64 `json:"publisher_id"`
	GameID       uint64 `json:"game_id"`
	FromHeight   uint64 `json:"from_height"`
	ToHeight     uint64 `json:"to_height"`
	TipHash      string `json:"tip_hash"`
	HeaderRoot   string `json:"header_root"`
	PayloadCount uint64 `json:"payload_count"`

	// MainBlockIndex is the main-chain block carrying the anchor.
	MainBlockIndex int64 `json:"main_block_index"`
	// MainBlockHash is that block's hash, when this node holds it.
	MainBlockHash string `json:"main_block_hash,omitempty"`
	// TransactionID is the ANCHOR transaction's id, when it can be located.
	TransactionID string `json:"transaction_id,omitempty"`
}

// Anchor converts the reply back into the SidechainAnchor that VerifyRecord
// checks against.
func (a AnchorInfo) Anchor() (SidechainAnchor, error) {
	tipHash, err := hex.DecodeString(a.TipHash)
	if err != nil {
		return SidechainAnchor{}, fmt.Errorf("%w: tip hash is not valid hex: %w", ErrInvalidAnchor, err)
	}
	headerRoot, err := hex.DecodeString(a.HeaderRoot)
	if err != nil {
		return SidechainAnchor{}, fmt.Errorf("%w: header root is not valid hex: %w", ErrInvalidAnchor, err)
	}
	anchor := SidechainAnchor{
		PublisherID:  a.PublisherID,
		GameID:       a.GameID,
		FromHeight:   a.FromHeight,
		ToHeight:     a.ToHeight,
		TipHash:      tipHash,
		HeaderRoot:   headerRoot,
		PayloadCount: a.PayloadCount,
	}
	if err := anchor.Validate(); err != nil {
		return SidechainAnchor{}, err
	}
	return anchor, nil
}

// Covers reports whether the anchor commits a sidechain height.
func (a AnchorInfo) Covers(height uint64) bool {
	return height >= a.FromHeight && height <= a.ToHeight
}

// AnchorList is the reply to GET /v1/sidechains/{publisherID}/{gameID}/anchors.
type AnchorList struct {
	PublisherID uint64 `json:"publisher_id"`
	GameID      uint64 `json:"game_id"`
	// Delinquent reports whether the sidechain has ever left history
	// unanchored for longer than MaxAnchorGap main-chain blocks.
	Delinquent bool         `json:"delinquent"`
	Page       int          `json:"page"`
	Limit      int          `json:"limit"`
	Total      int          `json:"total"`
	Anchors    []AnchorInfo `json:"anchors"`
}
