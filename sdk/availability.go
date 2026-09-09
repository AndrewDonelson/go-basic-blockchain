// Package sdk is a software development kit for building blockchain applications.
// File sdk/availability.go - proving that anchored sidechain data still exists.
package sdk

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
)

// THE PROBLEM
//
// An anchor proves a history existed. It does not make the publisher hand it
// over. A publisher who withholds their blocks leaves everyone able to prove that
// something was committed and unable to say what -- so a player's achievement is
// committed, unforgeable, and unreadable.
//
// WHY NOT THE USUAL ANSWERS
//
// Erasure coding with data-availability sampling is what a general-purpose
// rollup chain reaches for, and it is the right answer when anyone may publish
// and no one is accountable. Here neither holds: every sidechain has exactly one
// writer, that writer is a registered publisher with a bond, and they are
// commercially motivated to serve their own game's data. Sampling would impose a
// network of samplers and an erasure-coding scheme on every node to solve a
// problem that identified, bonded writers already mostly solve.
//
// Requiring payloads on the main chain solves it completely and defeats the
// purpose: that is the cost sidechains exist to avoid.
//
// WHAT THIS DOES INSTEAD
//
// Challenge and response, backed by the registration bond.
//
//	1. Anyone may challenge a publisher to produce one payload at one height.
//	2. The publisher answers with an inclusion proof against the anchor that
//	   already commits that height -- O(log n), no matter how large the range.
//	3. An unanswered challenge expires and slashes the bond.
//
// The asymmetry is the point. Answering is trivial for a publisher who has their
// data and impossible for one who does not, and it costs the chain a few hundred
// bytes rather than the whole payload set. Honest operation is free: no challenge
// is ever issued against a publisher nobody suspects.
//
// WHAT IT GIVES AND WHAT IT DOES NOT
//
// One answered challenge proves one payload was available at one moment. It is
// sampling, not proof of total availability, and repeated challenges give
// confidence proportional to how many are issued. What it does convert is the
// incentive: withholding stops being free and starts being a bonded, detectable
// fault that anybody can trigger for the price of a fee.

const (
	// AvailabilityResponseWindow is how many main-chain blocks a publisher has to
	// answer a challenge.
	//
	// Long enough to survive an ordinary restart or a short outage, short enough
	// that withheld data is established as withheld while anyone still cares.
	AvailabilityResponseWindow int64 = 100

	// AvailabilityChallengeFeeUnits is what a challenger stakes to ask.
	//
	// A challenge is not free because answering one costs the publisher work, and
	// an unpriced challenge is a way to make a competitor do that work forever.
	// The stake returns to the challenger if the publisher fails, and goes to the
	// publisher if they answer -- so a well-founded challenge is close to free and
	// a frivolous one pays the person it inconvenienced.
	AvailabilityChallengeFeeUnits int64 = 1 * UnitsPerToken

	// AvailabilitySlashUnits is taken from the bond when a challenge expires.
	AvailabilitySlashUnits int64 = 10 * UnitsPerToken
)

var (
	// ErrChallengeInvalid is returned for a malformed or unanswerable challenge.
	ErrChallengeInvalid = errors.New("invalid availability challenge")

	// ErrChallengeUnknown is returned when a response names no open challenge.
	ErrChallengeUnknown = errors.New("no such open availability challenge")

	// ErrProofRejected is returned when a response does not prove what it claims.
	ErrProofRejected = errors.New("availability proof rejected")
)

// AvailabilityChallenge asks a publisher to produce one payload.
//
// A single payload rather than a whole block: it is the smallest unit that
// distinguishes "has the data" from "does not", and keeping the answer small is
// what keeps the mechanism affordable enough to actually use.
type AvailabilityChallenge struct {
	PublisherID uint64
	GameID      uint64
	// Height is the sidechain height being challenged. It must already be
	// anchored -- challenging unanchored history would be asking for something
	// the publisher never committed to.
	Height uint64
	// PayloadIndex selects the payload inside that block.
	PayloadIndex uint32
	// Challenger is the address that staked the fee and is repaid on default.
	Challenger string
}

// SidechainID returns the sidechain being challenged.
func (c AvailabilityChallenge) SidechainID() SidechainID {
	return SidechainID{PublisherID: c.PublisherID, GameID: c.GameID}
}

