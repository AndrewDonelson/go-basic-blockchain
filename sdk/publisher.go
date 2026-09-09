// Package sdk is a software development kit for building blockchain applications.
// File sdk/publisher.go - Publisher and game registration as consensus state.
package sdk

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
)

// WHY IDS ARE ALLOCATED RATHER THAN CHOSEN
//
// A publisher id names a block space, and a game id names one inside it. If a
// registrant picked their own, the ids would be squattable: register publisher 1
// and every game id under it before anyone else, and sell them back. The same
// argument that makes a domain registry adversarial applies here.
//
// So ids are allocated by consensus, in the order registrations are applied:
// publisher ids from a chain-wide counter, game ids from a counter inside each
// publisher. Both start at 1, because zero is the value an omitted protobuf field
// decodes to and must never name anything real.
//
// The consequence is that a registrant does not know their id until the
// registration confirms. That is the normal shape of an allocation and it is the
// price of not having a land grab.

const (
	// MaxPublisherNameLength bounds the metadata a registration may carry.
	MaxPublisherNameLength = 128

	// MinPublisherBondUnits is what a publisher must lock to register.
	//
	// The bond is not a fee -- it is returned when nothing goes wrong. It exists
	// so that the availability obligations in docs/sidechains.md have something
	// behind them: a publisher who cannot produce data they committed to has
	// something to lose, and without that an availability rule is a request.
	MinPublisherBondUnits int64 = 100 * UnitsPerToken
)

var (
	// ErrPublisherNotRegistered is returned for an unknown publisher id.
	ErrPublisherNotRegistered = errors.New("publisher is not registered")

	// ErrGameNotRegistered is returned for a game id the publisher does not own.
	ErrGameNotRegistered = errors.New("game is not registered to this publisher")

	// ErrRegistrationInvalid is returned for a malformed registration.
	ErrRegistrationInvalid = errors.New("invalid registration")

	// ErrKeyAlreadyRegistered is returned when one key tries to hold two
	// publisher identities.
	ErrKeyAlreadyRegistered = errors.New("public key is already registered to a publisher")
)

// GameRecord is one registered game inside a publisher.
type GameRecord struct {
	GameID             uint64
	Name               string
	RegisteredAtHeight int64
}

// PublisherRecord is a registered publisher.
type PublisherRecord struct {
	PublisherID        uint64
	PublicKeyPEM       string
	Name               string
	BondUnits          int64
	RegisteredAtHeight int64

	games      map[uint64]*GameRecord
	nextGameID uint64
}

// Games returns the publisher's registered games.
func (p *PublisherRecord) Games() []GameRecord {
	out := make([]GameRecord, 0, len(p.games))
	for _, game := range p.games {
		out = append(out, *game)
	}
	return out
}

// PublisherRegistry is the chain's record of who may write which block space.
//
// It is consensus state: every change arrives through an applied block and is
// reversed when that block is rolled back.
type PublisherRegistry struct {
	mu sync.RWMutex

	publishers map[uint64]*PublisherRecord
	// byKey stops one key holding two publisher identities. Sharing a key across
	// identities would mean compromising it compromises both, and it would make
	// "which publisher signed this" ambiguous at exactly the moment it matters.
	byKey           map[string]uint64
	nextPublisherID uint64
}

// NewPublisherRegistry creates an empty registry.
func NewPublisherRegistry() *PublisherRegistry {
	return &PublisherRegistry{
		publishers:      map[uint64]*PublisherRecord{},
		byKey:           map[string]uint64{},
		nextPublisherID: 1,
	}
}

// DerivePublisherBondAddress returns the address holding a publisher's bond.
//
// It is the hash of a domain string, not of a public key, so no private key for
// it exists anywhere -- the same construction the emission reserve uses. A bond
// behind a real wallet could be withdrawn by whoever held that wallet, which
// would make it a deposit rather than a bond.
func DerivePublisherBondAddress(publisherID uint64) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("gbb/publisher/bond/v1/%d", publisherID)))
	return hex.EncodeToString(sum[:])
}

// validateName checks registration metadata.
func validateName(name string) error {
	if name == "" {
		return fmt.Errorf("%w: name is empty", ErrRegistrationInvalid)
	}
	if len(name) > MaxPublisherNameLength {
		return fmt.Errorf("%w: name exceeds %d bytes",
			ErrRegistrationInvalid, MaxPublisherNameLength)
	}
	return nil
}

