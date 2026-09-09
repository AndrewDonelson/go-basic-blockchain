// Package sdk is a software development kit for building blockchain applications.
// File sdk/anchortx.go - the transaction that carries a sidechain anchor.
package sdk

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
)

// AnchorTx commits a range of a sidechain's history to the main chain.
//
// The anchor mechanism is only worth something if the commitment lands somewhere
// the publisher cannot rewrite. That is this transaction: once it is in a mined
// block, the publisher's claim about their history is fixed by the same
// proof-of-work that fixes everybody else's balances.
//
// It carries no game data -- only the commitment -- so the main chain's cost
// scales with the number of publishers rather than with how much their players
// do.
type AnchorTx struct {
	Tx
	Anchor SidechainAnchor
	// AnchorSignature is the publisher's signature over Anchor.SigningBytes().
	//
	// It is separate from Tx.Signature deliberately. The transaction signature
	// says "this account authorised this transaction and its fee"; the anchor
	// signature says "the publisher who owns this sidechain vouches for this
	// history". They can be different keys, and conflating them would mean a
	// publisher had to spend from the same wallet that owns their identity.
	AnchorSignature []byte
}

// NewAnchorTransaction creates a transaction carrying a signed anchor.
func NewAnchorTransaction(from *Wallet, anchor SidechainAnchor, signature []byte) (*AnchorTx, error) {
	if err := anchor.Validate(); err != nil {
		return nil, err
	}
	if len(signature) == 0 {
		return nil, fmt.Errorf("%w: anchor is unsigned", ErrAnchorUnauthorised)
	}

	// The anchor is addressed to nobody: it moves no value, it records a fact.
	tx, err := NewTransaction(AnchorProtocolID, from, from)
	if err != nil {
		return nil, err
	}

	return &AnchorTx{
		Tx:              *tx,
		Anchor:          anchor,
		AnchorSignature: append([]byte{}, signature...),
	}, nil
}

// MarshalJSON encodes the anchor transaction in the canonical wire form.
func (a *AnchorTx) MarshalJSON() ([]byte, error) {
	w := a.Tx.toWire()
	w.PublisherID = a.Anchor.PublisherID
	w.GameID = a.Anchor.GameID
	w.FromHeight = a.Anchor.FromHeight
	w.ToHeight = a.Anchor.ToHeight
	w.TipHash = hex.EncodeToString(a.Anchor.TipHash)
	w.HeaderRoot = hex.EncodeToString(a.Anchor.HeaderRoot)
	w.AnchorPayloads = a.Anchor.PayloadCount
	w.AnchorSignature = hex.EncodeToString(a.AnchorSignature)
	return json.Marshal(w)
}

// UnmarshalJSON decodes an anchor transaction from the canonical wire form.
func (a *AnchorTx) UnmarshalJSON(data []byte) error {
	var w txWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	a.Tx.applyWire(w)

	anchor, signature, err := anchorFromWire(w)
	if err != nil {
		return err
	}
	a.Anchor = anchor
	a.AnchorSignature = signature
	return nil
}

// anchorFromWire rebuilds an anchor and its signature from the wire shape.
//
// Shared by UnmarshalJSON and DecodeTransaction. Those are two decode paths for
// the same bytes, and the last time a protocol had two that drifted apart, the
// coinbase subsidy silently decoded as zero and every reloaded block was
// rejected. One function means they cannot diverge.
func anchorFromWire(w txWire) (SidechainAnchor, []byte, error) {
	tipHash, err := hex.DecodeString(w.TipHash)
	if err != nil {
		return SidechainAnchor{}, nil, fmt.Errorf("%w: tip hash is not valid hex: %w",
			ErrInvalidAnchor, err)
	}
	headerRoot, err := hex.DecodeString(w.HeaderRoot)
	if err != nil {
		return SidechainAnchor{}, nil, fmt.Errorf("%w: header root is not valid hex: %w",
			ErrInvalidAnchor, err)
	}
	signature, err := hex.DecodeString(w.AnchorSignature)
	if err != nil {
		return SidechainAnchor{}, nil, fmt.Errorf("%w: anchor signature is not valid hex: %w",
			ErrAnchorUnauthorised, err)
	}

	return SidechainAnchor{
		PublisherID:  w.PublisherID,
		GameID:       w.GameID,
		FromHeight:   w.FromHeight,
		ToHeight:     w.ToHeight,
		TipHash:      tipHash,
		HeaderRoot:   headerRoot,
		PayloadCount: w.AnchorPayloads,
	}, signature, nil
}

