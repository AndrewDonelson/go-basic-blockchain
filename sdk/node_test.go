// Package sdk is a software development kit for building blockchain applications.
// File  sdk/node_test.go - Node Test for all Node related Protocol based transactions
package sdk

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNodeRun(t *testing.T) {
	node := NewNode()

	// Test running the node without enabling the API
	node.Config.EnableAPI = false
	go node.Run()

	// Let the node run for a short duration
	// You can add more specific tests here if needed
	sleepDuration := 1 * time.Second
	time.Sleep(sleepDuration)

	// Ensure that the node is still running
	assert.Equal(t, false, node.API.IsRunning())

	// Test running the node with the API enabled
	node.Config.EnableAPI = true
	go node.Run()

	// Let the node run for a short duration
	// You can add more specific tests here if needed
	time.Sleep(sleepDuration)

	// Ensure that the node API is running
	assert.Equal(t, true, node.API.IsRunning())
}

func TestNodeRunCoverage(t *testing.T) {
	// Test additional scenarios here to achieve full code coverage
	// For example, you can test different configurations and edge cases
	// to ensure that all branches of the code are exercised
}
