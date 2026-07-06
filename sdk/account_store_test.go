package sdk

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFileAccountStore_LegacyStateMigration(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "accounts", "state.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		t.Fatalf("mkdir accounts dir: %v", err)
	}

	legacy := map[string]interface{}{
		"pending": map[string]interface{}{
			"legacy@example.com": map[string]interface{}{
				"email":         "legacy@example.com",
				"password_hash": "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				"token":         "legacy-token",
				"expires_at":    time.Now().Add(30 * time.Minute),
			},
		},
		"verified": map[string]interface{}{},
	}

	bytes, err := json.MarshalIndent(legacy, "", "  ")
	if err != nil {
		t.Fatalf("marshal legacy state: %v", err)
	}
	if err := os.WriteFile(statePath, bytes, 0644); err != nil {
		t.Fatalf("write legacy state: %v", err)
	}

	store := &FileAccountStore{statePath: statePath}

	record, ok, err := store.GetPending("legacy@example.com")
	if err != nil {
		t.Fatalf("get pending from legacy state: %v", err)
	}
	if !ok {
		t.Fatal("expected pending record from legacy state")
	}
	if record.Token != "legacy-token" {
		t.Fatalf("unexpected token in migrated pending record: %s", record.Token)
	}

	if err := store.SaveVerified(VerifiedAccountRecord{
		Email:        "legacy@example.com",
		PasswordHash: record.PasswordHash,
		VerifiedAt:   time.Now(),
	}); err != nil {
		t.Fatalf("save verified after legacy migration: %v", err)
	}

	updatedBytes, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("read updated state: %v", err)
	}

	var state accountStoreState
	if err := json.Unmarshal(updatedBytes, &state); err != nil {
		t.Fatalf("unmarshal updated state: %v", err)
	}

	if state.SchemaVersion != accountStoreSchemaCurrent {
		t.Fatalf("expected schema version %d, got %d", accountStoreSchemaCurrent, state.SchemaVersion)
	}
}

func TestFileAccountStore_UnsupportedSchemaVersion(t *testing.T) {
	tmpDir := t.TempDir()
	statePath := filepath.Join(tmpDir, "accounts", "state.json")
	if err := os.MkdirAll(filepath.Dir(statePath), 0755); err != nil {
		t.Fatalf("mkdir accounts dir: %v", err)
	}

	invalid := map[string]interface{}{
		"schema_version": 999,
		"pending":        map[string]interface{}{},
		"verified":       map[string]interface{}{},
	}

	bytes, err := json.MarshalIndent(invalid, "", "  ")
	if err != nil {
		t.Fatalf("marshal invalid schema state: %v", err)
	}
	if err := os.WriteFile(statePath, bytes, 0644); err != nil {
		t.Fatalf("write invalid schema state: %v", err)
	}

	store := &FileAccountStore{statePath: statePath}
	if _, _, err := store.GetPending("nobody@example.com"); err == nil {
		t.Fatal("expected error for unsupported schema version")
	}
}
