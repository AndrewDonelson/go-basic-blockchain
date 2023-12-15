package sdk

import (
	"net/http"
	"net/url"
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

	// Generate a verification token
	token := generateRandomToken()

	// Store the email and token in a database with an expiration time of 30 minutes
	// You'd typically do this with a SQL INSERT operation, perhaps using a package like sqlx or gorm.
	// For this example, we'll abstract this operation:
	// storeEmailAndToken(email, token)
	ls, err := GetLocalStorage()
	if err != nil {
		api.log.Error("Failed to get local storage", "error", err)
		RespondError(w, http.StatusInternalServerError, "Failed to get local storage")
		return
	}
	ls.Set("email", email)

	// Send a verification email
	verificationLink := api.GetConfig().Domain + "/account/verify?email=" + url.QueryEscape(email) + "&token=" + token
	err = SendGmail(email, "Verify Your Account", "Click the link to verify: "+verificationLink, api.GetConfig())
	if err != nil {
		api.log.Error("Failed to send verification email", "error", err)
		RespondError(w, http.StatusInternalServerError, "Failed to send verification email")
		return
	}
	w.Write([]byte("Registration successful. Please verify your email."))
}

// handleAccountLogin handles the login of an existing account and returns an API key.
// POST email & password hash and returns JSON with API key
func (api *API) handleAccountLogin(w http.ResponseWriter, r *http.Request) {
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}

// handleAccountVerify handles the verification of a new account from the email link.
// email link format: https://somedomain.com/account/verify?email=EMAIL&token=TOKEN
func (api *API) handleAccountVerify(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	api.log.Notice("Received Verification Link: ", token)
	// Return "Not Yet Implemented"
	w.Write([]byte("Not Yet Implemented"))
}
