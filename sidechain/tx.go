// File sidechain/tx.go - REGISTER and ANCHOR transactions, built without the node.
package sidechain

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"time"
)

// An application talks to the main chain with two transactions: REGISTER, to
// claim a publisher id and then a game id for each sidechain, and ANCHOR, to
// commit a sidechain range. This file builds and signs both in exactly the
// canonical wire form the node decodes (sdk.DecodeTransaction) and verifies
// (sdk.VerifyTransactionAuthority), so an application never links the node.
// TestLiteTransactionsAreNodeTransactions in package sdk keeps the two in step.

// Protocols and registration kinds, as the node names them.
const (
	ProtocolAnchor   = "ANCHOR"
	ProtocolRegister = "REGISTER"

	RegistrationKindPublisher uint32 = 1
	RegistrationKindGame      uint32 = 2

	// TransactionVersion is the transaction structure version the node accepts.
	TransactionVersion = 1

	// DefaultFee is the node's default transaction fee, in coins.
	DefaultFee = 0.05
)

// ErrInvalidTransaction is returned when a transaction can't be built.
var ErrInvalidTransaction = errors.New("invalid transaction")

// Transaction is a signed REGISTER or ANCHOR transaction ready to submit
// (Client.Submit). Its JSON encoding is the node's wire form.
type Transaction struct {
	id        string
	time      time.Time
	protocol  string
	fee       float64
	nonce     uint64
	from      string
	fromKey   string
	signature string

	anchor          *Anchor
	anchorSignature []byte

	kind            uint32
	publisherID     uint64
	name            string
	registrationKey string
	bondUnits       int64
	registrationSig []byte
}

// TxOptions are the sender-side fields every transaction carries.
type TxOptions struct {
	// Nonce must be greater than every nonce the sender has used
	// (AccountInfo.NextNonce).
	Nonce uint64
	// Fee in coins; zero means none. DefaultFee is the node's default.
	Fee float64
	// Time stamps the transaction; zero means now.
	Time time.Time
}

// NewAnchorTransaction builds an ANCHOR transaction committing anchor, signed by
// the publisher (anchorSignature, from SignAnchor) and paid for by wallet.
func NewAnchorTransaction(wallet *Key, anchor Anchor, anchorSignature []byte, opts TxOptions) (*Transaction, error) {
	if err := anchor.Validate(); err != nil {
		return nil, err
	}
	if len(anchorSignature) == 0 {
		return nil, fmt.Errorf("%w: anchor is unsigned", ErrAnchorUnauthorised)
	}
	a := anchor
	tx, err := newTransaction(ProtocolAnchor, wallet, opts)
	if err != nil {
		return nil, err
	}
	tx.anchor = &a
	tx.anchorSignature = append([]byte{}, anchorSignature...)
	return tx, tx.sign(wallet)
}

// NewPublisherRegistration builds a REGISTER transaction claiming a publisher
// id for identity (the key that will sign the publisher's anchors), locking
// bondUnits of wallet's balance as the publisher's bond.
func NewPublisherRegistration(wallet, identity *Key, name string, bondUnits int64, opts TxOptions) (*Transaction, error) {
	if identity == nil {
		return nil, fmt.Errorf("%w: no publisher identity", ErrInvalidTransaction)
	}
	tx, err := newTransaction(ProtocolRegister, wallet, opts)
	if err != nil {
		return nil, err
	}
	tx.kind, tx.name, tx.registrationKey, tx.bondUnits = RegistrationKindPublisher, name, identity.PublicPEM(), bondUnits
	if tx.registrationSig, err = identity.Sign(tx.registrationBytes()); err != nil {
		return nil, err
	}
	return tx, tx.sign(wallet)
}

// NewGameRegistration builds a REGISTER transaction claiming the next game id
// under publisherID, signed by the publisher's registered identity.
func NewGameRegistration(wallet, identity *Key, publisherID uint64, name string, opts TxOptions) (*Transaction, error) {
	if identity == nil {
		return nil, fmt.Errorf("%w: no publisher identity", ErrInvalidTransaction)
	}
	if publisherID == 0 {
		return nil, fmt.Errorf("%w: publisher id must not be zero", ErrInvalidTransaction)
	}
	tx, err := newTransaction(ProtocolRegister, wallet, opts)
	if err != nil {
		return nil, err
	}
	tx.kind, tx.publisherID, tx.name = RegistrationKindGame, publisherID, name
	if tx.registrationSig, err = identity.Sign(tx.registrationBytes()); err != nil {
		return nil, err
	}
	return tx, tx.sign(wallet)
}

func newTransaction(protocol string, wallet *Key, opts TxOptions) (*Transaction, error) {
	if wallet == nil {
		return nil, fmt.Errorf("%w: no wallet", ErrInvalidTransaction)
	}
	if opts.Fee < 0 {
		return nil, fmt.Errorf("%w: negative fee", ErrInvalidTransaction)
	}
	// A transaction id is a PUID "organization:app:user:asset"; the node only
	// needs it to be unique, so the asset part is random.
	n, err := rand.Int(rand.Reader, big.NewInt(1<<62))
	if err != nil {
		return nil, err
	}
	at := opts.Time
	if at.IsZero() {
		at = time.Now()
	}
	return &Transaction{
		id:       fmt.Sprintf("0:0:0:%d", n.Int64()+1),
		time:     at.UTC().Round(0),
		protocol: protocol,
		fee:      opts.Fee,
		nonce:    opts.Nonce,
		from:     wallet.Address(),
		fromKey:  wallet.PublicPEM(),
	}, nil
}

// ID is the transaction's id, which the node reports it under.
func (t *Transaction) ID() string { return t.id }

