// Package sdk is a software development kit for building blockchain applications.
// File sdk/apikey.go - API key middleware
package sdk

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/http"
	"strings"

	"github.com/op/go-logging"
	//"gorm.io/gorm/logger"
)

// APIKeyList is a map of API keys to their corresponding values.
// This type is used to store and manage API keys.
type APIKeyList map[string]string

// APIKeyConfig is a configuration struct that holds the API key header name and a map of API keys.
// The APIKeys field is a map of API keys to their corresponding values, used to store and manage API keys.
type APIKeyConfig struct {
	APIKeyHeader string
	APIKeys      APIKeyList
}

var (
	// apiKeys is a map of API keys to their corresponding values, used to store and manage API keys.
	apiKeys = APIKeyList{
		// Not real, used for demo/testing/protoyping
		"nlaakald@gmail.com": "69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f",
	}

	// defaulAPIKeytConfig is a configuration struct that holds the API key header name and a map of API keys.
	// The APIKeys field is a map of API keys to their corresponding values, used to store and manage API keys.
	defaulAPIKeytConfig = APIKeyConfig{
		APIKeyHeader: "Authorization",
		APIKeys:      apiKeys,
	}
)

// ***[API Key Middleware]***
// curl -H "Authorization: Bearer 69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f" http://localhost:8080/protected

// ApiKeyMiddleware is a middleware function that checks for a valid API key in the request header.
// It takes an APIKeyConfig and a logging.Logger as input, and returns a middleware function
// that can be used to wrap an http.Handler.
//
// The middleware first checks if the requested path is public (using the isPublicPath function),
// and if so, skips the API key check and calls the next handler.
//
// If the path is secured, the middleware extracts the API key from the request header using the
// bearerToken function, and then checks if the API key is valid by looking it up in the
// decodedAPIKeys map. If the API key is not valid, the middleware responds with a 401 Unauthorized
// error.
//
// If the API key is valid, the middleware calls the next handler with the original request context.
func ApiKeyMiddleware(cfg APIKeyConfig, logger *logging.Logger) (func(handler http.Handler) http.Handler, error) {
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
			// If the path is public, skip the middleware checks
			if isPublicPath(r.URL.Path) {
				logger.Notice("public path, skipping API key check")
				next.ServeHTTP(w, r)
				return
			}

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

// apiKeyIsValid checks if the given API key is valid and returns the principal (email address) if it is.
// The function takes a raw API key string and a map of available API keys (where the keys are email addresses
// and the values are the corresponding API keys). It attempts to match the raw API key against the available
// keys, and if a match is found, it returns the email address and true. Otherwise, it returns an empty
// string and false.
func apiKeyIsValid(rawKey string, availableKeys map[string][]byte) (string, bool) {
	//expectedKey := generateAPIKeyForEmail("nlaakald@gmail.com") // Assuming you have this function
	//logger.Info("Expected API Key:", expectedKey)
	fmt.Printf("RAW Key API Key: %s", rawKey)

	email, ok := matchAPIKeyToEmail(rawKey, availableKeys)
	if ok {
		return email, true
	}

	return "", false
}

// matchAPIKeyToEmail checks if the given API key is valid and returns the principal (email address) if it is.
// The function takes a raw API key string and a map of available API keys (where the keys are email addresses
// and the values are the corresponding API keys). It attempts to match the raw API key against the available
// keys, and if a match is found, it returns the email address and true. Otherwise, it returns an empty
// string and false.
func matchAPIKeyToEmail(rawKey string, availableKeys map[string][]byte) (string, bool) {
	for email, key := range availableKeys {
		if rawKey == hex.EncodeToString(key) {
			return email, true
		}
	}

	return "", false
}

// generateAPIKey creates a new random API key and its SHA256 hashed version.
// It generates a 16-byte random API key, converts it to a hexadecimal string,
// and then hashes the API key using SHA256. It returns the API key, the hashed
// key, and any error that occurred during the process.
func generateAPIKey() (apiKey string, hashedKey string, err error) {
	// Generate a random 16-byte API key.
	randomBytes := make([]byte, 16)
	_, err = rand.Read(randomBytes)
	if err != nil {
		return "", "", err
	}

	// Convert the random bytes to a hexadecimal string to get the API key.
	apiKey = hex.EncodeToString(randomBytes)

	// Hash the API key using SHA256.
	hash := sha256.Sum256([]byte(apiKey))
	hashedKey = hex.EncodeToString(hash[:])

	return apiKey, hashedKey, nil
}

// generateAPIKeyForEmail generates an API key by combining the server's seed and the provided email address,
// hashing the combined string using SHA256, and returning the hex representation of the hash as the API key.
//
// The generated API key is intended to be associated with the provided email address.
func generateAPIKeyForEmail(email string) string {
	// Combine the server seed and the email
	combined := serverSeed + email

	// Hash the combined string
	hash := sha256.Sum256([]byte(combined))

	// Return the hex representation of the hash as the API key
	return hex.EncodeToString(hash[:])
}
