package sdk

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/gorilla/mux"
)

// These tests keep api/openapi.yaml honest against the router.
//
// Before the specification existed the only description of the API was a Postman
// collection, so a client author had to read the handlers -- and nothing noticed
// when a route changed.
//
// The document is read with a small structural parser rather than a YAML
// library. The module is vendored and adding a direct dependency for a test
// perturbs the dependency graph; all these tests need is the top-level path keys
// and each operation's `security` line, both of which sit at a known indent.

// specPaths returns the endpoints the specification documents, mapped to the
// operations declared under each.
func specPaths(t *testing.T) map[string][]specOperation {
	t.Helper()

	data, err := os.ReadFile(filepath.Join("..", "api", "openapi.yaml"))
	if err != nil {
		t.Fatalf("read the specification: %v", err)
	}

	paths := map[string][]specOperation{}

	var inPaths bool
	var currentPath string
	var currentOp *specOperation

	flush := func() {
		if currentPath != "" && currentOp != nil {
			paths[currentPath] = append(paths[currentPath], *currentOp)
			currentOp = nil
		}
	}

	for _, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(line, "paths:") {
			inPaths = true
			continue
		}
		if !inPaths {
			continue
		}
		// A new top-level key ends the paths section.
		if len(line) > 0 && line[0] != ' ' && line[0] != '#' {
			break
		}

		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))

		switch {
		case indent == 2 && strings.HasSuffix(trimmed, ":") && strings.HasPrefix(trimmed, "/"):
			flush()
			currentPath = strings.TrimSuffix(trimmed, ":")
			if _, seen := paths[currentPath]; !seen {
				paths[currentPath] = nil
			}
		case indent == 4 && strings.HasSuffix(trimmed, ":") && isHTTPMethod(strings.TrimSuffix(trimmed, ":")):
			flush()
			currentOp = &specOperation{Method: strings.TrimSuffix(trimmed, ":")}
		case currentOp != nil && strings.HasPrefix(trimmed, "security:"):
			currentOp.DeclaresSecurity = true
			currentOp.SecurityIsEmpty = strings.TrimSpace(strings.TrimPrefix(trimmed, "security:")) == "[]"
		}
	}
	flush()

	if len(paths) == 0 {
		t.Fatal("no paths were parsed out of api/openapi.yaml")
	}
	return paths
}

type specOperation struct {
	Method           string
	DeclaresSecurity bool
	SecurityIsEmpty  bool
}

func isHTTPMethod(s string) bool {
	switch s {
	case "get", "post", "put", "patch", "delete", "head", "options":
		return true
	}
	return false
}

// registeredRoutes walks the real router.
func registeredRoutes(t *testing.T) map[string]bool {
	t.Helper()

	bc := forkTestChain(t, 0, uint32(genesisDifficulty))
	api := NewAPI(bc)
	if api == nil {
		t.Fatal("failed to create the API")
	}

	routes := map[string]bool{}
	err := api.router.Walk(func(route *mux.Route, _ *mux.Router, _ []*mux.Route) error {
		template, err := route.GetPathTemplate()
		if err != nil {
			return nil // a route with no path template, e.g. bare middleware
		}
		// A subrouter mount point carries no methods and is not an endpoint;
		// documenting "/consensus" would describe something nothing answers.
		if methods, err := route.GetMethods(); err != nil || len(methods) == 0 {
			return nil
		}
		routes[template] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}
	return routes
}

// normalisePath makes gorilla/mux and OpenAPI path templates comparable.
//
// mux happily writes the same variable name twice ({id} in two segments);
// OpenAPI cannot, since a document with two identically named path parameters is
// invalid. Comparing the shape rather than the names keeps the two in step
// without pretending they name things the same way.
var pathVariable = regexp.MustCompile(`\{[^}]+\}`)

func normalisePath(path string) string {
	normalised := pathVariable.ReplaceAllString(path, "{}")
	if normalised != "/" {
		normalised = strings.TrimSuffix(normalised, "/")
	}
	return normalised
}

// TestOpenAPISpecCoversEveryRoute: an undocumented endpoint is one no client
// author knows exists.
func TestOpenAPISpecCoversEveryRoute(t *testing.T) {
	documented := map[string]bool{}
	for path := range specPaths(t) {
		documented[normalisePath(path)] = true
	}

	var undocumented []string
	for route := range registeredRoutes(t) {
		if !documented[normalisePath(route)] {
			undocumented = append(undocumented, route)
		}
	}

	sort.Strings(undocumented)
	if len(undocumented) > 0 {
		t.Fatalf("these routes are served but absent from api/openapi.yaml: %v",
			undocumented)
	}
}

// TestOpenAPISpecDescribesNoPhantomRoutes is the other direction: a documented
// endpoint that is not served sends a client after a guaranteed 404.
func TestOpenAPISpecDescribesNoPhantomRoutes(t *testing.T) {
	served := map[string]bool{}
	for route := range registeredRoutes(t) {
		served[normalisePath(route)] = true
	}

	var phantom []string
	for path := range specPaths(t) {
		if !served[normalisePath(path)] {
			phantom = append(phantom, path)
		}
	}

	sort.Strings(phantom)
	if len(phantom) > 0 {
		t.Fatalf("api/openapi.yaml documents endpoints that are not served: %v", phantom)
	}
}

// TestOpenAPIPublicPathsMatchTheCode: marking an authenticated path as public
// tells a client it needs no key, and it will get a 401 instead.
func TestOpenAPIPublicPathsMatchTheCode(t *testing.T) {
	paths := specPaths(t)

	for _, public := range publicPaths {
		operations, ok := paths[public]
		if !ok {
			t.Fatalf("%s is a public path in the code but is absent from the specification",
				public)
		}
		if len(operations) == 0 {
			t.Fatalf("%s is documented with no operations", public)
		}

		for _, op := range operations {
			if !op.DeclaresSecurity {
				t.Fatalf("%s %s is public in the code but inherits the global security "+
					"requirement in the specification", op.Method, public)
			}
			if !op.SecurityIsEmpty {
				t.Fatalf("%s %s should declare `security: []`", op.Method, public)
			}
		}
	}
}

// TestOpenAPIProtectedPathsRequireAuth is the converse: an authenticated
// endpoint that the document marks public.
func TestOpenAPIProtectedPathsRequireAuth(t *testing.T) {
	public := map[string]bool{}
	for _, path := range publicPaths {
		public[path] = true
	}

	for path, operations := range specPaths(t) {
		if public[path] {
			continue
		}
		for _, op := range operations {
			if op.DeclaresSecurity && op.SecurityIsEmpty {
				t.Fatalf("%s %s is documented as needing no credential, but it is not "+
					"a public path in the code", op.Method, path)
			}
		}
	}
}
