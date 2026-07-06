package sdk

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	accountStoreSchemaVersionV1 = 1
	accountStoreSchemaVersionV2 = 2
	accountStoreSchemaCurrent   = accountStoreSchemaVersionV2
)

type PendingAccountRecord struct {
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	Token        string    `json:"token"`
	ExpiresAt    time.Time `json:"expires_at"`
}

type VerifiedAccountRecord struct {
	Email        string    `json:"email"`
	PasswordHash string    `json:"password_hash"`
	VerifiedAt   time.Time `json:"verified_at"`
}

type AccountStore interface {
	SavePending(record PendingAccountRecord) error
	GetPending(email string) (PendingAccountRecord, bool, error)
	DeletePending(email string) error
	SaveVerified(record VerifiedAccountRecord) error
	GetVerified(email string) (VerifiedAccountRecord, bool, error)
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

func (s *FileAccountStore) loadStateLocked() (accountStoreState, error) {
	if err := os.MkdirAll(filepath.Dir(s.statePath), 0755); err != nil {
		return accountStoreState{}, fmt.Errorf("create account store directory: %w", err)
	}

	state := accountStoreState{
		SchemaVersion: accountStoreSchemaCurrent,
		Pending:       map[string]PendingAccountRecord{},
		Verified:      map[string]VerifiedAccountRecord{},
	}

	bytes, err := os.ReadFile(s.statePath)
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
		case accountStoreSchemaVersionV1:
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

	if err := os.WriteFile(s.statePath, bytes, 0644); err != nil {
		return fmt.Errorf("write account store: %w", err)
	}

	return nil
}

func normalizeAccountEmail(email string) string {
	return strings.TrimSpace(strings.ToLower(email))
}
