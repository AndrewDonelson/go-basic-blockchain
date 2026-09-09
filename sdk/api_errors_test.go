package sdk

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestErrorsAreAlwaysJSON: handlers used to mix the JSON envelope with
// http.Error's plain text, so a client had to guess which it had received --
// and the choice varied by handler rather than by error kind.
func TestErrorsAreAlwaysJSON(t *testing.T) {
	recorder := httptest.NewRecorder()
	RespondError(recorder, http.StatusBadRequest, "something was wrong")

	if got := recorder.Header().Get("Content-Type"); !strings.Contains(got, "application/json") {
		t.Fatalf("Content-Type is %q, want application/json", got)
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status is %d, want 400", recorder.Code)
	}

	var body ErrorResponse
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatalf("the error body is not valid JSON: %v (%q)", err, recorder.Body.String())
	}
	if body.Message != "something was wrong" {
		t.Fatalf("message = %q", body.Message)
	}
	// A client holding only the payload -- a log line, a queued webhook -- cannot
	// otherwise tell a 400 from a 500.
	if body.Status != http.StatusBadRequest {
		t.Fatalf("status in body = %d, want 400", body.Status)
	}
}

// TestAPIRejectionsUseTheEnvelope drives real handlers rather than the helper, so
// it catches a handler that still writes plain text.
func TestAPIRejectionsUseTheEnvelope(t *testing.T) {
	bc := forkTestChain(t, 0, uint32(genesisDifficulty))
	api := NewAPI(bc)
	if api == nil {
		t.Fatal("failed to create the API")
	}

	cases := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"unauthenticated request", http.MethodGet, "/blockchain", ""},
		{"malformed register payload", http.MethodPost, "/account/register", "{not json"},
		{"malformed login payload", http.MethodPost, "/account/login", "{not json"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var reader *strings.Reader
			if tc.body != "" {
				reader = strings.NewReader(tc.body)
			} else {
				reader = strings.NewReader("")
			}

			request := httptest.NewRequest(tc.method, tc.path, reader)
			recorder := httptest.NewRecorder()
			api.router.ServeHTTP(recorder, request)

			if recorder.Code < 400 {
				t.Skipf("this request was not rejected (status %d); nothing to check",
					recorder.Code)
			}

			body := strings.TrimSpace(recorder.Body.String())
			if body == "" {
				t.Fatalf("a %d response carried no body at all", recorder.Code)
			}

			var envelope ErrorResponse
			if err := json.Unmarshal([]byte(body), &envelope); err != nil {
				t.Fatalf("a %d response is not JSON: %q", recorder.Code, body)
			}
			if envelope.Message == "" {
				t.Fatalf("a %d response carried an empty message: %q", recorder.Code, body)
			}
		})
	}
}

// TestMetricsEndpointIsPublicAndWellFormed covers the observability endpoint end
// to end through the router.
func TestMetricsEndpointIsPublicAndWellFormed(t *testing.T) {
	bc := forkTestChain(t, 2, uint32(genesisDifficulty))
	api := NewAPI(bc)
	if api == nil {
		t.Fatal("failed to create the API")
	}

	request := httptest.NewRequest(http.MethodGet, "/metrics", nil)
	recorder := httptest.NewRecorder()
	api.router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusOK {
		t.Fatalf("/metrics returned %d; a metrics endpoint behind authentication "+
			"is one no scraper will be configured for", recorder.Code)
	}

	body := recorder.Body.String()
	for _, want := range []string{"gbb_chain_height", "gbb_blocks_accepted", "# TYPE"} {
		if !strings.Contains(body, want) {
			t.Fatalf("/metrics output is missing %q:\n%s", want, body)
		}
	}

	// The chain height gauge must reflect the actual chain.
	if !strings.Contains(body, "gbb_chain_height 2") {
		t.Fatalf("/metrics reports the wrong chain height:\n%s", body)
	}
}

// TestRouterIsAuthenticatedWithoutStart pins where middleware belongs.
//
// Authentication used to be attached in Start(), so api.router on its own was an
// unauthenticated API: anything serving the router directly -- an embedder, or a
// test harness wiring it into its own http.Server -- got no authentication at
// all. That is a security property of the router, not of the listener.
func TestRouterIsAuthenticatedWithoutStart(t *testing.T) {
	bc := forkTestChain(t, 0, uint32(genesisDifficulty))
	api := NewAPI(bc)
	if api == nil {
		t.Fatal("failed to create the API")
	}
	if api.IsRunning() {
		t.Fatal("the API reports running before Start")
	}

	protected := []string{"/blockchain", "/blockchain/blocks", "/wallets"}
	for _, path := range protected {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		api.router.ServeHTTP(recorder, request)

		if recorder.Code == http.StatusOK {
			t.Fatalf("%s answered an unauthenticated request with 200 before Start; "+
				"serving api.router must not bypass authentication", path)
		}
	}

	// Public paths stay public.
	for _, path := range []string{"/health", "/metrics", "/version"} {
		request := httptest.NewRequest(http.MethodGet, path, nil)
		recorder := httptest.NewRecorder()
		api.router.ServeHTTP(recorder, request)

		if recorder.Code == http.StatusUnauthorized {
			t.Fatalf("%s requires authentication, but it is a public path", path)
		}
	}
}

// TestVersionedAndLegacyMountsBehaveIdentically: an endpoint that answers
// differently depending on which mount you call is worse than no versioning.
func TestVersionedAndLegacyMountsBehaveIdentically(t *testing.T) {
	bc := forkTestChain(t, 2, uint32(genesisDifficulty))
	api := NewAPI(bc)
	if api == nil {
		t.Fatal("failed to create the API")
	}

	for _, path := range []string{"/health", "/metrics", "/version"} {
		legacy := httptest.NewRecorder()
		api.router.ServeHTTP(legacy, httptest.NewRequest(http.MethodGet, path, nil))

		versioned := httptest.NewRecorder()
		api.router.ServeHTTP(versioned, httptest.NewRequest(http.MethodGet, "/v1"+path, nil))

		if legacy.Code != versioned.Code {
			t.Fatalf("%s returned %d but /v1%s returned %d",
				path, legacy.Code, path, versioned.Code)
		}
		if legacy.Code != http.StatusOK {
			t.Fatalf("%s returned %d, want 200", path, legacy.Code)
		}
	}

	// A protected path is protected under both.
	for _, path := range []string{"/blockchain", "/v1/blockchain"} {
		recorder := httptest.NewRecorder()
		api.router.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, path, nil))
		if recorder.Code == http.StatusOK {
			t.Fatalf("%s answered an unauthenticated request with 200", path)
		}
	}
}
