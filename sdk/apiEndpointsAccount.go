// Package sdk is a software development kit for building blockchain applications.
// File sdk/apiEndpointsAccount.go - account registration, verification and login
package sdk

import (
	"crypto/subtle"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"time"
)

// accountCredentials is the request body for register and login.
//
// The password is sent in the body of a POST, not as a query parameter of a GET.
// Query strings end up in access logs, proxy logs, browser history and Referer
// headers, so the previous ?password_hash=... scheme leaked the credential to
// every intermediary on the path.
type accountCredentials struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// decodeAccountCredentials reads credentials from the request body, falling back
// to query parameters only for the email so error messages stay useful.
func decodeAccountCredentials(r *http.Request) (accountCredentials, bool) {
	var creds accountCredentials
	if err := json.NewDecoder(io.LimitReader(r.Body, maxAccountRequestBytes)).Decode(&creds); err != nil {
		return accountCredentials{}, false
	}
	creds.Email = normalizeAccountEmail(creds.Email)
	return creds, true
}

// minAccountPasswordLength is the shortest password a new account may use.
const (
	minAccountPasswordLength = 12
	maxAccountPasswordLength = 128
	maxAccountRequestBytes   = 1 << 16
	accountTokenTTL          = 30 * time.Minute
)

// handleAccountRegister registers a new account and emails a verification token.
//
// POST /account/register  {"email": "...", "password": "..."}
//
// Three things changed here, all of them security-relevant:
//
//  1. It is a POST. Registration creates state, so it was never safe or idempotent
//     as a GET, and GETs are cached and logged with their query strings.
//  2. The verification token is no longer returned in the response. Returning it
//     made the entire email-verification step a formality that any caller could
//     skip.
//  3. Registering an address that is already verified is refused. Previously an
//     attacker could re-register a victim's verified address, read the token
//     straight out of the response, verify, and thereby replace the victim's
//     stored credential -- a full account takeover.
func (api *API) handleAccountRegister(w http.ResponseWriter, r *http.Request) {
	if api.accountStore == nil {
		RespondError(w, http.StatusInternalServerError, "Account store unavailable")
		return
	}

	if !api.accountLimiter.allow(clientIP(r)) {
		RespondError(w, http.StatusTooManyRequests, "Too many registration attempts")
		return
	}

	creds, ok := decodeAccountCredentials(r)
	if !ok {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if !isValidEmail(creds.Email) {
		RespondError(w, http.StatusBadRequest, "Invalid email format")
		return
	}

	if len(creds.Password) < minAccountPasswordLength || len(creds.Password) > maxAccountPasswordLength {
		RespondError(w, http.StatusBadRequest, "Password must be between 12 and 128 characters")
		return
	}

	// Refuse to shadow an already-verified account.
	if _, exists, err := api.accountStore.GetVerified(creds.Email); err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to read account store")
		return
	} else if exists {
		// Deliberately the same generic response as success, so this endpoint
		// cannot be used to enumerate which addresses are registered.
		respondJSON(w, http.StatusAccepted, registrationAccepted())
		return
	}

	passwordHash, passwordSalt, err := hashAccountPassword(creds.Password)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to hash password")
		return
	}

	token := generateRandomToken()
	if token == "" {
		RespondError(w, http.StatusInternalServerError, "Failed to generate verification token")
		return
	}

	// Only the token's hash is stored, so a leaked store cannot be used to verify
	// somebody else's pending registration.
	err = api.accountStore.SavePending(PendingAccountRecord{
		Email:        creds.Email,
		PasswordHash: passwordHash,
		PasswordSalt: passwordSalt,
		TokenHash:    hashAPIKey(token),
		ExpiresAt:    time.Now().Add(accountTokenTTL),
	})
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to persist registration")
		return
	}

	api.deliverVerificationToken(creds.Email, token)

	respondJSON(w, http.StatusAccepted, registrationAccepted())
}

// registrationAccepted is the single response registration ever gives, so the
// endpoint reveals nothing about whether an address already exists.
func registrationAccepted() interface{} {
	return struct {
		Status  string `json:"status"`
		Message string `json:"message"`
	}{
		Status:  "ok",
		Message: "If the address is eligible, a verification link has been sent.",
	}
}