// RegisterPublisher allocates the next publisher id.
//
// The caller must have already checked that the registrant holds the private key
// for publicKeyPEM; this records the result of that check, it does not perform
// it.
func (r *PublisherRegistry) RegisterPublisher(publicKeyPEM, name string, bondUnits, height int64) (uint64, error) {
	if err := validateName(name); err != nil {
		return 0, err
	}
	if _, err := parsePeerPublicKey(publicKeyPEM); err != nil {
		return 0, fmt.Errorf("%w: %w", ErrRegistrationInvalid, err)
	}
	if bondUnits < MinPublisherBondUnits {
		return 0, fmt.Errorf("%w: bond of %d units is below the %d minimum",
			ErrRegistrationInvalid, bondUnits, MinPublisherBondUnits)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if existing, taken := r.byKey[publicKeyPEM]; taken {
		return 0, fmt.Errorf("%w: it holds publisher %d", ErrKeyAlreadyRegistered, existing)
	}

	id := r.nextPublisherID
	r.publishers[id] = &PublisherRecord{
		PublisherID:        id,
		PublicKeyPEM:       publicKeyPEM,
		Name:               name,
		BondUnits:          bondUnits,
		RegisteredAtHeight: height,
		games:              map[uint64]*GameRecord{},
		nextGameID:         1,
	}
	r.byKey[publicKeyPEM] = id
	r.nextPublisherID++

	return id, nil
}

// RegisterGame allocates the next game id inside a publisher.
func (r *PublisherRegistry) RegisterGame(publisherID uint64, name string, height int64) (uint64, error) {
	if err := validateName(name); err != nil {
		return 0, err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	publisher, ok := r.publishers[publisherID]
	if !ok {
		return 0, fmt.Errorf("%w: %d", ErrPublisherNotRegistered, publisherID)
	}

	id := publisher.nextGameID
	publisher.games[id] = &GameRecord{
		GameID:             id,
		Name:               name,
		RegisteredAtHeight: height,
	}
	publisher.nextGameID++

	return id, nil
}

// unregisterPublisher reverses a publisher registration.
//
// Only the most recent allocation can be reversed, which is the only thing a
// reorg ever asks for: blocks are rolled back newest first, so registrations
// unwind in exactly the order they were made. Refusing anything else keeps a
// buggy caller from leaving a hole in the id sequence, which would let a later
// registration reuse an id that a rolled-back sidechain still referred to.
func (r *PublisherRegistry) unregisterPublisher(publisherID uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if publisherID != r.nextPublisherID-1 {
		return fmt.Errorf("publisher %d is not the most recent registration", publisherID)
	}

	publisher, ok := r.publishers[publisherID]
	if !ok {
		return fmt.Errorf("%w: %d", ErrPublisherNotRegistered, publisherID)
	}

	delete(r.byKey, publisher.PublicKeyPEM)
	delete(r.publishers, publisherID)
	r.nextPublisherID--
	return nil
}

// unregisterGame reverses a game registration.
func (r *PublisherRegistry) unregisterGame(publisherID, gameID uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	publisher, ok := r.publishers[publisherID]
	if !ok {
		return fmt.Errorf("%w: %d", ErrPublisherNotRegistered, publisherID)
	}
	if gameID != publisher.nextGameID-1 {
		return fmt.Errorf("game %d is not publisher %d's most recent registration",
			gameID, publisherID)
	}

	delete(publisher.games, gameID)
	publisher.nextGameID--
	return nil
}

// Publisher returns a registered publisher's record.
func (r *PublisherRegistry) Publisher(publisherID uint64) (PublisherRecord, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	publisher, ok := r.publishers[publisherID]
	if !ok {
		return PublisherRecord{}, false
	}
	return *publisher, true
}

// PublicKey returns a publisher's registered key.
func (r *PublisherRegistry) PublicKey(publisherID uint64) (string, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	publisher, ok := r.publishers[publisherID]
	if !ok {
		return "", false
	}
	return publisher.PublicKeyPEM, true
}

// AuthoriseSidechain checks that a sidechain id names a registered game owned by
// a registered publisher, and returns the key entitled to anchor it.
//
// The game half is not a formality. Without it a registered publisher could
// anchor any game id at all, including ids belonging to games they never
// registered and ids another publisher's SDK is already using -- which would
// make the registry describe ownership it does not enforce.
func (r *PublisherRegistry) AuthoriseSidechain(id SidechainID) (string, error) {
	if err := id.Validate(); err != nil {
		return "", err
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	publisher, ok := r.publishers[id.PublisherID]
	if !ok {
		return "", fmt.Errorf("%w: %d", ErrPublisherNotRegistered, id.PublisherID)
	}
	if _, ok := publisher.games[id.GameID]; !ok {
		return "", fmt.Errorf("%w: publisher %d has no game %d",
			ErrGameNotRegistered, id.PublisherID, id.GameID)
	}

	return publisher.PublicKeyPEM, nil
}

// Count returns how many publishers are registered.
func (r *PublisherRegistry) Count() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.publishers)
}

// slashBond reduces a publisher's recorded bond, returning what was taken.
func (r *PublisherRegistry) slashBond(publisherID uint64, units int64) (int64, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	publisher, ok := r.publishers[publisherID]
	if !ok {
		return 0, fmt.Errorf("%w: %d", ErrPublisherNotRegistered, publisherID)
	}

	taken := units
	if taken > publisher.BondUnits {
		taken = publisher.BondUnits
	}
	publisher.BondUnits -= taken
	return taken, nil
}

// restoreBond puts a slashed amount back, for a reorg.
func (r *PublisherRegistry) restoreBond(publisherID uint64, units int64) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if publisher, ok := r.publishers[publisherID]; ok {
		publisher.BondUnits += units
	}
}
