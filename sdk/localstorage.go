// Package sdk is a software development kit for building blockchain applications.
// File sdk/localstorage.go - Local Storage for all data persist manager
package sdk

import (
	//"encoding/json"

	"fmt"
	"os"
	"path/filepath"
	"sync"

	jsoniter "github.com/json-iterator/go"
)

// LocalStorageOptions represents the options for the LocalStorage data persist manager.
// DataPath specifies the path where all data is stored.
// NodePrivateKey specifies the private key of the node, which can be used to encrypt data.
// NumCacheItems specifies the number of items to cache in memory, which applies to all types (blocks, transactions, etc).
type LocalStorageOptions struct {
	DataPath       string // path where all data is stored
	NodePrivateKey string // private key of the node (if you want to encrypt data)
	NumCacheItems  int    // number of items to cache in memory. this applies to all type (blocks, transactions, etc)
}

// LocalStorage represents the data persist manager using the Go standard library's file system.
// The dataPath field specifies the path where all data is stored.
//
// mu serialises reads and writes. This is a process-wide singleton written from
// the mining goroutine, the API handlers and the interactive menu at the same
// time; without it, concurrent Set calls interleaved and could corrupt a file.
type LocalStorage struct {
	dataPath string
	mu       sync.RWMutex
}

// writeFileAtomic writes data to path via a temporary file and an atomic rename,
// so a crash or a concurrent reader can never observe a half-written file.
//
// The previous direct os.WriteFile truncated the target first: an interrupted
// write left a truncated wallet or block on disk with no way to recover it.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+".tmp-*")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpName := tmp.Name()

	cleanup := func() {
		tmp.Close()
		os.Remove(tmpName)
	}

	if err := tmp.Chmod(perm); err != nil {
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if _, err := tmp.Write(data); err != nil {
		cleanup()
		return fmt.Errorf("write temp file: %w", err)
	}
	// fsync before rename so the rename cannot expose an empty file after a crash.
	if err := tmp.Sync(); err != nil {
		cleanup()
		return fmt.Errorf("sync temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp file: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename temp file: %w", err)
	}
	return nil
}

// localStorage is a global variable that holds an instance of the LocalStorage struct.
// It provides access to the data persist manager using the Go standard library's file system.
var localStorage *LocalStorage

// NewLocalStorage creates a new instance of the LocalStorage struct, which is the data persist manager
// using the Go standard library's file system. It takes a dataPath parameter to specify where the data
// should be stored. If local storage has already been initialized, this function will update the data path
// if it's different from the current one.
func NewLocalStorage(dataPath string) error {
	if localStorage != nil {
		// If already initialized, check if we need to update the data path
		if localStorage.dataPath != dataPath && dataPath != "" {
			localStorage.mu.Lock()
			localStorage.dataPath = dataPath
			localStorage.mu.Unlock()
			if err := localStorage.setup(); err != nil {
				return err
			}
			LogInfof("local storage reinitialized @ %s", localStorage.dataPath)
		}
		return nil
	}

	// Create the LocalStorage instance
	localStorage = &LocalStorage{}
	if dataPath == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			localStorage.dataPath = filepath.Join(home, "gbb-data")
		} else {
			localStorage.dataPath = "./gbb-data"
		}
	} else {
		// Use the given data path
		localStorage.dataPath = dataPath
	}

	// Perform any initial setup or data loading if needed
	if err := localStorage.setup(); err != nil {
		localStorage = nil
		return err
	}

	LogInfof("local storage initialized @ %s", localStorage.dataPath)
	return nil
}

// / GetLocalStorage returns the singleton instance of the LocalStorage struct, which provides access to the
// / data persist manager using the Go standard library's file system. If local storage has not been
// / initialized, this function will return an error.
func GetLocalStorage() (*LocalStorage, error) {
	if localStorage == nil {
		return nil, fmt.Errorf("local storage not initialized")
	}

	return localStorage, nil
}

// LocalStorageAvailable returns a boolean indicating whether the LocalStorage instance has been initialized.
// This function can be used to check if the local storage data persist manager is available for use.
func LocalStorageAvailable() bool {
	return localStorage != nil
}

// storagePerm returns the file mode for a persisted object. Wallets hold private
// key material and must not be world-readable.
func storagePerm(v interface{}) os.FileMode {
	if _, ok := v.(*Wallet); ok {
		return 0600
	}
	return 0644
}

