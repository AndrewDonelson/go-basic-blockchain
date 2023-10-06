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

type APIKeyList map[string]string

// Configuration structure (simplified for demonstration purposes)
type APIKeyConfig struct {
	APIKeyHeader string
	APIKeys      APIKeyList
}

var (
	apiKeys = APIKeyList{
		"nlaakald@gmail.com": "69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f",
	}
	defaulAPIKeytConfig = APIKeyConfig{
		APIKeyHeader: "Authorization",
		APIKeys:      apiKeys,
	}
)

// ***[API Key Middleware]***
// curl -H "Authorization: Bearer 69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f" http://localhost:8080/protected

// ApiKeyMiddleware is a middleware that checks for a valid API key in the request header
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

// bearerToken extracts the content from the header, striping the Bearer prefix
func bearerToken(r *http.Request, header string) (string, error) {
	rawToken := r.Header.Get(header)
	pieces := strings.SplitN(rawToken, " ", 2)

	if len(pieces) < 2 {
		return "", errors.New("token with incorrect bearer format")
	}

	token := strings.TrimSpace(pieces[1])

	return token, nil
}

// apiKeyIsValid checks if the given API key is valid and returns the principal if it is.
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

// matchAPIKeyToEmail() checks if the given API key is valid and returns the principal if it is.
func matchAPIKeyToEmail(rawKey string, availableKeys map[string][]byte) (string, bool) {
	for email, key := range availableKeys {
		if rawKey == hex.EncodeToString(key) {
			return email, true
		}
	}

	return "", false
}

// generateAPIKey creates a new random API key and its SHA256 hashed version.
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

// generateAPIKeyForEmail generates an API key using the server's seed and the provided email.
func generateAPIKeyForEmail(email string) string {
	// Combine the server seed and the email
	combined := serverSeed + email

	// Hash the combined string
	hash := sha256.Sum256([]byte(combined))

	// Return the hex representation of the hash as the API key
	return hex.EncodeToString(hash[:])
}