// SidechainID returns the sidechain this transaction commits.
func (a *AnchorTx) SidechainID() SidechainID { return a.Anchor.SidechainID() }

// Process describes the commitment.
func (a *AnchorTx) Process() string {
	a.Status = StatusConfirmed
	return fmt.Sprintf("Anchored sidechain %s heights %d..%d at %s",
		a.Anchor.SidechainID(), a.Anchor.FromHeight, a.Anchor.ToHeight,
		hex.EncodeToString(a.Anchor.TipHash))
}

// SigningBytes covers the anchor as well as the base fields.
//
// The anchor signature is deliberately excluded: it is a signature, and signing
// over it would only bind this transaction to one particular encoding of it. What
// must be covered is every field the anchor asserts, so the committed range
// cannot be edited after the sender signs.
func (a *AnchorTx) SigningBytes() ([]byte, error) {
	fields := a.Tx.signingFields()
	fields["anchor_publisher_id"] = a.Anchor.PublisherID
	fields["anchor_game_id"] = a.Anchor.GameID
	fields["anchor_from_height"] = a.Anchor.FromHeight
	fields["anchor_to_height"] = a.Anchor.ToHeight
	fields["anchor_tip_hash"] = hex.EncodeToString(a.Anchor.TipHash)
	fields["anchor_header_root"] = hex.EncodeToString(a.Anchor.HeaderRoot)
	fields["anchor_payload_count"] = a.Anchor.PayloadCount
	return json.Marshal(fields)
}

// Sign signs the full anchor transaction.
func (a *AnchorTx) Sign(privPEM []byte) (string, error) {
	payload, err := a.SigningBytes()
	if err != nil {
		return "", fmt.Errorf("error marshaling transaction: %w", err)
	}
	return signPayload(payload, privPEM)
}

// Verify verifies a signature over the full anchor transaction.
func (a *AnchorTx) Verify(pubKey []byte, sign string) (bool, error) {
	payload, err := a.SigningBytes()
	if err != nil {
		return false, fmt.Errorf("error marshaling transaction: %w", err)
	}
	return verifyPayload(payload, pubKey, sign)
}

// Hash covers the anchor as well as the base fields.
func (a *AnchorTx) Hash() string {
	a.Tx.hash = hashTransaction(a)
	return a.Tx.hash
}

// Bytes returns the canonical encoding.
func (a *AnchorTx) Bytes() []byte {
	payload, err := a.SigningBytes()
	if err != nil {
		return nil
	}
	return payload
}

// Size reports the size of the full transaction.
func (a *AnchorTx) Size() int { return len(a.Bytes()) }

// EstimateFee is derived from the full transaction size.
func (a *AnchorTx) EstimateFee(feePerByte float64) float64 {
	return float64(a.Size()) * feePerByte
}

// Send queues the anchor transaction itself rather than its base transaction.
func (a *AnchorTx) Send(bc *Blockchain) error {
	if err := a.Validate(); err != nil {
		return fmt.Errorf("invalid transaction: %w", err)
	}
	bc.AddTransaction(a)
	return nil
}

// Validate checks the base transaction plus anchor-specific invariants.
//
// What it deliberately does NOT check is contiguity or publisher authorisation.
// Those are questions about chain state -- what has already been anchored, and
// which key owns this publisher -- and a transaction cannot answer them on its
// own. They are enforced where the state lives, when the block is applied.
func (a *AnchorTx) Validate() error {
	if err := a.Tx.Validate(); err != nil {
		return err
	}
	if a.Tx.Protocol != AnchorProtocolID {
		return fmt.Errorf("anchor transaction has wrong protocol: %s", a.Tx.Protocol)
	}
	if err := a.Anchor.Validate(); err != nil {
		return err
	}
	if len(a.AnchorSignature) == 0 {
		return errors.New("anchor transaction carries no publisher signature")
	}
	return nil
}

// GetID is nil-safe, matching the other protocol transactions.
func (a *AnchorTx) GetID() string {
	if a == nil || a.Tx.ID == nil {
		return ""
	}
	return a.Tx.GetID()
}
