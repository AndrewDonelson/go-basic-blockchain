package sdk

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/scrypt"
)

const (
	accountStoreSchemaVersionV1 = 1
	accountStoreSchemaVersionV2 = 2
	accountStoreSchemaVersionV3 = 3
	accountStoreSchemaCurrent   = accountStoreSchemaVersionV3
)

// PendingAccountRecord is a registration awaiting email verification.
//
// PasswordHash/PasswordSalt are produced *server-side* by hashAccountPassword.
// Previously the API accepted a client-supplied "password_hash" and stored it
// verbatim, which made the stored value itself the credential: anyone who could
// read state.json could authenticate directly.
type PendingAccountRecord struct {
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	PasswordSalt string    `json:"password_salt"`
	TokenHash    string    `json:"token_hash"`
	ExpiresAt    time.Time `json:"expires_at"`
}

// VerifiedAccountRecord is a verified account.
type VerifiedAccountRecord struct {
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	PasswordSalt string    `json:"password_salt"`
	APIKeyHash   string    `json:"api_key_hash"`
	VerifiedAt   time.Time `json:"verified_at"`
}

// AccountStore persists account registrations.
type AccountStore interface {
	SavePending(record PendingAccountRecord) error
	GetPending(email string) (PendingAccountRecord, bool, error)
	DeletePending(email string) error
	SaveVerified(record VerifiedAccountRecord) error
	GetVerified(email string) (VerifiedAccountRecord, bool, error)
	// GetVerifiedByAPIKeyHash resolves an account from the hash of an issued API
	// key, so the middleware can authenticate keys handed out by /account/login.
	GetVerifiedByAPIKeyHash(hash string) (VerifiedAccountRecord, bool, error)
}

type accountStoreState struct {
	SchemaVersion int                              `json:"schema_version"`
	Pending       map[string]PendingAccountRecord  `json:"pending"`
	Verified      map[string]VerifiedAccountRecord `json:"verified"`
}

type FileAccountStore struct {
	statePath string
	mu        sync.Mutex
}

func NewFileAccountStore(dataPath string) AccountStore {
	if dataPath == "" {
		dataPath = dataFolder
	}
	return &FileAccountStore{statePath: filepath.Join(dataPath, "accounts", "state.json")}
}

func (s *FileAccountStore) SavePending(record PendingAccountRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.loadStateLocked()
	if err != nil {
		return err
	}

	email := normalizeAccountEmail(record.Email)
	record.Email = email
	state.Pending[email] = record

	return s.saveStateLocked(state)
}

func (s *FileAccountStore) GetPending(email string) (PendingAccountRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.loadStateLocked()
	if err != nil {
		return PendingAccountRecord{}, false, err
	}

	record, ok := state.Pending[normalizeAccountEmail(email)]
	return record, ok, nil
}

func (s *FileAccountStore) DeletePending(email string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.loadStateLocked()
	if err != nil {
		return err
	}

	delete(state.Pending, normalizeAccountEmail(email))
	return s.saveStateLocked(state)
}

func (s *FileAccountStore) SaveVerified(record VerifiedAccountRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.loadStateLocked()
	if err != nil {
		return err
	}

	email := normalizeAccountEmail(record.Email)
	record.Email = email
	state.Verified[email] = record

	return s.saveStateLocked(state)
}

func (s *FileAccountStore) GetVerified(email string) (VerifiedAccountRecord, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.loadStateLocked()
	if err != nil {
		return VerifiedAccountRecord{}, false, err
	}

	record, ok := state.Verified[normalizeAccountEmail(email)]
	return record, ok, nil
}

