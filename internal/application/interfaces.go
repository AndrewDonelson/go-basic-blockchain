package application

// BlockchainGateway defines the minimum application-facing contract for
// blockchain operations used by services and transport adapters.
type BlockchainGateway interface {
	GetBlockCount() int
	GetMempoolSize() int
}

// NodeGateway defines application-facing node operations needed by
// transport and orchestration adapters.
type NodeGateway interface {
	IsReady() bool
	GetStatus() string
}

// AuthGateway defines application-facing auth concerns for API and CLI paths.
type AuthGateway interface {
	ValidateAPIKey(rawKey string) (principal string, ok bool)
}
