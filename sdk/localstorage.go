package sdk

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// LocalStorage represents a local storage mechanism with a specified data path.
// It includes a read-write mutex to ensure thread-safe access to the stored data.
type LocalStorage struct {
	dataPath string
	mutex    sync.RWMutex
}

// localStorage is a singleton instance of the LocalStorage struct.
// It is initialized only once using the sync.Once mechanism to ensure
// thread-safe initialization.
var (
	localStorage *LocalStorage
	once         sync.Once
)

// NewLocalStorage initializes a new LocalStorage instance with the specified data path.
// If the provided data path is empty, it defaults to "./data". This function ensures that
// the LocalStorage instance is set up only once using a sync.Once mechanism.
//
// Parameters:
//   - dataPath: The path where the local storage data will be stored.
//
// Returns:
//   - error: An error if the setup of the LocalStorage instance fails, otherwise nil.
func NewLocalStorage(dataPath string) error {
	var err error
	once.Do(func() {
		if dataPath == "" {
			dataPath = "./data"
		}
		localStorage = &LocalStorage{dataPath: dataPath}
		err = localStorage.setup()
	})
	return err
}

// GetLocalStorage retrieves the singleton instance of LocalStorage.
// If the local storage has not been initialized, it returns an error.
//
// Returns:
//   - (*LocalStorage, error): The instance of LocalStorage if initialized, otherwise an error.
func GetLocalStorage() (*LocalStorage, error) {
	if localStorage == nil {
		return nil, fmt.Errorf("local storage not initialized")
	}
	return localStorage, nil
}

// setup initializes the local storage by creating necessary directories
// such as "node", "blocks", and "wallets" within the specified data path.
// It returns an error if any of the directory creation operations fail.
func (ls *LocalStorage) setup() error {
	directories := []string{"node", "blocks", "wallets"}
	for _, dir := range directories {
		err := os.MkdirAll(filepath.Join(ls.dataPath, dir), 0755)
		if err != nil {
			return fmt.Errorf("failed to create directory %s: %w", dir, err)
		}
	}
	return nil
}

// Get retrieves the value associated with the given key from the LocalStorage.

// Get retrieves the value associated with the given key from the local storage.
// It reads the data from the file corresponding to the key, and unmarshals it into the provided interface.
//
// Parameters:
//   - key: The key associated with the value to retrieve.
//   - v: A pointer to the variable where the unmarshaled data will be stored.
//
// Returns:
//   - error: An error if the file cannot be read or the data cannot be unmarshaled, otherwise nil.
func (ls *LocalStorage) Get(key string, v interface{}) error {
	ls.mutex.RLock()
	defer ls.mutex.RUnlock()

	filePath := ls.getFilePath(key)
	data, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("failed to read file %s: %w", filePath, err)
	}

	err = json.Unmarshal(data, v)
	if err != nil {
		return fmt.Errorf("failed to unmarshal data: %w", err)
	}

	return nil
}

// Set stores the given value in the local storage under the specified key.
// It serializes the value to JSON format and writes it to a file.
// The file is named based on the key and stored in the local storage directory.
// If an error occurs during serialization or file writing, it returns an error.
//
// Parameters:
//   - key: The key under which the value will be stored.
//   - v: The value to be stored, which will be serialized to JSON.
//
// Returns:
//   - error: An error if the value could not be serialized or the file could not be written.
//
// Set stores the provided value under the given key in the LocalStorage.
func (ls *LocalStorage) Set(key string, v interface{}) error {
	ls.mutex.Lock()
	defer ls.mutex.Unlock()

	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("failed to marshal data: %w", err)
	}

	filePath := ls.getFilePath(key)
	err = os.WriteFile(filePath, data, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	return nil
}

// List retrieves a list of all file paths relative to the dataPath directory.
// It locks the mutex for reading to ensure thread safety while accessing the file system.
// The function walks through the dataPath directory and collects all file paths that are not directories.
// If an error occurs during the walk, it returns the error wrapped in a descriptive message.
// Returns a slice of relative file paths or an error if the operation fails.
func (ls *LocalStorage) List() ([]string, error) {
	ls.mutex.RLock()
	defer ls.mutex.RUnlock()

	var keys []string
	err := filepath.Walk(ls.dataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			relPath, _ := filepath.Rel(ls.dataPath, path)
			keys = append(keys, relPath)
		}
		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	return keys, nil
}

// getFilePath returns the file path for the given key.
func (ls *LocalStorage) getFilePath(key string) string {
	return filepath.Join(ls.dataPath, key+".json")
}
