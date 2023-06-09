// Package sdk is a software development kit for building blockchain applications.
// File sdk/persisttx.go - Persistance Transaction for all On Chain Storage related Protocol based transactions
package sdk

// Persist is a transaction protocol for storing key/value pairs on the blockchain with indexing support.
type Persist struct {
	Tx
	Data map[string]string // Key/value pairs to be stored
}

// NewPersistTransaction creates a new Persist transaction.
func NewPersistTransaction(from *Wallet, to *Wallet, fee float64, data map[string]string) (*Persist, error) {
	tx, err := NewTransaction(PersistProtocolID, from, to)
	if err != nil {
		return nil, err
	}

	return &Persist{
		Tx:   *tx,
		Data: data,
	}, nil
}

// Process processes the Persist transaction.
func (p *Persist) Process() string {
	// Process the Persist transaction logic here
	// This may involve storing the key/value pairs on the blockchain

	p.Status = "processed"
	return "Persist transaction processed successfully"
}
