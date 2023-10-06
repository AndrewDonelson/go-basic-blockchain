package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"

	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
	"github.com/op/go-logging"
)

const serverSeed = "0ebe1955e527d0a3f354315d0af97e88be3d4a499c9dacd0d947bf1bd5c71bca"

type APIKeyList map[string]string

// ErrorResponse represents the structure of an error response.
type ErrorResponse struct {
	Message string `json:"message"`
}

// User is a user
type User struct {
	ID        int
	Username  string
	FirstName string
	LastName  string
}

// Configuration structure (simplified for demonstration purposes)
type Config struct {
	APIKeyHeader string
	APIKeys      APIKeyList
}

var (
	apiKeys = APIKeyList{
		"nlaakald@gmail.com": "69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f",
	}
	logger        = logging.MustGetLogger("example")
	defaultConfig = Config{
		APIKeyHeader: "Authorization",
		APIKeys:      apiKeys,
	}
)

func main() {
	email := "nlaakald@gmail.com"
	apiKey := generateAPIKeyForEmail(email)
	fmt.Println("Generated API Key for", email, ":", apiKey)

	// Enable below code to create a new API Key
	// apiKey, hashedKey, err := generateAPIKey()
	// if err != nil {
	// 	fmt.Println("Error generating API key:", err)
	// 	return
	// }

	// fmt.Println("Generated API Key:", apiKey)
	// fmt.Println("Hashed API Key (SHA256):", hashedKey)

	// // append to apiKeys the user email (nlaakald@gmail.com) and the hashedKey
	// apiKeys["nlaakald@gmail.com"] = hashedKey

	r := mux.NewRouter()

	// Logging middleware
	r.Use(loggingMiddleware)

	// API key middleware
	apiKeyMiddleware, err := ApiKeyMiddleware(defaultConfig, *logger)
	if err != nil {
		logger.Fatal("Error initializing API key middleware:", err)
	}
	r.Use(apiKeyMiddleware)

	// Routes
	r.HandleFunc("/", homePage)

	// Testing protected page
	r.HandleFunc("/protected", protectedPage)

	fmt.Printf("Listening on :8080 ...\n")
	http.ListenAndServe(":8080", r)
	fmt.Printf("exising\n")
}

// RespondError sends an error response with the given status code and message.
func RespondError(w http.ResponseWriter, statusCode int, message string) {
	// Set the Content-Type header and status code
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	// Create an ErrorResponse instance
	errorResponse := ErrorResponse{Message: message}

	// Encode and send the error message as JSON
	json.NewEncoder(w).Encode(errorResponse)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return handlers.LoggingHandler(os.Stdout, next)
}

// ***[API Key Middleware]***
// curl -H "Authorization: Bearer 69a082ff3996745bd4b48bcc92d5bb40ff97115896183f1cb53a3409f818b15f" http://localhost:8080/protected

// ApiKeyMiddleware is a middleware that checks for a valid API key in the request header
func ApiKeyMiddleware(cfg Config, logger logging.Logger) (func(handler http.Handler) http.Handler, error) {
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
	expectedKey := generateAPIKeyForEmail("nlaakald@gmail.com") // Assuming you have this function
	logger.Info("Expected API Key:", expectedKey)
	logger.Info("RAW Key API Key:", rawKey)

	if rawKey == expectedKey {
		return "nlaakald@gmail.com", true
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

func homePage(w http.ResponseWriter, r *http.Request) {
	// Public page
	w.Write([]byte("Welcome to the public page!"))
}

func protectedPage(w http.ResponseWriter, r *http.Request) {
	userValue := r.Context().Value("user")
	if userValue == nil {
		http.Error(w, "User not found", http.StatusInternalServerError)
		return
	}

	user, ok := userValue.(User)
	if !ok {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	fmt.Fprintf(w, "Welcome, %s! You're viewing the protected page.", user.Username)
}
