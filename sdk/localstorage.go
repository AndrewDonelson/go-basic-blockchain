// Package sdk is a software development kit for building blockchain applications.
// File sdk/localstorage.go - Local Storage for all data persist manager
package sdk

import (
	//"encoding/json"

	"fmt"
	"log"
	"os"
	"path/filepath"

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
type LocalStorage struct {
	dataPath string
}

// localStorage is a global variable that holds an instance of the LocalStorage struct.
// It provides access to the data persist manager using the Go standard library's file system.
var localStorage *LocalStorage

// NewLocalStorage creates a new instance of the LocalStorage struct, which is the data persist manager
// using the Go standard library's file system. If dataPath is an empty string, it will use the default
// data path "./data". The function also performs any initial setup or data loading if needed.
//
// If local storage has already been initialized, this function will return an error.
func NewLocalStorage(dataPath string) error {
	if localStorage != nil {
		// Return the existing instance
		return fmt.Errorf("local storage already initialized")
	}

	// Create the LocalStorage instance
	localStorage = &LocalStorage{}
	if dataPath == "" {
		// Use the default data path
		localStorage.dataPath = "./data" // Modify the data path based on your requirements
	} else {
		// Use the given data path
		localStorage.dataPath = dataPath
	}

	// Perform any initial setup or data loading if needed
	localStorage.setup()

	fmt.Println("local storage initialized @", localStorage.dataPath)
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

// setup creates the necessary directories for the LocalStorage data persist manager.
// It creates the following directories if they don't already exist:
// - data directory (specified by ls.dataPath)
// - node directory (under the data directory)
// - blocks directory (under the data directory)
// - wallets directory (under the data directory)
// This function is called during the initialization of the LocalStorage instance.
func (ls *LocalStorage) setup() {
	// Create the data directory if it doesn't exist
	err := os.MkdirAll(ls.dataPath, 0755)
	if err != nil {
		log.Fatal(err)
	}

	// Create the node directory if it doesn't exist
	// err = os.MkdirAll(filepath.Join(ls.dataPath, "node"), 0755)
	// if err != nil {
	// 	log.Fatal(err)
	// }

	// Create the blocks directory if it doesn't exist
	err = os.MkdirAll(filepath.Join(ls.dataPath, "blocks"), 0755)
	if err != nil {
		log.Fatal(err)
	}

	// Create the wallets directory if it doesn't exist
	err = os.MkdirAll(filepath.Join(ls.dataPath, "wallets"), 0755)
	if err != nil {
		log.Fatal(err)
	}
}

// file returns the file path for the given type of data that needs to be persisted.
// It handles different types of data, such as NodePersistData, BlockchainPersistData, Block, and Wallet,
// and generates the appropriate file path based on the type.
// If the type is not supported, it returns an error.
func (ls *LocalStorage) file(t interface{}) (filePath string, err error) {

	log.Printf("LocalStorage.file: Interface Detected: %T", t)
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
	if verbose {
		log.Println("LocalStorage.Set: Start")
	}

	filePath, err := ls.file(v)
	if err != nil {
		return err
	}

	if verbose {
		log.Printf("LocalStorage.Set: Preparing %s to %v\n", filePath, PrettyPrint(v))
	}

	data, err := jsoniter.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	if verbose {
		log.Printf("LocalStorage.Set: Writing %s to %s\n", filePath, data)
	}
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
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
		fmt.Printf("BlockQueryCriteria: %+v\n", criteria)
		// Query Blocks based on criteria
		blocks := []*Block{}
		// Implement logic to query Blocks based on criteria

		// Return the found Blocks
		return []interface{}{blocks}, nil
	case *TransactionQueryCriteria:
		fmt.Printf("TransactionQueryCriteria: %+v\n", criteria)
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