// GetVerifiedByAPIKeyHash resolves an account from an issued API key's hash.
func (s *FileAccountStore) GetVerifiedByAPIKeyHash(hash string) (VerifiedAccountRecord, bool, error) {
	if hash == "" {
		return VerifiedAccountRecord{}, false, nil
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	state, err := s.loadStateLocked()
	if err != nil {
		return VerifiedAccountRecord{}, false, err
	}

	// Constant-time comparison across all candidates so a valid prefix is not
	// distinguishable by timing from an invalid one.
	var match VerifiedAccountRecord
	found := false
	for _, record := range state.Verified {
		if record.APIKeyHash == "" {
			continue
		}
		if subtle.ConstantTimeCompare([]byte(record.APIKeyHash), []byte(hash)) == 1 {
			match = record
			found = true
		}
	}
	return match, found, nil
}

func (s *FileAccountStore) loadStateLocked() (accountStoreState, error) {
	//nolint:gosec // G703: statePath is operator-configured, not request-derived
	if err := os.MkdirAll(filepath.Dir(s.statePath), 0700); err != nil {
		return accountStoreState{}, fmt.Errorf("create account store directory: %w", err)
	}

	state := accountStoreState{
		SchemaVersion: accountStoreSchemaCurrent,
		Pending:       map[string]PendingAccountRecord{},
		Verified:      map[string]VerifiedAccountRecord{},
	}

	// s.statePath is built from the configured data directory at construction; no
	// part of it comes from a request.
	bytes, err := os.ReadFile(s.statePath) //nolint:gosec // G703: operator-configured path
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return state, nil
		}
		return accountStoreState{}, fmt.Errorf("read account store: %w", err)
	}

	if len(bytes) == 0 {
		return state, nil
	}

	var envelope map[string]json.RawMessage
	if err := json.Unmarshal(bytes, &envelope); err != nil {
		return accountStoreState{}, fmt.Errorf("unmarshal account store envelope: %w", err)
	}

	if _, hasSchema := envelope["schema_version"]; hasSchema {
		if err := json.Unmarshal(bytes, &state); err != nil {
			return accountStoreState{}, fmt.Errorf("unmarshal account store: %w", err)
		}

		switch state.SchemaVersion {
		case accountStoreSchemaVersionV1, accountStoreSchemaVersionV2:
			// V1/V2 stored a client-supplied password hash with no salt and no
			// issued API key. Those records cannot be migrated into the new scheme
			// without the original password, so they are carried forward as-is and
			// re-verified on next login attempt.
			state.SchemaVersion = accountStoreSchemaCurrent
		case accountStoreSchemaCurrent:
			// no migration required
		default:
			return accountStoreState{}, fmt.Errorf("unsupported account store schema version: %d", state.SchemaVersion)
		}
	} else {
		// Legacy schema (pre-versioned): root-level pending/verified only.
		if err := json.Unmarshal(bytes, &state); err != nil {
			return accountStoreState{}, fmt.Errorf("unmarshal legacy account store: %w", err)
		}
		state.SchemaVersion = accountStoreSchemaCurrent
	}

	if state.Pending == nil {
		state.Pending = map[string]PendingAccountRecord{}
	}
	if state.Verified == nil {
		state.Verified = map[string]VerifiedAccountRecord{}
	}

	for k, record := range state.Pending {
		normalized := normalizeAccountEmail(k)
		record.Email = normalizeAccountEmail(record.Email)
		delete(state.Pending, k)
		state.Pending[normalized] = record
	}

	for k, record := range state.Verified {
		normalized := normalizeAccountEmail(k)
		record.Email = normalizeAccountEmail(record.Email)
		delete(state.Verified, k)
		state.Verified[normalized] = record
	}

	return state, nil
}

func (s *FileAccountStore) saveStateLocked(state accountStoreState) error {
	state.SchemaVersion = accountStoreSchemaCurrent

	bytes, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal account store: %w", err)
	}

	// 0600, not 0644: this file holds password hashes and API key hashes.
	if err := writeFileAtomic(s.statePath, bytes, 0600); err != nil {
		return fmt.Errorf("write account store: %w", err)
	}

	return nil
}

// hashAccountPassword derives a password hash using scrypt with a fresh random
// salt, and returns both hex-encoded.
func hashAccountPassword(password string) (hash string, salt string, err error) {
	saltBytes := make([]byte, saltSize)
	if _, err := rand.Read(saltBytes); err != nil {
		return "", "", fmt.Errorf("generate password salt: %w", err)
	}
	sum, err := scryptPassword(password, saltBytes)
	if err != nil {
		return "", "", err
	}
	return hex.EncodeToString(sum), hex.EncodeToString(saltBytes), nil
}

// verifyAccountPassword checks a password against a stored hash and salt in
// constant time.
func verifyAccountPassword(password, hexHash, hexSalt string) bool {
	saltBytes, err := hex.DecodeString(hexSalt)
	if err != nil {
		return false
	}
	expected, err := hex.DecodeString(hexHash)
	if err != nil {
		return false
	}
	sum, err := scryptPassword(password, saltBytes)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare(sum, expected) == 1
}

// scryptPassword applies the account password KDF.
func scryptPassword(password string, salt []byte) ([]byte, error) {
	sp := defaultScryptParams
	sum, err := scrypt.Key([]byte(password), salt, sp.N, sp.R, sp.P, 32)
	if err != nil {
		return nil, fmt.Errorf("derive password hash: %w", err)
	}
	return sum, nil
}

func normalizeAccountEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}
