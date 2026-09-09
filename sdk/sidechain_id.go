// Package sdk is a software development kit for building blockchain applications.
// File sdk/sidechain_id.go - Sidechain identity: (publisher, game).
package sdk

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
)

// SidechainID names one game's block space.
//
// A publisher registers once and receives a PublisherID; each of their games
// receives a GameID. The pair is the sidechain, and it is the unit of isolation:
// nothing in one publisher's space can affect another's, and nothing in one game
// can affect that publisher's other games.
//
// Both are fixed-width integers rather than strings. They are compared,
// range-scanned and used as map keys constantly, and they have to survive a
// protobuf round trip unchanged -- a numeric identity does all of that without a
// canonicalisation question (is "Acme" the same publisher as "acme"?) attached to
// every comparison.
type SidechainID struct {
	PublisherID uint64
	GameID      uint64
}

var (
	// ErrInvalidSidechainID is returned for an unusable identity.
	ErrInvalidSidechainID = errors.New("invalid sidechain id")

	// ErrSidechainMismatch is returned when a block belongs to another chain.
	ErrSidechainMismatch = errors.New("block belongs to a different sidechain")
)

// NewSidechainID builds an identity, rejecting the zero values.
//
// Zero is reserved: it is what an unset protobuf field decodes to, so allowing it
// would make "no publisher" and "publisher 0" the same value, and a message that
// simply omitted the field would address a real sidechain.
func NewSidechainID(publisherID, gameID uint64) (SidechainID, error) {
	if publisherID == 0 {
		return SidechainID{}, fmt.Errorf("%w: publisher id must not be zero", ErrInvalidSidechainID)
	}
	if gameID == 0 {
		return SidechainID{}, fmt.Errorf("%w: game id must not be zero", ErrInvalidSidechainID)
	}
	return SidechainID{PublisherID: publisherID, GameID: gameID}, nil
}

// Validate reports whether the identity is usable.
func (id SidechainID) Validate() error {
	_, err := NewSidechainID(id.PublisherID, id.GameID)
	return err
}

// IsZero reports whether the identity is unset.
func (id SidechainID) IsZero() bool {
	return id.PublisherID == 0 && id.GameID == 0
}

// Bytes returns the canonical 16-byte encoding: publisher then game, big-endian.
//
// Fixed width, so two identities can never produce the same bytes -- the hazard a
// delimited text form would carry, where "1|23" and "12|3" collide.
func (id SidechainID) Bytes() []byte {
	out := make([]byte, 16)
	binary.BigEndian.PutUint64(out[0:8], id.PublisherID)
	binary.BigEndian.PutUint64(out[8:16], id.GameID)
	return out
}

// String returns a human-readable form, used in logs and as a map key.
func (id SidechainID) String() string {
	return fmt.Sprintf("%d:%d", id.PublisherID, id.GameID)
}

// Hex returns the canonical encoding as hex, for use in APIs and storage paths.
func (id SidechainID) Hex() string {
	return hex.EncodeToString(id.Bytes())
}

// SidechainIDFromBytes decodes the canonical encoding.
func SidechainIDFromBytes(data []byte) (SidechainID, error) {
	if len(data) != 16 {
		return SidechainID{}, fmt.Errorf("%w: expected 16 bytes, got %d",
			ErrInvalidSidechainID, len(data))
	}
	return NewSidechainID(
		binary.BigEndian.Uint64(data[0:8]),
		binary.BigEndian.Uint64(data[8:16]),
	)
}

// Equal reports whether two identities name the same sidechain.
func (id SidechainID) Equal(other SidechainID) bool {
	return id.PublisherID == other.PublisherID && id.GameID == other.GameID
}
