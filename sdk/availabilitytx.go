// Package sdk is a software development kit for building blockchain applications.
// File sdk/availabilitytx.go - the transaction carrying challenges and proofs.
package sdk

import (
	"encoding/json"
	"fmt"
)

// Availability transaction kinds.
const (
	// AvailabilityKindChallenge asks a publisher to produce a payload.
	AvailabilityKindChallenge uint32 = 1
	// AvailabilityKindProof answers a challenge.
	AvailabilityKindProof uint32 = 2
)

// AvailabilityTx carries either a challenge or the proof answering one.
//
// Both halves are main-chain transactions because both have to be witnessed by
// everyone. A challenge nobody saw cannot expire into a slash, and a proof
// nobody saw cannot clear one.
type AvailabilityTx struct {
	Tx

	Kind      uint32
	Challenge AvailabilityChallenge
	Proof     AvailabilityProof
}

// NewAvailabilityChallenge builds a challenge transaction.
//
// The challenger's address is taken from the wallet paying for it, so the stake
// and the refund necessarily go to the same party.
func NewAvailabilityChallenge(from *Wallet, id SidechainID, height uint64, payloadIndex uint32) (*AvailabilityTx, error) {
	if from == nil {
		return nil, fmt.Errorf("%w: no challenger wallet", ErrChallengeInvalid)
	}

	tx, err := NewTransaction(AvailabilityProtocolID, from, from)
	if err != nil {
		return nil, err
	}

	challenge := AvailabilityChallenge{
		PublisherID:  id.PublisherID,
		GameID:       id.GameID,
		Height:       height,
		PayloadIndex: payloadIndex,
		Challenger:   from.GetAddress(),
	}
	if err := challenge.Validate(); err != nil {
		return nil, err
	}

	return &AvailabilityTx{
		Tx:        *tx,
		Kind:      AvailabilityKindChallenge,
		Challenge: challenge,
	}, nil
}

// NewAvailabilityProof builds a proof transaction answering a challenge.
func NewAvailabilityProof(from *Wallet, proof AvailabilityProof) (*AvailabilityTx, error) {
	if proof.ChallengeID == "" {
		return nil, fmt.Errorf("%w: proof names no challenge", ErrProofRejected)
	}

	tx, err := NewTransaction(AvailabilityProtocolID, from, from)
	if err != nil {
		return nil, err
	}

	return &AvailabilityTx{
		Tx:    *tx,
		Kind:  AvailabilityKindProof,
		Proof: proof,
	}, nil
}

// MarshalJSON encodes the transaction in the canonical wire form.
func (a *AvailabilityTx) MarshalJSON() ([]byte, error) {
	w := a.Tx.toWire()
	w.AvailabilityKind = a.Kind
	if a.Kind == AvailabilityKindChallenge {
		w.AvailabilityChallenge = &a.Challenge
	} else {
		w.AvailabilityProof = &a.Proof
	}
	return json.Marshal(w)
}

// UnmarshalJSON decodes the transaction from the canonical wire form.
func (a *AvailabilityTx) UnmarshalJSON(data []byte) error {
	var w txWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	a.Tx.applyWire(w)

	decoded := availabilityFromWire(w)
	a.Kind = decoded.Kind
	a.Challenge = decoded.Challenge
	a.Proof = decoded.Proof
	return nil
}

// availabilityFromWire rebuilds the availability fields from the wire shape.
//
// Shared by UnmarshalJSON and DecodeTransaction, so the two decode paths cannot
// drift apart.
func availabilityFromWire(w txWire) *AvailabilityTx {
	out := &AvailabilityTx{Kind: w.AvailabilityKind}
	if w.AvailabilityChallenge != nil {
		out.Challenge = *w.AvailabilityChallenge
	}
	if w.AvailabilityProof != nil {
		out.Proof = *w.AvailabilityProof
	}
	return out
}

// Process describes the transaction.
func (a *AvailabilityTx) Process() string {
	a.Status = StatusConfirmed
	if a.Kind == AvailabilityKindChallenge {
		return fmt.Sprintf("Challenging %s to produce payload %d at height %d",
			a.Challenge.SidechainID(), a.Challenge.PayloadIndex, a.Challenge.Height)
	}
	return fmt.Sprintf("Answering availability challenge %s", a.Proof.ChallengeID)
}

