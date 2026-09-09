// Package sdk is a software development kit for building blockchain applications.
// File sdk/registrationtx.go - the transaction that registers publishers and games.
package sdk

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
)

// Registration kinds.
const (
	// RegistrationKindPublisher claims a new publisher id.
	RegistrationKindPublisher uint32 = 1
	// RegistrationKindGame claims a new game id inside an existing publisher.
	RegistrationKindGame uint32 = 2
)

// RegistrationTx registers a publisher or one of their games.
//
// Registration is a main-chain transaction rather than a local call because the
// id it allocates has to mean the same thing on every node. A registry each node
// filled in for itself would let two nodes disagree about who owns publisher 7,
// and an anchor accepted by one would be refused by the other.
type RegistrationTx struct {
	Tx

	Kind uint32
	// PublisherID names the publisher a game is being registered under. It is
	// unset for a publisher registration, where the id is what gets allocated.
	PublisherID uint64
	Name        string

	// PublicKeyPEM is the key that will sign this publisher's anchors.
	PublicKeyPEM string
	// BondUnits is locked at registration and is what availability failures are
	// paid out of.
	BondUnits int64

	// RegistrationSignature proves the right to make this registration.
	//
	// For a publisher it is made by the key being registered, which proves
	// possession of the matching private key -- otherwise anyone could register
	// a key they had merely seen, burning an id and creating an identity whose
	// owner never asked for it.
	//
	// For a game it is made by the owning publisher's registered key, which is
	// what stops one publisher registering games inside another's namespace.
	RegistrationSignature []byte
}

// NewPublisherRegistration builds a publisher registration signed by the key
// being registered.
func NewPublisherRegistration(from *Wallet, identity *PeerIdentity, name string, bondUnits int64) (*RegistrationTx, error) {
	if identity == nil {
		return nil, fmt.Errorf("%w: no publisher identity", ErrRegistrationInvalid)
	}

	tx, err := NewTransaction(RegisterProtocolID, from, from)
	if err != nil {
		return nil, err
	}

	registration := &RegistrationTx{
		Tx:           *tx,
		Kind:         RegistrationKindPublisher,
		Name:         name,
		PublicKeyPEM: identity.PublicPEM,
		BondUnits:    bondUnits,
	}
	if err := registration.checkShape(); err != nil {
		return nil, err
	}

	signature, err := identity.Sign(registration.registrationBytes())
	if err != nil {
		return nil, err
	}
	registration.RegistrationSignature = signature

	return registration, nil
}

// NewGameRegistration builds a game registration signed by the owning publisher.
func NewGameRegistration(from *Wallet, identity *PeerIdentity, publisherID uint64, name string) (*RegistrationTx, error) {
	if identity == nil {
		return nil, fmt.Errorf("%w: no publisher identity", ErrRegistrationInvalid)
	}

	tx, err := NewTransaction(RegisterProtocolID, from, from)
	if err != nil {
		return nil, err
	}

	registration := &RegistrationTx{
		Tx:          *tx,
		Kind:        RegistrationKindGame,
		PublisherID: publisherID,
		Name:        name,
	}
	if err := registration.checkShape(); err != nil {
		return nil, err
	}

	signature, err := identity.Sign(registration.registrationBytes())
	if err != nil {
		return nil, err
	}
	registration.RegistrationSignature = signature

	return registration, nil
}

// registrationBytes is the canonical payload the registration signature covers.
//
// Every field the registration asserts is included. A signature that did not
// cover the bond could be lifted onto a registration offering less; one that did
// not cover the kind could turn a game registration into a publisher one.
func (r *RegistrationTx) registrationBytes() []byte {
	var buf bytes.Buffer
	buf.WriteString("gbb/registration/v1")

	var tmp [8]byte
	binary.BigEndian.PutUint32(tmp[:4], r.Kind)
	buf.Write(tmp[:4])
	binary.BigEndian.PutUint64(tmp[:], r.PublisherID)
	buf.Write(tmp[:])
	// Reinterpreting the signed bond's bits, not converting its value: the
	// mapping is injective either way, which is all a signature preimage needs.
	binary.BigEndian.PutUint64(tmp[:], uint64(r.BondUnits)) //nolint:gosec // bit reinterpretation
	buf.Write(tmp[:])

	for _, field := range []string{r.Name, r.PublicKeyPEM} {
		binary.BigEndian.PutUint64(tmp[:], uint64(len(field)))
		buf.Write(tmp[:])
		buf.WriteString(field)
	}

	return buf.Bytes()
}

