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

var (
	localStorage *LocalStorage
	once         sync.Once
)

// NewLocalStorage creates a new instance of the LocalStorage struct.
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

// GetLocalStorage returns the singleton instance of the LocalStorage struct.
func GetLocalStorage() (*LocalStorage, error) {
	if localStorage == nil {
		return nil, fmt.Errorf("local storage not initialized")
	}
	return localStorage, nil
}

// setup creates the necessary directories for the LocalStorage data persist manager.
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

// Delete removes the file associated with the given key.
func (ls *LocalStorage) Delete(key string) error {
	ls.mutex.Lock()
	defer ls.mutex.Unlock()

	filePath := ls.getFilePath(key)
	err := os.Remove(filePath)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file %s: %w", filePath, err)
	}

	return nil
}

// List returns a list of all keys in the storage.
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
