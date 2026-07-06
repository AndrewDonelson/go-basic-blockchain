package services

import (
	"github.com/AndrewDonelson/go-basic-blockchain/internal/application"
)

// BlockchainService provides a stable application layer around blockchain
// operations so transports can depend on interfaces rather than SDK internals.
type BlockchainService struct {
	gateway application.BlockchainGateway
}

// BlockchainStatus is an application-layer status DTO that can be reused
// across API, CLI, and tests.
type BlockchainStatus struct {
	BlockCount  int `json:"block_count"`
	MempoolSize int `json:"mempool_size"`
}

func NewBlockchainService(gateway application.BlockchainGateway) *BlockchainService {
	return &BlockchainService{gateway: gateway}
}

func (s *BlockchainService) Status() BlockchainStatus {
	return BlockchainStatus{
		BlockCount:  s.gateway.GetBlockCount(),
		MempoolSize: s.gateway.GetMempoolSize(),
	}
}