// ID is the challenge's identity, derived from its contents.
//
// Deriving it rather than assigning one means two identical challenges collide by
// construction, so a publisher cannot be made to answer the same question twice
// in parallel and a challenger cannot inflate the appearance of doubt by
// resubmitting.
func (c AvailabilityChallenge) ID() string {
	var buf bytes.Buffer
	buf.WriteString("gbb/availability/challenge/v1")

	var tmp [8]byte
	for _, v := range []uint64{c.PublisherID, c.GameID, c.Height, uint64(c.PayloadIndex)} {
		binary.BigEndian.PutUint64(tmp[:], v)
		buf.Write(tmp[:])
	}

	sum := sha256.Sum256(buf.Bytes())
	return fmt.Sprintf("%x", sum[:])
}

// Validate checks a challenge's shape.
func (c AvailabilityChallenge) Validate() error {
	if err := c.SidechainID().Validate(); err != nil {
		return err
	}
	if c.Challenger == "" {
		return fmt.Errorf("%w: challenge has no challenger", ErrChallengeInvalid)
	}
	return nil
}

// AvailabilityProof answers a challenge.
//
// It carries two inclusion proofs, and the pair is what makes the answer both
// small and binding: the header proof ties the block to the anchor already on
// the main chain, and the payload proof ties the payload to that block. Either
// one alone would prove nothing about anchored history.
type AvailabilityProof struct {
	ChallengeID string

	// Header is the challenged block's header, from which the block hash is
	// recomputed rather than taken on trust.
	Header SidechainBlockHeader
	// HeaderPath proves Header's hash sits in the anchor's HeaderRoot.
	HeaderPath [][]byte

	// Payload is the data being produced -- the whole point of the exchange.
	Payload []byte
	// PayloadPath proves Payload sits in Header.PayloadRoot.
	PayloadPath [][]byte
}

// openChallenge is a challenge awaiting an answer.
type openChallenge struct {
	Challenge AvailabilityChallenge
	// Deadline is the main-chain height after which the challenge expires.
	Deadline int64
	// OpenedAt is kept so a rollback can tell which challenges this block opened.
	OpenedAt int64
}

// AvailabilityLedger tracks open challenges and their outcomes.
type AvailabilityLedger struct {
	mu sync.RWMutex

	anchors *AnchorLedger
	open    map[string]*openChallenge
	// failures counts expired challenges per sidechain. A publisher who has
	// failed is not automatically evicted -- that is a policy decision for whoever
	// consumes this -- but the count is on the chain and cannot be argued with.
	failures map[string]int
}

// NewAvailabilityLedger creates a ledger over an anchor ledger.
func NewAvailabilityLedger(anchors *AnchorLedger) *AvailabilityLedger {
	return &AvailabilityLedger{
		anchors:  anchors,
		open:     map[string]*openChallenge{},
		failures: map[string]int{},
	}
}

// Open returns an open challenge.
func (l *AvailabilityLedger) Open(challengeID string) (AvailabilityChallenge, int64, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	entry, ok := l.open[challengeID]
	if !ok {
		return AvailabilityChallenge{}, 0, false
	}
	return entry.Challenge, entry.Deadline, true
}

// OpenCount returns how many challenges are awaiting an answer.
func (l *AvailabilityLedger) OpenCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.open)
}

// Failures returns how many challenges a sidechain has failed to answer.
func (l *AvailabilityLedger) Failures(id SidechainID) int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return l.failures[id.String()]
}

// checkChallengeable verifies a challenge names anchored history.
func (l *AvailabilityLedger) checkChallengeable(challenge AvailabilityChallenge) error {
	if err := challenge.Validate(); err != nil {
		return err
	}

	// The height must already be anchored. Challenging unanchored history would
	// be demanding a publisher produce something they have not committed to, and
	// there would be nothing to check the answer against.
	anchor, ok := l.anchors.AnchorCovering(challenge.SidechainID(), challenge.Height)
	if !ok {
		return fmt.Errorf("%w: sidechain %s has not anchored height %d",
			ErrChallengeInvalid, challenge.SidechainID(), challenge.Height)
	}
	_ = anchor

	return nil
}

