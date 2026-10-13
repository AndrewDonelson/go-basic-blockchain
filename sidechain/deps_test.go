// File sidechain/deps_test.go - The package stays importable without the node.
package sidechain

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoNodeDependencies fails if this package (or anything it imports) pulls in
// a third-party module or the node: applications import it precisely so they
// don't link mux, the spinner, the p2p stack or any other node dependency.
func TestNoNodeDependencies(t *testing.T) {
	goBin, err := exec.LookPath("go")
	if err != nil {
		t.Skip("no go tool")
	}
	out, err := exec.Command(goBin, "list", "-deps", "-f", "{{if not .Standard}}{{.ImportPath}}{{end}}", ".").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	allowed := map[string]bool{
		"github.com/AndrewDonelson/go-basic-blockchain/sidechain":       true,
		"github.com/AndrewDonelson/go-basic-blockchain/internal/merkle": true,
	}
	for _, dep := range strings.Fields(string(out)) {
		if !allowed[dep] {
			t.Errorf("sidechain must depend only on the standard library; it imports %s", dep)
		}
	}
}
