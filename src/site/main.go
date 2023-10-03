package main

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/dgrijalva/jwt-go"
	"github.com/gorilla/handlers"
	"github.com/gorilla/mux"
)

var mySigningKey = []byte("6873uaj9837fhk264al0973kf765kjh")

// User is a user
type User struct {
	ID        int
	Username  string
	FirstName string
	LastName  string
}

type contextKey string

const userIDKey contextKey = "userid"

func main() {

	r := mux.NewRouter()

	// Logging middleware
	r.Use(loggingMiddleware)

	// JWT authentication middleware
	r.Use(jwtAuthenticationMiddleware)

	// Routes
	r.HandleFunc("/", homePage)
	r.HandleFunc("/protected", protectedPage)

	fmt.Printf("Listening on :8080 ...\n")
	http.ListenAndServe(":8080", r)
	fmt.Printf("exising\n")
}

func loggingMiddleware(next http.Handler) http.Handler {
	return handlers.LoggingHandler(os.Stdout, next)
}

func jwtAuthenticationMiddleware(next http.Handler) http.Handler {

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

		// Validate JWT
		tokenString := extractToken(r)
		token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
			return mySigningKey, nil
		})

		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		// JWT valid, pass user info to context
		ctx := context.WithValue(r.Context(), userIDKey, token.Claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func homePage(w http.ResponseWriter, r *http.Request) {
	// Public page

	w.WriteHeader(http.StatusOK)
}

func protectedPage(w http.ResponseWriter, r *http.Request) {
	user := r.Context().Value("user").(User)
	fmt.Printf("User: %v\n", user)

	// Protected page
	w.WriteHeader(http.StatusOK)
}

func extractToken(r *http.Request) string {
	// Extract token from request
	return "token"
}