// VerifyProof checks that a proof answers its challenge.
func (l *AvailabilityLedger) VerifyProof(proof AvailabilityProof) error {
	l.mu.RLock()
	entry, ok := l.open[proof.ChallengeID]
	l.mu.RUnlock()
	if !ok {
		return fmt.Errorf("%w: %s", ErrChallengeUnknown, proof.ChallengeID)
	}

	challenge := entry.Challenge
	id := challenge.SidechainID()

	// The header must be the one that was asked about. Checking this before the
	// Merkle work means a publisher cannot answer a question about height 7 with
	// a perfectly valid proof concerning height 8.
	if proof.Header.Height != challenge.Height {
		return fmt.Errorf("%w: proof is for height %d, the challenge asked for %d",
			ErrProofRejected, proof.Header.Height, challenge.Height)
	}
	if proof.Header.PublisherID != id.PublisherID || proof.Header.GameID != id.GameID {
		return fmt.Errorf("%w: proof is for sidechain %d:%d, the challenge asked about %s",
			ErrProofRejected, proof.Header.PublisherID, proof.Header.GameID, id)
	}

	anchor, ok := l.anchors.AnchorCovering(id, challenge.Height)
	if !ok {
		return fmt.Errorf("%w: no anchor covers height %d", ErrProofRejected, challenge.Height)
	}

	// The block hash is recomputed from the header rather than supplied, so a
	// respondent cannot present a hash that is in the anchor alongside header
	// fields that are not.
	block := &SidechainBlock{Header: proof.Header}
	blockHash := block.ComputeHash()

	headerIndex, headerCount, ok := anchor.offsetOf(challenge.Height)
	if !ok {
		return fmt.Errorf("%w: height %d is outside the anchored range %d..%d",
			ErrProofRejected, challenge.Height, anchor.FromHeight, anchor.ToHeight)
	}
	if !merkleVerify(sidechainHeaderDomain, anchor.HeaderRoot, blockHash,
		headerIndex, headerCount, proof.HeaderPath) {
		return fmt.Errorf("%w: the block does not sit in the anchored header root; "+
			"this is not the history that was anchored", ErrProofRejected)
	}

	if challenge.PayloadIndex >= proof.Header.PayloadCount {
		return fmt.Errorf("%w: payload %d is outside a block of %d",
			ErrProofRejected, challenge.PayloadIndex, proof.Header.PayloadCount)
	}
	if !VerifyPayloadInclusion(proof.Header.PayloadRoot, proof.Payload,
		int(challenge.PayloadIndex), int(proof.Header.PayloadCount), proof.PayloadPath) {
		return fmt.Errorf("%w: the payload does not sit in the block's payload root",
			ErrProofRejected)
	}

	return nil
}

// BuildProof answers a challenge from a sidechain the publisher holds.
func BuildProof(challenge AvailabilityChallenge, chain *Sidechain, anchor SidechainAnchor) (*AvailabilityProof, error) {
	if chain == nil {
		return nil, fmt.Errorf("%w: chain is nil", ErrProofRejected)
	}

	block, ok := chain.BlockAt(challenge.Height)
	if !ok {
		return nil, fmt.Errorf("%w: the chain has no block at height %d",
			ErrProofRejected, challenge.Height)
	}

	payloadPath, err := block.PayloadProof(int(challenge.PayloadIndex))
	if err != nil {
		return nil, err
	}

	hashes := make([][]byte, 0, anchor.ToHeight-anchor.FromHeight+1)
	for height := anchor.FromHeight; height <= anchor.ToHeight; height++ {
		ranged, ok := chain.BlockAt(height)
		if !ok {
			return nil, fmt.Errorf("%w: the chain has a gap at height %d",
				ErrProofRejected, height)
		}
		hashes = append(hashes, ranged.Hash)
	}

	headerIndex, _, ok := anchor.offsetOf(challenge.Height)
	if !ok {
		return nil, fmt.Errorf("%w: height %d is outside the anchored range %d..%d",
			ErrProofRejected, challenge.Height, anchor.FromHeight, anchor.ToHeight)
	}
	headerPath, err := merkleProof(sidechainHeaderDomain, hashes, headerIndex)
	if err != nil {
		return nil, err
	}

	return &AvailabilityProof{
		ChallengeID: challenge.ID(),
		Header:      block.Header,
		HeaderPath:  headerPath,
		Payload:     append([]byte{}, block.Payloads[challenge.PayloadIndex]...),
		PayloadPath: payloadPath,
	}, nil
}