// checkShape validates the fields a registration of this kind must and must not
// carry.
func (r *RegistrationTx) checkShape() error {
	if err := validateName(r.Name); err != nil {
		return err
	}

	switch r.Kind {
	case RegistrationKindPublisher:
		if r.PublisherID != 0 {
			return fmt.Errorf("%w: a publisher registration cannot name its own id; "+
				"the id is allocated", ErrRegistrationInvalid)
		}
		if _, err := parsePeerPublicKey(r.PublicKeyPEM); err != nil {
			return fmt.Errorf("%w: %w", ErrRegistrationInvalid, err)
		}
		if r.BondUnits < MinPublisherBondUnits {
			return fmt.Errorf("%w: bond of %d units is below the %d minimum",
				ErrRegistrationInvalid, r.BondUnits, MinPublisherBondUnits)
		}

	case RegistrationKindGame:
		if r.PublisherID == 0 {
			return fmt.Errorf("%w: a game registration must name its publisher",
				ErrRegistrationInvalid)
		}
		if r.PublicKeyPEM != "" || r.BondUnits != 0 {
			return fmt.Errorf("%w: a game registration carries no key and no bond",
				ErrRegistrationInvalid)
		}

	default:
		return fmt.Errorf("%w: unknown registration kind %d", ErrRegistrationInvalid, r.Kind)
	}

	return nil
}

// MarshalJSON encodes the registration in the canonical wire form.
func (r *RegistrationTx) MarshalJSON() ([]byte, error) {
	w := r.Tx.toWire()
	w.RegistrationKind = r.Kind
	w.PublisherID = r.PublisherID
	w.RegistrationName = r.Name
	w.RegistrationKey = r.PublicKeyPEM
	w.BondUnits = r.BondUnits
	w.RegistrationSignature = hex.EncodeToString(r.RegistrationSignature)
	return json.Marshal(w)
}

// UnmarshalJSON decodes a registration from the canonical wire form.
func (r *RegistrationTx) UnmarshalJSON(data []byte) error {
	var w txWire
	if err := json.Unmarshal(data, &w); err != nil {
		return err
	}
	r.Tx.applyWire(w)

	registration, err := registrationFromWire(w)
	if err != nil {
		return err
	}
	r.Kind = registration.Kind
	r.PublisherID = registration.PublisherID
	r.Name = registration.Name
	r.PublicKeyPEM = registration.PublicKeyPEM
	r.BondUnits = registration.BondUnits
	r.RegistrationSignature = registration.RegistrationSignature
	return nil
}

// registrationFromWire rebuilds the registration fields from the wire shape.
//
// Shared by UnmarshalJSON and DecodeTransaction so the two decode paths cannot
// drift, which is how the coinbase subsidy once decoded as zero on reload.
func registrationFromWire(w txWire) (*RegistrationTx, error) {
	signature, err := hex.DecodeString(w.RegistrationSignature)
	if err != nil {
		return nil, fmt.Errorf("%w: registration signature is not valid hex: %w",
			ErrRegistrationInvalid, err)
	}

	return &RegistrationTx{
		Kind:                  w.RegistrationKind,
		PublisherID:           w.PublisherID,
		Name:                  w.RegistrationName,
		PublicKeyPEM:          w.RegistrationKey,
		BondUnits:             w.BondUnits,
		RegistrationSignature: signature,
	}, nil
}

