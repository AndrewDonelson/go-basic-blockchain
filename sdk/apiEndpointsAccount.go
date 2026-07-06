// Package sdk is a software development kit for building blockchain applications.
// File sdk/apiEndpointsAccount.go -
package sdk

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// handleAccountRegister handles the registration of a new account and returns an API key.
// api.router.HandleFunc("/account/register", api.handleAccountRegister).Methods("GET")
// Shell: curl -X GET "http://localhost:8080/account/register?email=EMAIL&password_hash=PASSWORD_HASH"
// Go: http.Get("http://localhost:8080/account/register?email=EMAIL&password_hash=PASSWORD_HASH")
// GDScript: HTTP.request("http://localhost:8080/account/register?email=EMAIL&password_hash=PASSWORD_HASH", [], true, HTTP.METHOD_GET)
//
// 1. receives a GET with email & password hash query parameters
// 2. validates the email & password hash for SQL injection and password complexity
// 3. sends an email with a verification link that expires is 30 minutes
// 4. email link format: https://somedomain.com/account/verify?email=EMAIL&token=TOKEN
func (api *API) handleAccountRegister(w http.ResponseWriter, r *http.Request) {
	if api.accountStore == nil {
		RespondError(w, http.StatusInternalServerError, "Account store unavailable")
		return
	}

	// Extract email and password hash from query parameters
	email := r.URL.Query().Get("email")
	passwordHash := r.URL.Query().Get("password_hash")

	// Validate email
	if !isValidEmail(email) {
		RespondError(w, http.StatusBadRequest, "Invalid email format")
		return
	}

	// Validate password hash (for simplicity, just check length)
	if len(passwordHash) != 64 { // Assuming SHA-256 hash
		RespondError(w, http.StatusBadRequest, "Invalid password hash")
		return
	}

	token := generateRandomToken()

	err := api.accountStore.SavePending(PendingAccountRecord{
		Email:        strings.ToLower(email),
		PasswordHash: passwordHash,
		Token:        token,
		ExpiresAt:    time.Now().Add(30 * time.Minute),
	})
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to persist registration")
		return
	}

	response := struct {
		Status            string `json:"status"`
		Message           string `json:"message"`
		VerificationToken string `json:"verification_token"`
	}{
		Status:            "ok",
		Message:           "Registration successful. Please verify your email.",
		VerificationToken: token,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleAccountLogin handles the login of an existing account and returns an API key.
// POST email & password hash and returns JSON with API key
func (api *API) handleAccountLogin(w http.ResponseWriter, r *http.Request) {
	if api.accountStore == nil {
		RespondError(w, http.StatusInternalServerError, "Account store unavailable")
		return
	}

	email := strings.ToLower(r.URL.Query().Get("email"))
	passwordHash := r.URL.Query().Get("password_hash")

	if !isValidEmail(email) {
		RespondError(w, http.StatusBadRequest, "Invalid email format")
		return
	}

	if len(passwordHash) != 64 {
		RespondError(w, http.StatusBadRequest, "Invalid password hash")
		return
	}

	record, exists, err := api.accountStore.GetVerified(email)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to read account store")
		return
	}

	if !exists {
		RespondError(w, http.StatusUnauthorized, "Account not verified")
		return
	}

	if record.PasswordHash != passwordHash {
		RespondError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	response := struct {
		Status string `json:"status"`
		APIKey string `json:"api_key"`
	}{
		Status: "ok",
		APIKey: generateAPIKeyForEmail(email),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}

// handleAccountVerify handles the verification of a new account from the email link.
// email link format: https://somedomain.com/account/verify?email=EMAIL&token=TOKEN
func (api *API) handleAccountVerify(w http.ResponseWriter, r *http.Request) {
	if api.accountStore == nil {
		RespondError(w, http.StatusInternalServerError, "Account store unavailable")
		return
	}

	email := strings.ToLower(r.URL.Query().Get("email"))
	token := r.URL.Query().Get("token")

	if !isValidEmail(email) {
		RespondError(w, http.StatusBadRequest, "Invalid email format")
		return
	}

	if token == "" {
		RespondError(w, http.StatusBadRequest, "Invalid verification token")
		return
	}

	pending, exists, err := api.accountStore.GetPending(email)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to read account store")
		return
	}

	if !exists {
		RespondError(w, http.StatusNotFound, "Pending registration not found")
		return
	}

	if pending.Token != token {
		RespondError(w, http.StatusUnauthorized, "Invalid verification token")
		return
	}

	if time.Now().After(pending.ExpiresAt) {
		if err := api.accountStore.DeletePending(email); err != nil {
			RespondError(w, http.StatusInternalServerError, "Failed to update account store")
			return
		}
		RespondError(w, http.StatusUnauthorized, "Verification token expired")
		return
	}

	err = api.accountStore.SaveVerified(VerifiedAccountRecord{
		Email:        email,
		PasswordHash: pending.PasswordHash,
		VerifiedAt:   time.Now(),
	})
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to persist verified account")
		return
	}

	if err := api.accountStore.DeletePending(email); err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to update account store")
		return
	}

	response := struct {
		Status string `json:"status"`
		APIKey string `json:"api_key"`
	}{
		Status: "ok",
		APIKey: generateAPIKeyForEmail(email),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
}