// Protocol is ProtocolAnchor or ProtocolRegister.
func (t *Transaction) Protocol() string { return t.protocol }

// Nonce is the sender's sequence number this transaction uses.
func (t *Transaction) Nonce() uint64 { return t.nonce }

// registrationBytes is what a registration signature covers (the node's
// RegistrationTx.registrationBytes).
func (t *Transaction) registrationBytes() []byte {
	var buf bytes.Buffer
	buf.WriteString("gbb/registration/v1")
	var tmp [8]byte
	binary.BigEndian.PutUint32(tmp[:4], t.kind)
	buf.Write(tmp[:4])
	binary.BigEndian.PutUint64(tmp[:], t.publisherID)
	buf.Write(tmp[:])
	binary.BigEndian.PutUint64(tmp[:], uint64(t.bondUnits)) //nolint:gosec // bit reinterpretation, as the node does
	buf.Write(tmp[:])
	for _, field := range []string{t.name, t.registrationKey} {
		binary.BigEndian.PutUint64(tmp[:], uint64(len(field)))
		buf.Write(tmp[:])
		buf.WriteString(field)
	}
	return buf.Bytes()
}

// SigningBytes is the canonical payload the wallet signs (the node's
// Tx.signingFields plus the protocol's own fields). json.Marshal sorts map keys.
func (t *Transaction) SigningBytes() ([]byte, error) {
	fields := map[string]interface{}{
		"version":  TransactionVersion,
		"protocol": t.protocol,
		"fee":      t.fee,
		"nonce":    t.nonce,
		"time":     t.time.UnixNano(),
		"data":     []byte(nil),
		"id":       t.id,
		"from":     t.from,
		"to":       t.from,
	}
	switch t.protocol {
	case ProtocolAnchor:
		fields["anchor_publisher_id"] = t.anchor.PublisherID
		fields["anchor_game_id"] = t.anchor.GameID
		fields["anchor_from_height"] = t.anchor.FromHeight
		fields["anchor_to_height"] = t.anchor.ToHeight
		fields["anchor_tip_hash"] = hex.EncodeToString(t.anchor.TipHash)
		fields["anchor_header_root"] = hex.EncodeToString(t.anchor.HeaderRoot)
		fields["anchor_payload_count"] = t.anchor.PayloadCount
	case ProtocolRegister:
		fields["registration_kind"] = t.kind
		fields["registration_publisher_id"] = t.publisherID
		fields["registration_name"] = t.name
		fields["registration_key"] = t.registrationKey
		fields["registration_bond_units"] = t.bondUnits
	}
	return json.Marshal(fields)
}

func (t *Transaction) sign(wallet *Key) error {
	payload, err := t.SigningBytes()
	if err != nil {
		return err
	}
	sig, err := wallet.Sign(payload)
	if err != nil {
		return err
	}
	t.signature = base64.StdEncoding.EncodeToString(sig)
	return nil
}

// txWire is the subset of the node's transaction wire form these protocols use.
type txWire struct {
	ID        string    `json:"id"`
	Time      time.Time `json:"time"`
	Version   int       `json:"version"`
	Protocol  string    `json:"protocol"`
	From      string    `json:"from"`
	To        string    `json:"to"`
	FromPub   string    `json:"from_public_key,omitempty"`
	ToPub     string    `json:"to_public_key,omitempty"`
	Fee       float64   `json:"fee"`
	Status    string    `json:"status"`
	BlockNum  int       `json:"block_num"`
	Signature string    `json:"signature"`
	Nonce     uint64    `json:"nonce"`

	PublisherID     uint64 `json:"publisher_id,omitempty"`
	GameID          uint64 `json:"game_id,omitempty"`
	FromHeight      uint64 `json:"from_height,omitempty"`
	ToHeight        uint64 `json:"to_height,omitempty"`
	TipHash         string `json:"tip_hash,omitempty"`
	HeaderRoot      string `json:"header_root,omitempty"`
	AnchorPayloads  uint64 `json:"anchor_payload_count,omitempty"`
	AnchorSignature string `json:"anchor_signature,omitempty"`

	RegistrationKind      uint32 `json:"registration_kind,omitempty"`
	RegistrationName      string `json:"registration_name,omitempty"`
	RegistrationKey       string `json:"registration_key,omitempty"`
	BondUnits             int64  `json:"bond_units,omitempty"`
	RegistrationSignature string `json:"registration_signature,omitempty"`
}

// MarshalJSON encodes the transaction in the node's wire form.
func (t *Transaction) MarshalJSON() ([]byte, error) {
	w := txWire{
		ID: t.id, Time: t.time, Version: TransactionVersion, Protocol: t.protocol,
		From: t.from, To: t.from, FromPub: t.fromKey, ToPub: t.fromKey,
		Fee: t.fee, Status: "pending", Signature: t.signature, Nonce: t.nonce,
	}
	switch t.protocol {
	case ProtocolAnchor:
		w.PublisherID, w.GameID = t.anchor.PublisherID, t.anchor.GameID
		w.FromHeight, w.ToHeight = t.anchor.FromHeight, t.anchor.ToHeight
		w.TipHash, w.HeaderRoot = hex.EncodeToString(t.anchor.TipHash), hex.EncodeToString(t.anchor.HeaderRoot)
		w.AnchorPayloads = t.anchor.PayloadCount
		w.AnchorSignature = hex.EncodeToString(t.anchorSignature)
	case ProtocolRegister:
		w.RegistrationKind, w.PublisherID = t.kind, t.publisherID
		w.RegistrationName, w.RegistrationKey, w.BondUnits = t.name, t.registrationKey, t.bondUnits
		w.RegistrationSignature = hex.EncodeToString(t.registrationSig)
	}
	return json.Marshal(w)
}
