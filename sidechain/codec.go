// File sidechain/codec.go - Stable JSON encoding for sidechain blocks.
package sidechain

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// WHY AN EXPLICIT WIRE SHAPE
//
// A sidechain block is written by the publisher's node and read back later --
// from the application's own database, from a JSON Lines file, or by a third
// party holding a proof. encoding/json's default for these structs would emit
// Go field names and base64 for every []byte, which is an accident of the Go
// type rather than a format anyone chose, and it would change the moment a
// field was renamed.
//
// The shape below is chosen and pinned instead: snake_case names, hex for every
// byte string (the convention the transaction wire format already uses for
// hashes and signatures), and the timestamp as a decimal string. The last is
// deliberate: Unix nanoseconds today are around 1.7e18, well past the 2^53 at
// which a JavaScript number stops being exact, and a block whose timestamp was
// rounded by a non-Go reader no longer hashes to its own Hash.
//
// The header's own JSON encoding is NOT changed. BlockHeader is already
// embedded in AvailabilityProof, which travels inside AVAILABILITY transactions
// stored in mined blocks; giving the header a MarshalJSON would silently change
// how those historic transactions decode. The wire shape lives here, private,
// and is used only by the types that own it.

// sidechainHeaderWire is the pinned JSON shape of a sidechain block header.
type sidechainHeaderWire struct {
	Version           uint32 `json:"version"`
	PublisherID       uint64 `json:"publisher_id"`
	GameID            uint64 `json:"game_id"`
	Height            uint64 `json:"height"`
	PreviousHash      string `json:"previous_hash"`
	PayloadRoot       string `json:"payload_root"`
	TimestampUnixNano int64  `json:"timestamp_unix_nano,string"`
	PayloadCount      uint32 `json:"payload_count"`
}

// headerToWire projects a header onto its pinned JSON shape.
func headerToWire(h BlockHeader) sidechainHeaderWire {
	return sidechainHeaderWire{
		Version:           h.Version,
		PublisherID:       h.PublisherID,
		GameID:            h.GameID,
		Height:            h.Height,
		PreviousHash:      hex.EncodeToString(h.PreviousHash),
		PayloadRoot:       hex.EncodeToString(h.PayloadRoot),
		TimestampUnixNano: h.TimestampUnixNano,
		PayloadCount:      h.PayloadCount,
	}
}

// headerFromWire rebuilds a header from its pinned JSON shape.
//
// It decodes; it does not judge. Whether the header is consistent with anything
// is the business of Validate, Restore and the proof verifiers, which
// is where a caller expects that question to be answered.
func headerFromWire(w sidechainHeaderWire) (BlockHeader, error) {
	previous, err := hex.DecodeString(w.PreviousHash)
	if err != nil {
		return BlockHeader{}, fmt.Errorf("%w: previous hash is not valid hex: %w",
			ErrInvalidSidechainBlock, err)
	}
	root, err := hex.DecodeString(w.PayloadRoot)
	if err != nil {
		return BlockHeader{}, fmt.Errorf("%w: payload root is not valid hex: %w",
			ErrInvalidSidechainBlock, err)
	}
	return BlockHeader{
		Version:           w.Version,
		PublisherID:       w.PublisherID,
		GameID:            w.GameID,
		Height:            w.Height,
		PreviousHash:      previous,
		PayloadRoot:       root,
		TimestampUnixNano: w.TimestampUnixNano,
		PayloadCount:      w.PayloadCount,
	}, nil
}

// sidechainBlockWire is the pinned JSON shape of a sidechain block.
type sidechainBlockWire struct {
	Header   sidechainHeaderWire `json:"header"`
	Hash     string              `json:"hash"`
	Payloads []string            `json:"payloads"`
}

// MarshalJSON encodes the block in its stable wire form.
//
//	{"header":{"version":1,"publisher_id":1,"game_id":1,"height":0,
//	           "previous_hash":"","payload_root":"<hex>",
//	           "timestamp_unix_nano":"1700000000000000000","payload_count":1},
//	 "hash":"<hex>","payloads":["<hex>", ...]}
func (b Block) MarshalJSON() ([]byte, error) {
	payloads := make([]string, 0, len(b.Payloads))
	for _, payload := range b.Payloads {
		payloads = append(payloads, hex.EncodeToString(payload))
	}
	return json.Marshal(sidechainBlockWire{
		Header:   headerToWire(b.Header),
		Hash:     hex.EncodeToString(b.Hash),
		Payloads: payloads,
	})
}

// UnmarshalJSON decodes a block from its stable wire form.
//
// The decoded block is not validated here: decoding and judging are separate
// steps, so a caller can inspect a block that fails validation (to report what
// is wrong with it) rather than receiving only a parse error. Append and
// Restore validate before anything is trusted.
func (b *Block) UnmarshalJSON(data []byte) error {
	var w sidechainBlockWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}

	header, err := headerFromWire(w.Header)
	if err != nil {
		return err
	}
	hash, err := hex.DecodeString(w.Hash)
	if err != nil {
		return fmt.Errorf("%w: hash is not valid hex: %w", ErrInvalidSidechainBlock, err)
	}
	if len(w.Payloads) > MaxPayloads {
		return fmt.Errorf("%w: %d payloads exceeds the %d limit",
			ErrInvalidSidechainBlock, len(w.Payloads), MaxPayloads)
	}

	payloads := make([][]byte, 0, len(w.Payloads))
	for i, encoded := range w.Payloads {
		payload, err := hex.DecodeString(encoded)
		if err != nil {
			return fmt.Errorf("%w: payload %d is not valid hex: %w",
				ErrInvalidSidechainBlock, i, err)
		}
		payloads = append(payloads, payload)
	}

	b.Header = header
	b.Hash = hash
	b.Payloads = payloads
	return nil
}