// setup creates the necessary directories for the LocalStorage data persist manager.
// It creates the following directories if they don't already exist:
// - data directory (specified by ls.dataPath)
// - node directory (under the data directory)
// - blocks directory (under the data directory)
// - wallets directory (under the data directory)
// This function is called during the initialization of the LocalStorage instance.
func (ls *LocalStorage) setup() error {
	// Create the data directory if it doesn't exist.
	// These used to be log.Fatal: a library has no business terminating the
	// host process because a directory could not be created.
	if err := os.MkdirAll(ls.dataPath, 0755); err != nil {
		return fmt.Errorf("create data directory: %w", err)
	}

	// Create the node directory if it doesn't exist
	// err = os.MkdirAll(filepath.Join(ls.dataPath, "node"), 0755)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// Create the blocks directory if it doesn't exist
	if err := os.MkdirAll(filepath.Join(ls.dataPath, "blocks"), 0755); err != nil {
		return fmt.Errorf("create blocks directory: %w", err)
	}

	// Create the wallets directory if it doesn't exist. 0700: it holds keys.
	if err := os.MkdirAll(filepath.Join(ls.dataPath, "wallets"), 0700); err != nil {
		return fmt.Errorf("create wallets directory: %w", err)
	}

	return nil
}

// file returns the file path for the given type of data that needs to be persisted.
// It handles different types of data, such as NodePersistData, BlockchainPersistData, Block, and Wallet,
// and generates the appropriate file path based on the type.
// If the type is not supported, it returns an error.
func (ls *LocalStorage) file(t interface{}) (filePath string, err error) {

	LogVerbosef("LocalStorage.file: Interface Detected: %T", t)
	switch tt := t.(type) {
	case *NodePersistData:
		filePath = filepath.Join(ls.dataPath, "node.json")
	case *BlockchainPersistData:
		filePath = filepath.Join(ls.dataPath, "blockchain.json")
	case *Block:
		// where t is a Block
		filePath = filepath.Join(ls.dataPath, "blocks", fmt.Sprintf("%s.json", (t.(*Block).Index).String()))
	case *Wallet:
		filePath = filepath.Join(ls.dataPath, "wallets", tt.Address+".json")
	default:
		err = fmt.Errorf("unsupported type [%T]", tt)
	}

	return filePath, err
}

// Get retrieves the value associated with the given key from the LocalStorage.
// It decodes the JSON data from the file corresponding to the type of the provided value.
// If the file does not exist or the JSON data cannot be decoded, an error is returned.
func (ls *LocalStorage) Get(key string, v interface{}) error {
	ls.mu.RLock()
	defer ls.mu.RUnlock()

	filePath, err := ls.file(v)
	if err != nil {
		return err
	}

	// Open the file
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Decode the JSON data from the file
	//err = json.NewDecoder(file).Decode(v)
	err = jsoniter.NewDecoder(file).Decode(v)
	if err != nil {
		return err
	}

	return nil
}

// Set stores the provided value under the given key in the LocalStorage.
// It creates or truncates the file corresponding to the type of the provided value,
// and encodes the value as JSON data in the file.
// If an error occurs while creating the file or encoding the data, an error is returned.
func (ls *LocalStorage) Set(key string, v interface{}) error {
	ls.mu.Lock()
	defer ls.mu.Unlock()

	filePath, err := ls.file(v)
	if err != nil {
		return err
	}

	data, err := jsoniter.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if err := writeFileAtomic(filePath, data, storagePerm(v)); err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}

// Find searches for data in the LocalStorage based on the given criteria.
// It supports two types of criteria: BlockQueryCriteria and TransactionQueryCriteria.
// For BlockQueryCriteria, it will query and return the matching Blocks.
// For TransactionQueryCriteria, it will query and return the matching Transactions.
// If the criteria type is unsupported, an error is returned.
func (ls *LocalStorage) Find(criteria interface{}) ([]interface{}, error) {
	// Implement the logic to find data based on the given criteria using file system operations

	// Dummy implementation
	switch criteria := criteria.(type) {
	case *BlockQueryCriteria:
		LogInfof("BlockQueryCriteria: %+v", criteria)
		// Query Blocks based on criteria
		blocks := []*Block{}
		// Implement logic to query Blocks based on criteria

		// Return the found Blocks
		return []interface{}{blocks}, nil
	case *TransactionQueryCriteria:
		LogInfof("TransactionQueryCriteria: %+v", criteria)
		// Query Transactions based on criteria
		transactions := []*Transaction{}
		// Implement logic to query Transactions based on criteria

		// Return the found Transactions
		return []interface{}{transactions}, nil
	default:
		return nil, fmt.Errorf("unsupported criteria type")
	}
}

// NodeData represents a node in the system, with an ID and a Name.
type NodeData struct {
	ID   string
	Name string
	// Additional fields as needed
}

// BlockchainData represents the data persisted for a blockchain.
// It contains an ID and a Version field, along with any additional fields as needed.
type BlockchainData struct {
	ID      string
	Version string
	// Additional fields as needed
}

// BlockQueryCriteria represents the criteria for querying blocks.
type BlockQueryCriteria struct {
	Number int
	// Additional criteria fields as needed
}

// TransactionQueryCriteria represents the criteria for querying transactions.
type TransactionQueryCriteria struct {
	Amount float64
	// Additional criteria fields as needed
}