// VerifyRegistrationSignature checks the registration signature against the key
// entitled to make it.
func (r *RegistrationTx) VerifyRegistrationSignature(registry *PublisherRegistry) error {
	if len(r.RegistrationSignature) == 0 {
		return fmt.Errorf("%w: registration is unsigned", ErrRegistrationInvalid)
	}

	var publicKey string
	switch r.Kind {
	case RegistrationKindPublisher:
		// Proof of possession: signed by the very key being claimed.
		publicKey = r.PublicKeyPEM

	case RegistrationKindGame:
		key, ok := registry.PublicKey(r.PublisherID)
		if !ok {
			return fmt.Errorf("%w: %d", ErrPublisherNotRegistered, r.PublisherID)
		}
		publicKey = key

	default:
		return fmt.Errorf("%w: unknown registration kind %d", ErrRegistrationInvalid, r.Kind)
	}

	if err := VerifyPeerSignature(publicKey, r.registrationBytes(), r.RegistrationSignature); err != nil {
		return fmt.Errorf("%w: %w", ErrRegistrationInvalid, err)
	}
	return nil
}

// Process describes the registration.
func (r *RegistrationTx) Process() string {
	r.Status = StatusConfirmed
	if r.Kind == RegistrationKindPublisher {
		return fmt.Sprintf("Registering publisher %q with a bond of %d units", r.Name, r.BondUnits)
	}
	return fmt.Sprintf("Registering game %q under publisher %d", r.Name, r.PublisherID)
}

// SigningBytes covers the registration as well as the base fields.
func (r *RegistrationTx) SigningBytes() ([]byte, error) {
	fields := r.Tx.signingFields()
	fields["registration_kind"] = r.Kind
	fields["registration_publisher_id"] = r.PublisherID
	fields["registration_name"] = r.Name
	fields["registration_key"] = r.PublicKeyPEM
	fields["registration_bond_units"] = r.BondUnits
	return json.Marshal(fields)
}

// Sign signs the full registration transaction.
func (r *RegistrationTx) Sign(privPEM []byte) (string, error) {
	payload, err := r.SigningBytes()
	if err != nil {
		return "", fmt.Errorf("error marshaling transaction: %w", err)
	}
	return signPayload(payload, privPEM)
}

// Verify verifies a signature over the full registration transaction.
func (r *RegistrationTx) Verify(pubKey []byte, sign string) (bool, error) {
	payload, err := r.SigningBytes()
	if err != nil {
		return false, fmt.Errorf("error marshaling transaction: %w", err)
	}
	return verifyPayload(payload, pubKey, sign)
}

// Hash covers the registration as well as the base fields.
func (r *RegistrationTx) Hash() string {
	r.Tx.hash = hashTransaction(r)
	return r.Tx.hash
}

// Bytes returns the canonical encoding.
func (r *RegistrationTx) Bytes() []byte {
	payload, err := r.SigningBytes()
	if err != nil {
		return nil
	}
	return payload
}

// Size reports the size of the full transaction.
func (r *RegistrationTx) Size() int { return len(r.Bytes()) }

// EstimateFee is derived from the full transaction size.
func (r *RegistrationTx) EstimateFee(feePerByte float64) float64 {
	return float64(r.Size()) * feePerByte
}

// Send queues the registration itself rather than its base transaction.
func (r *RegistrationTx) Send(bc *Blockchain) error {
	if err := r.Validate(); err != nil {
		return fmt.Errorf("invalid transaction: %w", err)
	}
	bc.AddTransaction(r)
	return nil
}

// Validate checks the base transaction plus registration-specific invariants.
//
// It does not check whether the publisher exists or the key is already taken:
// those are questions about chain state, and they are answered where that state
// lives, when the block is applied.
func (r *RegistrationTx) Validate() error {
	if err := r.Tx.Validate(); err != nil {
		return err
	}
	if r.Tx.Protocol != RegisterProtocolID {
		return fmt.Errorf("registration has wrong protocol: %s", r.Tx.Protocol)
	}
	if err := r.checkShape(); err != nil {
		return err
	}
	if len(r.RegistrationSignature) == 0 {
		return fmt.Errorf("%w: registration is unsigned", ErrRegistrationInvalid)
	}
	return nil
}

// GetID is nil-safe, matching the other protocol transactions.
func (r *RegistrationTx) GetID() string {
	if r == nil || r.Tx.ID == nil {
		return ""
	}
	return r.Tx.GetID()
}