// SigningBytes covers the challenge or proof as well as the base fields.
//
// The proof's Merkle paths are not signed over: they are verified against a
// commitment already on the chain, so a tampered path fails verification rather
// than being accepted under a stale signature. What must be signed is which
// challenge is being answered, so an answer cannot be redirected to a different
// one.
func (a *AvailabilityTx) SigningBytes() ([]byte, error) {
	fields := a.Tx.signingFields()
	fields["availability_kind"] = a.Kind

	if a.Kind == AvailabilityKindChallenge {
		fields["challenge_publisher_id"] = a.Challenge.PublisherID
		fields["challenge_game_id"] = a.Challenge.GameID
		fields["challenge_height"] = a.Challenge.Height
		fields["challenge_payload_index"] = a.Challenge.PayloadIndex
		fields["challenge_challenger"] = a.Challenge.Challenger
	} else {
		fields["proof_challenge_id"] = a.Proof.ChallengeID
	}

	return json.Marshal(fields)
}

// Sign signs the full transaction.
func (a *AvailabilityTx) Sign(privPEM []byte) (string, error) {
	payload, err := a.SigningBytes()
	if err != nil {
		return "", fmt.Errorf("error marshaling transaction: %w", err)
	}
	return signPayload(payload, privPEM)
}

// Verify verifies a signature over the full transaction.
func (a *AvailabilityTx) Verify(pubKey []byte, sign string) (bool, error) {
	payload, err := a.SigningBytes()
	if err != nil {
		return false, fmt.Errorf("error marshaling transaction: %w", err)
	}
	return verifyPayload(payload, pubKey, sign)
}

// Hash covers the availability fields as well as the base fields.
func (a *AvailabilityTx) Hash() string {
	a.Tx.hash = hashTransaction(a)
	return a.Tx.hash
}

// Bytes returns the canonical encoding.
func (a *AvailabilityTx) Bytes() []byte {
	payload, err := a.SigningBytes()
	if err != nil {
		return nil
	}
	return payload
}

// Size reports the size of the full transaction.
//
// The Merkle paths and the payload are counted, because they are what the block
// actually has to carry and what the fee should be priced against.
func (a *AvailabilityTx) Size() int {
	size := len(a.Bytes())
	if a.Kind == AvailabilityKindProof {
		size += len(a.Proof.Payload)
		for _, node := range a.Proof.HeaderPath {
			size += len(node)
		}
		for _, node := range a.Proof.PayloadPath {
			size += len(node)
		}
	}
	return size
}

// EstimateFee is derived from the full transaction size.
func (a *AvailabilityTx) EstimateFee(feePerByte float64) float64 {
	return float64(a.Size()) * feePerByte
}

// Send queues the transaction itself rather than its base transaction.
func (a *AvailabilityTx) Send(bc *Blockchain) error {
	if err := a.Validate(); err != nil {
		return fmt.Errorf("invalid transaction: %w", err)
	}
	bc.AddTransaction(a)
	return nil
}

// Validate checks the base transaction plus availability-specific invariants.
func (a *AvailabilityTx) Validate() error {
	if err := a.Tx.Validate(); err != nil {
		return err
	}
	if a.Tx.Protocol != AvailabilityProtocolID {
		return fmt.Errorf("availability transaction has wrong protocol: %s", a.Tx.Protocol)
	}

	switch a.Kind {
	case AvailabilityKindChallenge:
		return a.Challenge.Validate()

	case AvailabilityKindProof:
		if a.Proof.ChallengeID == "" {
			return fmt.Errorf("%w: proof names no challenge", ErrProofRejected)
		}
		if len(a.Proof.Header.PayloadRoot) != 32 {
			return fmt.Errorf("%w: proof header has no payload root", ErrProofRejected)
		}
		return nil

	default:
		return fmt.Errorf("%w: unknown availability kind %d", ErrChallengeInvalid, a.Kind)
	}
}

// GetID is nil-safe, matching the other protocol transactions.
func (a *AvailabilityTx) GetID() string {
	if a == nil || a.Tx.ID == nil {
		return ""
	}
	return a.Tx.GetID()
}
