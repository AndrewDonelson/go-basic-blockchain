package main

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"

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

type TokenData struct {
	Key  string
	User User
}

// Configuration structure (simplified for demonstration purposes)
type Config struct {
	APIKeyHeader string
	APIKeys      APIKeyList
	Users        []User
}

var (
	apiKeys = APIKeyList{
		"nlaakald@gmail.com": "0065db4b04d8969ad21109084d8e9038444a06692a7625373012c5fb7b1cd131",
	}

	logger        = logging.MustGetLogger("example")
	defaultConfig = Config{
		APIKeyHeader: "Authorization",
		APIKeys:      apiKeys,
	}

	r = mux.NewRouter()
)

func main() {
	defaultConfig.Users = append(defaultConfig.Users, User{ID: 1, Username: "jsmith", Email: "nlaakald@gmail.com", FirstName: "John", Password: "Pa$$w0rD!"})
	apiKey := defaultConfig.Users[0].NewAPIKey()
	fmt.Println("Generated API Key for", defaultConfig.Users[0].Email, ":", apiKey)

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
// curl -H "Authorization: Bearer 0065db4b04d8969ad21109084d8e9038444a06692a7625373012c5fb7b1cd131" http://localhost:8080/protected
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

			email, ok := apiKeyIsValid(apiKey, decodedAPIKeys)
			// Check if the API key is valid
			if !ok {
				hostIP, _, err := net.SplitHostPort(r.RemoteAddr)
				if err != nil {
					logger.Error("failed to parse remote address", "error", err)
					hostIP = r.RemoteAddr
				}
				logger.Error("no matching API key found", "remoteIP", hostIP)

				RespondError(w, http.StatusUnauthorized, "invalid api key")
				return
			}

			logger.Info("API key for is valid")
			ctx = context.WithValue(r.Context(), "user", email)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}, nil
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

	fmt.Fprintf(w, "Welcome, %s! You're viewing the protected page\n\n", userValue.(string))
}
