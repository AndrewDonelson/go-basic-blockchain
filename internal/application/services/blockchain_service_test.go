package services

import "testing"

type mockGateway struct {
	blocks  int
	mempool int
}

func (m mockGateway) GetBlockCount() int  { return m.blocks }
func (m mockGateway) GetMempoolSize() int { return m.mempool }

func TestNewBlockchainServiceAndStatus(t *testing.T) {
	svc := NewBlockchainService(mockGateway{blocks: 7, mempool: 3})
	if svc == nil {
		t.Fatal("expected non-nil service")
	}

	status := svc.Status()
	if status.BlockCount != 7 {
		t.Fatalf("expected block count 7, got %d", status.BlockCount)
	}
	if status.MempoolSize != 3 {
		t.Fatalf("expected mempool size 3, got %d", status.MempoolSize)
	}
}

func TestStatusZeroValues(t *testing.T) {
	svc := NewBlockchainService(mockGateway{})
	status := svc.Status()
	if status.BlockCount != 0 || status.MempoolSize != 0 {
		t.Fatalf("expected zero status, got %+v", status)
	}
}
