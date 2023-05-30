package sdk

// Transaction is an interface that defines the Processes for the different types of protocol transactions.
type Transaction interface {
	Process() string
}

// Tx is a transaction that represents a generic transaction.
type Tx struct {
	From *Wallet
	To   *Wallet
	Fee  float64
}
