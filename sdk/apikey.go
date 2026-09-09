// Package sdk is a software development kit for building blockchain applications.
// File sdk/apikey.go - API key middleware
package sdk

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	logging "github.com/op/go-logging"
)

// APIKeyList is a map of principal (email address) to their hex-encoded API key.
type APIKeyList map[string]string

// APIKeyConfig holds the API key header name and the set of accepted API keys.
type APIKeyConfig struct {
	APIKeyHeader string
	APIKeys      APIKeyList
	// accountStore, when set, allows keys issued to verified accounts to
	// authenticate in addition to the statically configured keys.
	accountStore AccountStore
}

const (
	// authRateLimitAttempts / authRateLimitWindow throttle credential guessing.
	authRateLimitAttempts = 10
	authRateLimitWindow   = time.Minute

	// These are the NAMES of environment variables, not values. The credentials
	// themselves have no defaults at all -- authentication fails closed, which is
	// the whole point of the change that removed the built-in demo key.
	envAPIKeyHeader       = "API_KEY_HEADER"     //nolint:gosec // G101: a variable name, not a credential
	envBlockchainAPIKey   = "BLOCKCHAIN_API_KEY" //nolint:gosec // G101: a variable name, not a credential
	envBlockchainAPIEmail = "BLOCKCHAIN_API_EMAIL"
	envServerSeed         = "BLOCKCHAIN_SERVER_SEED" //nolint:gosec // G101: a variable name, not a credential
)

// ErrNoAPIKeyConfigured is returned when the node has no usable API credentials.
//
// The node used to fall back to a demo key and server seed that were hardcoded in
// this file and published in the repository, so any deployment that did not set
// the environment variables accepted a key every reader of the source already
// knew -- a complete authentication bypass, including on /consensus/*. There is
// no fallback any more: with nothing configured, authentication fails closed.
var ErrNoAPIKeyConfigured = errors.New(
	"no API key configured: set BLOCKCHAIN_API_KEY (hex-encoded) to enable authenticated endpoints")

// defaultAPIKeyConfig builds API auth settings from the environment.
// It returns a config with an empty key set when nothing is configured; callers
// must treat that as "authentication unavailable", never as "allow everything".
func defaultAPIKeyConfig() APIKeyConfig {
	header := getEnv(envAPIKeyHeader, "Authorization")
	principal := getEnv(envBlockchainAPIEmail, "local-dev")
	apiKey := getEnv(envBlockchainAPIKey, "")

	keys := APIKeyList{}
	if apiKey != "" {
		keys[principal] = apiKey
	}

	return APIKeyConfig{
		APIKeyHeader: header,
		APIKeys:      keys,
	}
}

// configuredServerSeed returns the server seed used to derive per-account values.
// It has no default: a published constant seed let anyone mint valid credentials.
func configuredServerSeed() (string, error) {
	seed := getEnv(envServerSeed, "")
	if seed == "" {
		return "", errors.New("no server seed configured: set BLOCKCHAIN_SERVER_SEED")
	}
	return seed, nil
}

// APIKeyMiddleware returns middleware that authenticates requests by API key.
//
// Public paths bypass the check. Every other path requires a bearer token that
// matches either a configured key or a key issued to a verified account. All
// comparisons are constant-time.
func APIKeyMiddleware(cfg APIKeyConfig, logger *logging.Logger) (func(handler http.Handler) http.Handler, error) {
	apiKeyHeader := cfg.APIKeyHeader
	if apiKeyHeader == "" {
		apiKeyHeader = "Authorization"
	}

	decodedAPIKeys := make(map[string][]byte)
	for name, value := range cfg.APIKeys {
		decodedKey, err := hex.DecodeString(value)
		if err != nil {
			return nil, err
		}

		decodedAPIKeys[name] = decodedKey
	}

	if len(decodedAPIKeys) == 0 && cfg.accountStore == nil {
		return nil, ErrNoAPIKeyConfigured
	}

	limiter := newRateLimiter(authRateLimitAttempts, authRateLimitWindow)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// If the path is public, skip the middleware checks
			if isPublicPath(r.URL.Path) {
				next.ServeHTTP(w, r)
				return
			}

			hostIP := clientIP(r)
			if !limiter.allow(hostIP) {
				RespondError(w, http.StatusTooManyRequests, "too many authentication attempts")
				return
			}

			apiKey, err := bearerToken(r, apiKeyHeader)
			if err != nil {
				logger.Error("failed to extract API key from request", "error", err)
				RespondError(w, http.StatusUnauthorized, "invalid API key")
				return
			}

			principal, ok := apiKeyIsValid(apiKey, decodedAPIKeys)
			if !ok && cfg.accountStore != nil {
				principal, ok = accountKeyIsValid(apiKey, cfg.accountStore)
			}
			if !ok {
				logger.Error("no matching API key found", "remoteIP", hostIP)
				RespondError(w, http.StatusUnauthorized, "invalid api key")
				return
			}

			limiter.reset(hostIP)
			next.ServeHTTP(w, r.WithContext(withPrincipal(r.Context(), principal)))
		})
	}, nil
}

