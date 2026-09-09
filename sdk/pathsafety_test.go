package sdk

import (
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestWalletAddressCannotEscapeTheDataDirectory pins a path traversal.
//
// A wallet address arrives straight from a URL path variable
// (/blockchain/wallets/{id}) and was joined into a file path unchecked, so
// "../node" resolved to <dataPath>/node.json. An authenticated caller could read
// -- and through the update handler write -- any .json file relative to the data
// directory.
func TestWalletAddressCannotEscapeTheDataDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := NewLocalStorage(dir); err != nil {
		t.Fatalf("storage: %v", err)
	}
	t.Cleanup(func() { _ = NewLocalStorage("./test_data") })

	ls, err := GetLocalStorage()
	if err != nil {
		t.Fatalf("get storage: %v", err)
	}

	// A file outside the wallets directory that must stay unreachable.
	if err := os.WriteFile(filepath.Join(dir, "node.json"), []byte(`{"ID":"secret"}`), 0600); err != nil {
		t.Fatalf("plant: %v", err)
	}

	hostile := []string{
		"../node",
		"../../etc/passwd",
		"..",
		".",
		"wallets/../../node",
		"sub/dir",
		`..\node`,
		"a/../../b",
		"",
	}

	walletsDir := filepath.Join(dir, "wallets")
	for _, address := range hostile {
		t.Run("address="+address, func(t *testing.T) {
			path, err := ls.file(&Wallet{Address: address})
			if err != nil {
				return // refused outright, which is the intent
			}

			resolved, absErr := filepath.Abs(path)
			if absErr != nil {
				t.Fatalf("abs: %v", absErr)
			}
			root, absErr := filepath.Abs(walletsDir)
			if absErr != nil {
				t.Fatalf("abs: %v", absErr)
			}

			rel, relErr := filepath.Rel(root, resolved)
			if relErr != nil {
				t.Fatalf("rel: %v", relErr)
			}
			if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
				t.Fatalf("address %q resolved to %s, outside the wallets directory",
					address, resolved)
			}
		})
	}
}

// TestBlockIndexCannotEscapeTheDataDirectory covers the other identifier that
// becomes a filename.
func TestBlockIndexCannotEscapeTheDataDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := NewLocalStorage(dir); err != nil {
		t.Fatalf("storage: %v", err)
	}
	t.Cleanup(func() { _ = NewLocalStorage("./test_data") })

	ls, err := GetLocalStorage()
	if err != nil {
		t.Fatalf("get storage: %v", err)
	}

	// A legitimate index still resolves.
	block := NewBlock(nil, "")
	block.Index = *big.NewInt(7)
	path, err := ls.file(block)
	if err != nil {
		t.Fatalf("a valid block index was refused: %v", err)
	}
	if !strings.HasSuffix(path, filepath.Join("blocks", "7.json")) {
		t.Fatalf("unexpected block path: %s", path)
	}
}

// TestStorageFileNameRejectsSeparators is the unit-level rule.
//
// There is no legitimate identifier in this project that contains a separator --
// addresses are hex, indices decimal -- so a separator is refused rather than
// escaped. Escaping invites the next encoding bug; refusing does not.
func TestStorageFileNameRejectsSeparators(t *testing.T) {
	valid := []string{"abc123", "0", "deadbeef", "a-b_c",
		strings.Repeat("a", 64)}
	for _, id := range valid {
		if _, err := storageFileName(id); err != nil {
			t.Fatalf("a legitimate identifier %q was refused: %v", id, err)
		}
	}

	hostile := []string{
		"", "..", ".", "../x", "a/b", `a\b`, "a\x00b", "a b",
		"a:b", "a.json", strings.Repeat("a", maxStorageNameLength+1),
	}
	for _, id := range hostile {
		if name, err := storageFileName(id); err == nil {
			t.Fatalf("identifier %q was accepted as %q", id, name)
		}
	}
}

// TestContainmentCatchesAnEscape covers the second line of defence directly, so
// a future caller that builds a name some other way still cannot escape.
func TestContainmentCatchesAnEscape(t *testing.T) {
	dir := t.TempDir()
	ls := &LocalStorage{dataPath: dir}

	if err := ls.containedInDataPath(filepath.Join(dir, "wallets", "ok.json")); err != nil {
		t.Fatalf("a path inside the data directory was refused: %v", err)
	}
	if err := ls.containedInDataPath(filepath.Join(dir, "..", "escaped.json")); err == nil {
		t.Fatal("a path outside the data directory was accepted")
	}
	if err := ls.containedInDataPath("/etc/passwd"); err == nil {
		t.Fatal("an absolute path outside the data directory was accepted")
	}
}
