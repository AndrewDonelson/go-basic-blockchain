package sdk

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
)

// LocalStorageOptions represents the options for the LocalStorage data persist manager.
type LocalStorageOptions struct {
	DataPath       string // path where all data is stored
	NodePrivateKey string // private key of the node (if you want to encrypt data)
	NumCacheItems  int    // number of items to cache in memory. this applies to all type (blocks, transactions, etc)
}

// LocalStorage represents the data persist manager using the Go standard library's file system.
type LocalStorage struct {
	dataPath string
}

var localStorage *LocalStorage

// NewLocalStorage creates a new LocalStorage instance.
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

func GetLocalStorage() (*LocalStorage, error) {
	if localStorage == nil {
		return nil, fmt.Errorf("local storage not initialized")
	}

	return localStorage, nil
}

// LocalStorageAvailable returns true if the LocalStorage is available, false otherwise.
func LocalStorageAvailable() bool {
	return localStorage != nil
}

// setup performs any initial setup or data loading if needed
func (ls *LocalStorage) setup() {
	// Create the data directory if it doesn't exist
	err := os.MkdirAll(ls.dataPath, 0755)
	if err != nil {
		log.Fatal(err)
	}

	// Create the node directory if it doesn't exist
	err = os.MkdirAll(filepath.Join(ls.dataPath, "node"), 0755)
	if err != nil {
		log.Fatal(err)
	}

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

// file returns the file path for the given type
func (ls *LocalStorage) file(t interface{}) (filePath string, err error) {

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

// Get returns the data for the given key
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
	err = json.NewDecoder(file).Decode(v)
	if err != nil {
		return err
	}

	return nil
}

// Set sets the data for the given key
func (ls *LocalStorage) Set(key string, v interface{}) error {
	filePath, err := ls.file(v)
	if err != nil {
		return err
	}

	// Create or truncate the file
	file, err := os.Create(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Encode the data as JSON and write it to the file
	err = json.NewEncoder(file).Encode(v)
	if err != nil {
		return err
	}

	return nil
}

// Find finds data based on the given criteria
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

// NodeData represents the data persisted for a node.
type NodeData struct {
	ID   string
	Name string
	// Additional fields as needed
}

// BlockchainData represents the data persisted for a blockchain.
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