// clientIP returns the address to rate-limit on. It deliberately ignores
// X-Forwarded-For: that header is attacker-controlled, so trusting it here would
// let a caller sidestep the limiter by varying one header.
func clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

// bearerToken extracts the token from the Authorization header, assuming the header
// is in the "Bearer <token>" format. It returns the token string, or an error if
// the header is not in the expected format.
func bearerToken(r *http.Request, header string) (string, error) {
	rawToken := r.Header.Get(header)
	pieces := strings.SplitN(rawToken, " ", 2)

	if len(pieces) < 2 {
		return "", errors.New("token with incorrect bearer format")
	}

	token := strings.TrimSpace(pieces[1])

	return token, nil
}

// apiKeyIsValid checks a raw API key against the configured keys and returns the
// matching principal.
func apiKeyIsValid(rawKey string, availableKeys map[string][]byte) (string, bool) {
	return matchAPIKeyToEmail(rawKey, availableKeys)
}

// matchAPIKeyToEmail compares the supplied key against every configured key in
// constant time.
//
// This used to be `rawKey == hex.EncodeToString(key)`. Go's string comparison
// short-circuits on the first differing byte, which leaks the key one byte at a
// time to an attacker who can measure response latency. The loop below always
// examines every candidate and every byte.
func matchAPIKeyToEmail(rawKey string, availableKeys map[string][]byte) (string, bool) {
	rawBytes, err := hex.DecodeString(rawKey)
	if err != nil {
		// Still walk the candidates so a malformed key is not distinguishable by
		// timing from a well-formed but wrong one.
		rawBytes = nil
	}

	matchedEmail := ""
	found := 0
	for email, key := range availableKeys {
		if subtle.ConstantTimeCompare(rawBytes, key) == 1 {
			matchedEmail = email
			found = 1
		}
	}

	if found == 1 {
		return matchedEmail, true
	}
	return "", false
}

// accountKeyIsValid checks a raw API key against the keys issued to verified
// accounts. Only the SHA-256 hash of an issued key is ever stored.
func accountKeyIsValid(rawKey string, store AccountStore) (string, bool) {
	if store == nil || rawKey == "" {
		return "", false
	}

	record, ok, err := store.GetVerifiedByAPIKeyHash(hashAPIKey(rawKey))
	if err != nil || !ok {
		return "", false
	}
	return record.Email, true
}

// hashAPIKey returns the hex-encoded SHA-256 of an API key. Only this value is
// persisted, so a leaked account store does not yield usable credentials.
func hashAPIKey(rawKey string) string {
	sum := sha256.Sum256([]byte(rawKey))
	return hex.EncodeToString(sum[:])
}

// generateAPIKey creates a new random API key and its SHA-256 hashed version.
func generateAPIKey() (apiKey string, hashedKey string, err error) {
	// Generate a random 32-byte API key (256 bits).
	randomBytes := make([]byte, 32)
	if _, err = rand.Read(randomBytes); err != nil {
		return "", "", err
	}

	apiKey = hex.EncodeToString(randomBytes)
	return apiKey, hashAPIKey(apiKey), nil
}

// rateLimiter is a small fixed-window limiter used to slow credential guessing.
type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*rateLimitEntry
	limit    int
	window   time.Duration
}

type rateLimitEntry struct {
	count      int
	windowFrom time.Time
}

func newRateLimiter(limit int, window time.Duration) *rateLimiter {
	return &rateLimiter{
		attempts: make(map[string]*rateLimitEntry),
		limit:    limit,
		window:   window,
	}
}

// allow records an attempt for key and reports whether it is within the limit.
func (rl *rateLimiter) allow(key string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	entry, ok := rl.attempts[key]
	if !ok || now.Sub(entry.windowFrom) > rl.window {
		rl.attempts[key] = &rateLimitEntry{count: 1, windowFrom: now}
		return true
	}

	entry.count++
	return entry.count <= rl.limit
}

// reset clears the counter for key, so successful callers are never throttled.
func (rl *rateLimiter) reset(key string) {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, key)
}