// availabilityRestore captures one challenge's state before a block changed it.
//
// One shape covers opening, answering and expiry, because all three are the same
// change viewed differently: whether the challenge is open, and what the
// sidechain's failure count was.
type availabilityRestore struct {
	ChallengeID   string
	WasOpen       bool
	Prior         openChallenge
	SidechainKey  string
	PriorFailures int
}

// DeriveChallengeEscrowAddress holds a challenger's stake while a challenge is
// open.
//
// Derived from a domain string, so no private key exists and neither party can
// take the stake back unilaterally -- only the consensus rules resolve it.
func DeriveChallengeEscrowAddress(challengeID string) string {
	sum := sha256.Sum256([]byte("gbb/availability/escrow/v1/" + challengeID))
	return fmt.Sprintf("%x", sum[:])
}

// openForUndo records a new challenge.
func (l *AvailabilityLedger) openForUndo(challenge AvailabilityChallenge, mainHeight int64) (availabilityRestore, error) {
	if err := l.checkChallengeable(challenge); err != nil {
		return availabilityRestore{}, err
	}

	id := challenge.ID()
	key := challenge.SidechainID().String()

	l.mu.Lock()
	defer l.mu.Unlock()

	prior, wasOpen := l.open[id]
	if wasOpen {
		return availabilityRestore{}, fmt.Errorf("%w: challenge %s is already open",
			ErrChallengeInvalid, id)
	}

	restore := availabilityRestore{
		ChallengeID:   id,
		WasOpen:       false,
		SidechainKey:  key,
		PriorFailures: l.failures[key],
	}
	_ = prior

	l.open[id] = &openChallenge{
		Challenge: challenge,
		Deadline:  mainHeight + AvailabilityResponseWindow,
		OpenedAt:  mainHeight,
	}
	return restore, nil
}

// answerForUndo closes a challenge that has been proved.
func (l *AvailabilityLedger) answerForUndo(proof AvailabilityProof) (availabilityRestore, AvailabilityChallenge, error) {
	if err := l.VerifyProof(proof); err != nil {
		return availabilityRestore{}, AvailabilityChallenge{}, err
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.open[proof.ChallengeID]
	if !ok {
		return availabilityRestore{}, AvailabilityChallenge{}, fmt.Errorf("%w: %s",
			ErrChallengeUnknown, proof.ChallengeID)
	}

	key := entry.Challenge.SidechainID().String()
	restore := availabilityRestore{
		ChallengeID:   proof.ChallengeID,
		WasOpen:       true,
		Prior:         *entry,
		SidechainKey:  key,
		PriorFailures: l.failures[key],
	}

	delete(l.open, proof.ChallengeID)
	return restore, entry.Challenge, nil
}

// dueChallenges returns the challenges whose deadline has passed.
func (l *AvailabilityLedger) dueChallenges(mainHeight int64) []AvailabilityChallenge {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var due []AvailabilityChallenge
	for _, entry := range l.open {
		if mainHeight > entry.Deadline {
			due = append(due, entry.Challenge)
		}
	}
	return due
}

// expireForUndo closes a challenge that was never answered.
func (l *AvailabilityLedger) expireForUndo(challengeID string) (availabilityRestore, AvailabilityChallenge, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	entry, ok := l.open[challengeID]
	if !ok {
		return availabilityRestore{}, AvailabilityChallenge{}, fmt.Errorf("%w: %s",
			ErrChallengeUnknown, challengeID)
	}

	key := entry.Challenge.SidechainID().String()
	restore := availabilityRestore{
		ChallengeID:   challengeID,
		WasOpen:       true,
		Prior:         *entry,
		SidechainKey:  key,
		PriorFailures: l.failures[key],
	}

	delete(l.open, challengeID)
	l.failures[key]++

	return restore, entry.Challenge, nil
}

// restore puts one challenge back to its state before a block was applied.
func (l *AvailabilityLedger) restore(r availabilityRestore) {
	l.mu.Lock()
	defer l.mu.Unlock()

	if r.WasOpen {
		prior := r.Prior
		l.open[r.ChallengeID] = &prior
	} else {
		delete(l.open, r.ChallengeID)
	}

	if r.PriorFailures == 0 {
		delete(l.failures, r.SidechainKey)
		return
	}
	l.failures[r.SidechainKey] = r.PriorFailures
}