// deliverVerificationToken sends the verification link out of band.
//
// The doc comment on the old handler promised an email and no email was ever
// sent; the token came back in the HTTP response instead. When no mail transport
// is configured the token is written to the node log so a local operator can
// still complete the flow, but it never reaches the HTTP client.
func (api *API) deliverVerificationToken(email, token string) {
	cfg := api.GetConfig()
	link := "/account/verify?email=" + email + "&token=" + token
	if cfg != nil && cfg.Domain != "" {
		link = strings.TrimRight(cfg.Domain, "/") + link
	}

	if cfg != nil && cfg.GMailEmail != "" && cfg.GMailPassword != "" {
		body := "Please verify your account by visiting:\r\n" + link
		if err := SendGmail(email, "Verify your account", body, cfg); err != nil {
			LogInfof("Failed to send verification email to %s: %v", email, err)
		} else {
			return
		}
	}

	LogInfof("Verification link for %s: %s", email, link)
}

// handleAccountLogin authenticates an account and issues a fresh API key.
//
// POST /account/login  {"email": "...", "password": "..."}
//
// The issued key is random, stored only as a SHA-256 hash, and accepted by the
// API key middleware. Previously login returned SHA256(serverSeed+email): a value
// that was deterministic, unrevocable, derivable by anyone who knew the seed (which
// was a published constant), and -- because the middleware only ever checked the
// single statically configured key -- did not actually authenticate anything.
func (api *API) handleAccountLogin(w http.ResponseWriter, r *http.Request) {
	if api.accountStore == nil {
		RespondError(w, http.StatusInternalServerError, "Account store unavailable")
		return
	}

	if !api.accountLimiter.allow(clientIP(r)) {
		RespondError(w, http.StatusTooManyRequests, "Too many login attempts")
		return
	}

	creds, ok := decodeAccountCredentials(r)
	if !ok {
		RespondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if !isValidEmail(creds.Email) {
		RespondError(w, http.StatusBadRequest, "Invalid email format")
		return
	}

	record, exists, err := api.accountStore.GetVerified(creds.Email)
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to read account store")
		return
	}

	// Same response for "no such account" and "wrong password" so the endpoint
	// cannot be used to enumerate accounts.
	if !exists || !verifyAccountPassword(creds.Password, record.PasswordHash, record.PasswordSalt) {
		RespondError(w, http.StatusUnauthorized, "Invalid credentials")
		return
	}

	apiKey, hashed, err := generateAPIKey()
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to issue API key")
		return
	}

	record.APIKeyHash = hashed
	if err := api.accountStore.SaveVerified(record); err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to persist API key")
		return
	}

	api.accountLimiter.reset(clientIP(r))

	respondJSON(w, http.StatusOK, struct {
		Status string `json:"status"`
		APIKey string `json:"api_key"`
	}{
		Status: "ok",
		APIKey: apiKey,
	})
}

// handleAccountVerify verifies a pending registration.
//
// GET /account/verify?email=EMAIL&token=TOKEN  (a GET because it is followed from
// an emailed link, and it is idempotent once the pending record is consumed).
func (api *API) handleAccountVerify(w http.ResponseWriter, r *http.Request) {
	if api.accountStore == nil {
		RespondError(w, http.StatusInternalServerError, "Account store unavailable")
		return
	}

	if !api.accountLimiter.allow(clientIP(r)) {
		RespondError(w, http.StatusTooManyRequests, "Too many verification attempts")
		return
	}

	email := normalizeAccountEmail(r.URL.Query().Get("email"))
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

	// Constant-time: a byte-by-byte string compare leaks the token by timing.
	if subtle.ConstantTimeCompare([]byte(pending.TokenHash), []byte(hashAPIKey(token))) != 1 {
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

	apiKey, hashed, err := generateAPIKey()
	if err != nil {
		RespondError(w, http.StatusInternalServerError, "Failed to issue API key")
		return
	}

	err = api.accountStore.SaveVerified(VerifiedAccountRecord{
		Email:        email,
		PasswordHash: pending.PasswordHash,
		PasswordSalt: pending.PasswordSalt,
		APIKeyHash:   hashed,
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

	api.accountLimiter.reset(clientIP(r))

	respondJSON(w, http.StatusOK, struct {
		Status string `json:"status"`
		APIKey string `json:"api_key"`
	}{
		Status: "ok",
		APIKey: apiKey,
	})
}
