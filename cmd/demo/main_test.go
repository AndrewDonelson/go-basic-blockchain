package main

import (
	"testing"
	"time"
)

func TestDemoTransactionIDs(t *testing.T) {
	ids := demoTransactionIDs()
	if len(ids) == 0 {
		t.Fatal("expected demo transaction ids")
	}
}

func TestDemoHeliosStages(t *testing.T) {
	stages := demoHeliosStages()
	if len(stages) != 3 {
		t.Fatalf("expected 3 helios stages, got %d", len(stages))
	}
}

func TestBuildDemoStatus(t *testing.T) {
	status := buildDemoStatus(2)
	if status.BlockCount != 3 {
		t.Fatalf("expected block count 3, got %d", status.BlockCount)
	}
	if status.TxQueueSize != 6 {
		t.Fatalf("expected tx queue 6, got %d", status.TxQueueSize)
	}
	if status.Uptime != 3*time.Minute {
		t.Fatalf("expected uptime 3m, got %s", status.Uptime)
	}
}
