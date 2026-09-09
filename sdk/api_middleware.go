// Package sdk is a software development kit for building blockchain applications.
// File sdk/api_middleware.go - HTTP middleware: authentication, logging, public paths.
package sdk

import (
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"
)

var publicPaths = []string{
	"/",
	"/version",
	"/info",
	"/health",
	"/metrics",
	"/account/register",
	"/account/login",
	"/account/verify",
}

// installMiddleware attaches logging and API key authentication to the router.
func (api *API) installMiddleware() error {
	keyCfg := defaultAPIKeyConfig()
	keyCfg.accountStore = api.accountStore

	api.router.Use(loggingMiddleware)

	apiKeyMiddleware, err := ApiKeyMiddleware(keyCfg, api.log)
	if err != nil {
		return fmt.Errorf("error initializing API key middleware: %w", err)
	}
	api.router.Use(apiKeyMiddleware)
	return nil
}

func isPublicPath(path string) bool {
	// Every route is mounted twice, so a path is public under either mount.
	// Matching the literal string alone would have left /v1/health demanding a
	// credential while /health did not -- the same endpoint, two answers.
	candidates := []string{path}
	if trimmed := strings.TrimPrefix(path, apiVersionPrefix); trimmed != path {
		if trimmed == "" {
			trimmed = "/"
		}
		candidates = append(candidates, trimmed)
	}

	for _, candidate := range candidates {
		for _, publicPath := range publicPaths {
			if candidate == publicPath {
				return true
			}
		}
	}
	return false
}

// authenticateNode guards the /consensus endpoints.
//
// With no key configured this now refuses every request rather than relying on a
// published fallback key. Comparisons are constant-time and attempts are rate
// limited per source address.
func authenticateNode(next http.Handler) http.Handler {
	cfg := defaultAPIKeyConfig()
	decodedAPIKeys := make(map[string][]byte)

	for name, value := range cfg.APIKeys {
		decodedKey, err := hex.DecodeString(value)
		if err != nil {
			LogInfof("invalid consensus API key configuration for %s: %v", name, err)
			continue
		}

		decodedAPIKeys[name] = decodedKey
	}

	limiter := newRateLimiter(authRateLimitAttempts, authRateLimitWindow)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if len(decodedAPIKeys) == 0 {
			RespondError(w, http.StatusServiceUnavailable,
				"consensus authentication unavailable: no API key configured")
			return
		}

		host := clientIP(r)
		if !limiter.allow(host) {
			RespondError(w, http.StatusTooManyRequests, "too many authentication attempts")
			return
		}

		apiKey, err := bearerToken(r, cfg.APIKeyHeader)
		if err != nil {
			RespondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		principal, ok := apiKeyIsValid(apiKey, decodedAPIKeys)
		if !ok {
			RespondError(w, http.StatusUnauthorized, "unauthorized")
			return
		}

		limiter.reset(host)
		next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principal)))
	})
}

// Logging middleware logs the request and response details
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Get the user's remote IP address
		ip := GetUserIP(r)

		// Log the request details
		cacheReqStr := fmt.Sprintf("[%s] Request: %s %s %s", time.Now().Format(logDateTimeFormat), ip, r.Method, r.URL.Path)

		// Create a response writer wrapper to capture the response status code
		ww := &responseWriterWrapper{ResponseWriter: w}

		// Call the next handler
		next.ServeHTTP(ww, r)

		// Log the response details
		//api.logger.Printf("%s -> Response: %d %d bytes", cacheReqStr, ww.statusCode, ww.bytesWritten)
		LogVerbosef("%s -> Response: %d %d bytes", cacheReqStr, ww.statusCode, ww.bytesWritten)
	})
}

// responseWriterWrapper is a wrapper around http.ResponseWriter to capture the response status code.
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode   int
	bytesWritten int
}

func (ww *responseWriterWrapper) WriteHeader(code int) {
	ww.statusCode = code
	ww.ResponseWriter.WriteHeader(code)
}

func (ww *responseWriterWrapper) Write(data []byte) (int, error) {
	n, err := ww.ResponseWriter.Write(data)
	ww.bytesWritten += n
	return n, err
}
