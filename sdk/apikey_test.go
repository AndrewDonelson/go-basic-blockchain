package sdk

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/op/go-logging"
	"github.com/stretchr/testify/assert"
)

func TestApiKeyMiddleware(t *testing.T) {
	logger := logging.MustGetLogger("testLogger")
	cfg := APIKeyConfig{
		APIKeyHeader: "Authorization",
		APIKeys: APIKeyList{
			"test@example.com": "69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f",
		},
	}

	middleware, err := ApiKeyMiddleware(cfg, logger)
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
