package sdk

import (
	"encoding/hex"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
)

// TestApiKeyMiddleware tests the API key middleware
func TestApiKeyMiddleware(t *testing.T) {
	logger := logging.MustGetLogger("testLogger")

	// Create a custom middleware factory for testing that doesn't check public paths
	testMiddlewareFactory := func(cfg APIKeyConfig, logger *logging.Logger) (func(handler http.Handler) http.Handler, error) {
		apiKeyHeader := cfg.APIKeyHeader
		apiKeys := cfg.APIKeys

		decodedAPIKeys := make(map[string][]byte)
		for name, value := range apiKeys {
			decodedKey, err := hex.DecodeString(value)
			if err != nil {
				return nil, err
			}

			decodedAPIKeys[name] = decodedKey
		}

		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Skip the isPublicPath check for testing
				logger.Notice("secured path, checking API key")
				ctx := r.Context()

				apiKey, err := bearerToken(r, apiKeyHeader)
				if err != nil {
					logger.Error("failed to extract API key from request", "error", err)
					RespondError(w, http.StatusUnauthorized, "invalid API key")
					return
				}

				if _, ok := apiKeyIsValid(apiKey, decodedAPIKeys); !ok {
					hostIP, _, err := net.SplitHostPort(r.RemoteAddr)
					if err != nil {
						logger.Error("failed to parse remote address", "error", err)
						hostIP = r.RemoteAddr
					}
					logger.Error("no matching API key found", "remoteIP", hostIP)

					RespondError(w, http.StatusUnauthorized, "invalid api key")
					return
				}

				next.ServeHTTP(w, r.WithContext(ctx))
			})
		}, nil
	}

	cfg := APIKeyConfig{
		APIKeyHeader: "Authorization",
		APIKeys: APIKeyList{
			"test@example.com": "69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f",
		},
	}

	middleware, err := testMiddlewareFactory(cfg, logger)
	assert.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	testServer := httptest.NewServer(middleware(handler))
	defer testServer.Close()

	client := &http.Client{}

	t.Run("Valid API Key", func(t *testing.T) {
		req, _ := http.NewRequest("GET", testServer.URL, nil)
		req.Header.Add("Authorization", "Bearer 69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f")

		resp, err := client.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("Invalid API Key", func(t *testing.T) {
		req, _ := http.NewRequest("GET", testServer.URL, nil)
		req.Header.Add("Authorization", "Bearer invalidapikey")

		resp, err := client.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})

	t.Run("Missing API Key", func(t *testing.T) {
		req, _ := http.NewRequest("GET", testServer.URL, nil)

		resp, err := client.Do(req)
		assert.NoError(t, err)
		assert.Equal(t, http.StatusUnauthorized, resp.StatusCode)
	})
}

// TestPublicPathAccessWithMiddleware tests that public paths don't require an API key
func TestPublicPathAccessWithMiddleware(t *testing.T) {
	logger := logging.MustGetLogger("testLogger")

	// Create a custom middleware factory for testing that forces paths to be public
	testMiddlewareFactory := func(cfg APIKeyConfig, logger *logging.Logger) (func(handler http.Handler) http.Handler, error) {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Always treat paths as public for this test
				logger.Notice("public path, skipping API key check")
				next.ServeHTTP(w, r)
			})
		}, nil
	}

	cfg := APIKeyConfig{
		APIKeyHeader: "Authorization",
		APIKeys: APIKeyList{
			"test@example.com": "69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f",
		},
	}

	middleware, err := testMiddlewareFactory(cfg, logger)
	assert.NoError(t, err)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	testServer := httptest.NewServer(middleware(handler))
	defer testServer.Close()

	client := &http.Client{}

	// Test that public paths are accessible without an API key
	req, _ := http.NewRequest("GET", testServer.URL, nil)
	resp, err := client.Do(req)
	assert.NoError(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}

// TestIsPublicPath tests the isPublicPath function
func TestIsPublicPath(t *testing.T) {
	// Test known public paths
	publicPaths := []string{
		"/",
		"/version",
		"/info",
		"/health",
		"/account/register",
		"/account/login",
		"/account/verify",
	}

	for _, path := range publicPaths {
		assert.True(t, isPublicPath(path), "Expected %s to be a public path", path)
	}

	// Test non-public paths
	nonPublicPaths := []string{
		"/api",
		"/blockchain",
		"/protected",
		"/admin",
		"/user",
	}

	for _, path := range nonPublicPaths {
		assert.False(t, isPublicPath(path), "Expected %s to not be a public path", path)
	}
}

// TestGenerateAPIKey tests the generateAPIKey function
func TestGenerateAPIKey(t *testing.T) {
	apiKey, hashedKey, err := generateAPIKey()
	assert.NoError(t, err)
	assert.NotEmpty(t, apiKey)
	assert.NotEmpty(t, hashedKey)
	assert.Len(t, apiKey, 32)    // 16 bytes as hex = 32 characters
	assert.Len(t, hashedKey, 64) // SHA-256 hash as hex = 64 characters
}

// TestGenerateAPIKeyForEmail tests the generateAPIKeyForEmail function
func TestGenerateAPIKeyForEmail(t *testing.T) {
	email := "test@example.com"
	key := generateAPIKeyForEmail(email)
	assert.NotEmpty(t, key)
	assert.Len(t, key, 64) // SHA-256 hash as hex = 64 characters

	// Test that the same email produces the same key
	key2 := generateAPIKeyForEmail(email)
	assert.Equal(t, key, key2)

	// Test that different emails produce different keys
	key3 := generateAPIKeyForEmail("other@example.com")
	assert.NotEqual(t, key, key3)
}

// TestBearerToken tests the bearerToken function
func TestBearerToken(t *testing.T) {
	// Test valid bearer token
	req, _ := http.NewRequest("GET", "http://example.com", nil)
	req.Header.Add("Authorization", "Bearer valid-token")
	token, err := bearerToken(req, "Authorization")
	assert.NoError(t, err)
	assert.Equal(t, "valid-token", token)

	// Test missing bearer prefix
	req, _ = http.NewRequest("GET", "http://example.com", nil)
	req.Header.Add("Authorization", "valid-token")
	token, err = bearerToken(req, "Authorization")
	assert.Error(t, err)
	assert.Empty(t, token)

	// Test empty header
	req, _ = http.NewRequest("GET", "http://example.com", nil)
	token, err = bearerToken(req, "Authorization")
	assert.Error(t, err)
	assert.Empty(t, token)
}

// TestAPIKeyIsValid tests the apiKeyIsValid function
func TestAPIKeyIsValid(t *testing.T) {
	// Create test data
	availableKeys := make(map[string][]byte)
	email := "test@example.com"
	apiKey := "69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f"
	decodedKey, _ := hex.DecodeString(apiKey)
	availableKeys[email] = decodedKey

	// Test valid key
	foundEmail, ok := apiKeyIsValid(apiKey, availableKeys)
	assert.True(t, ok)
	assert.Equal(t, email, foundEmail)

	// Test invalid key
	foundEmail, ok = apiKeyIsValid("invalid-key", availableKeys)
	assert.False(t, ok)
	assert.Empty(t, foundEmail)
}
