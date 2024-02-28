package main

import (
	"encoding/hex"
	"errors"
	"net/http"
	"strings"
)

// bearerToken extracts the content from the header, striping the Bearer prefix
func bearerToken(r *http.Request, header string) (string, error) {
	rawToken := r.Header.Get(header)
	if rawToken == "" {
		return "", errors.New("no token found in request")
	}
	pieces := strings.SplitN(rawToken, " ", 2)

	if len(pieces) < 2 {
		return "", errors.New("token with incorrect bearer format")
	}

	token := strings.TrimSpace(pieces[1])

	return token, nil
}

// apiKeyIsValid checks if the given API key is valid and returns the principal if it is.
func apiKeyIsValid(rawKey string, availableKeys map[string][]byte) (string, bool) {
	logger.Info("Validating API Key:", rawKey)

	// check and see if rawKey is in availableKeys and if so return availableKey map string
	for email, key := range availableKeys {
		if hex.EncodeToString(key) == rawKey {
			logger.Info("API Key is valid for user:", email)
			return email, true
		}
	}

	return "", false
}
