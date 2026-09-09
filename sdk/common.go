// Package sdk is a software development kit for building blockchain applications.
// File sdk/common.go - Common functions for the sdk
package sdk

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// VerifySignature verifies an ASN.1/DER ECDSA signature over message.
//
// It previously split the signature in half and treated the parts as raw r||s.
// Everything in this project signs with ecdsa.SignASN1, which produces DER, so
// this function accepted and rejected essentially at random -- a dangerous
// property for anything named VerifySignature.
func VerifySignature(message []byte, signature []byte, publicKey *ecdsa.PublicKey) bool {
	if publicKey == nil || len(signature) == 0 {
		return false
	}

	hash := sha256.Sum256(message)
	return ecdsa.VerifyASN1(publicKey, hash[:], signature)
}

// PrettyPrint takes an arbitrary interface{} value and returns a formatted string
// representation of the value. It uses json.MarshalIndent to pretty-print the
// JSON encoding of the value, and prepends the type name of the value.
func PrettyPrint(v interface{}) string {

	name := GetType(v)
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}

	return fmt.Sprintf("Dump of [%s]:\n%s\n", name, string(b))
}

// GetType returns the type name of the provided interface{} value. If the value is a pointer,
// it returns the type name prefixed with "*".
func GetType(i interface{}) string {
	t := reflect.TypeOf(i)
	if t.Kind() == reflect.Ptr {
		return "*" + t.Elem().Name()
	}

	return t.Name()
}

// GetUserIP returns the IP address of the client making the HTTP request. It handles cases where the request
// comes through a proxy by parsing the X-Forwarded-For header. If the header is not set, it falls back to
// the RemoteAddr field of the request.
// GetUserIP returns the client address for logging.
//
// X-Forwarded-For is attacker-controlled and only meaningful behind a proxy you
// operate, so it is honoured only when TRUST_PROXY_HEADERS is set. Trusting it
// unconditionally let any client forge the address in the logs.
func GetUserIP(r *http.Request) string {
	if getEnvAsBool("TRUST_PROXY_HEADERS", false) {
		if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
			ips := strings.Split(forwardedFor, ",")
			return strings.TrimSpace(ips[0])
		}
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}

	return ip
}

//nolint:unused
func getUserIPLegacy(r *http.Request) string {
	// Check if the request comes through a proxy
	if forwardedFor := r.Header.Get("X-Forwarded-For"); forwardedFor != "" {
		// The X-Forwarded-For header may contain a comma-separated list of IP addresses
		// where the left-most address is the original client IP and the rest are proxy addresses.
		// Split the header value and return the left-most IP address.
		ips := strings.Split(forwardedFor, ",")
		return strings.TrimSpace(ips[0])
	}

	// If X-Forwarded-For header is not set, fallback to RemoteAddr
	// RemoteAddr typically has the format "IP:Port"
	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		// In case of error, return an empty string or handle the error as needed
		return ""
	}

	return ip
}

// IntToBytes converts an integer to a 4-byte big-endian slice.
//
// The output is always 4 bytes, so only the low 32 bits survive: a value outside
// [-2147483648, 4294967295] is truncated, and a negative one comes back as its
// two's-complement pattern. That is fine for a hash preimage or an identifier,
// and wrong for anything that has to round-trip -- use binary.BigEndian.PutUint64
// on a fixed-width type instead.
func IntToBytes(n int) []byte {
	// Create a byte slice with a fixed size to hold the converted int.
	byteSlice := make([]byte, 4) // Assuming int is 32 bits (4 bytes)

	// Convert the int to bytes using big-endian encoding and store it in the byte slice.
	binary.BigEndian.PutUint32(byteSlice, uint32(n)) //nolint:gosec // documented truncation

	return byteSlice
}

// ConvertToFloat64 converts various types to float64.
// It supports the following types: float64, float32, int, int64, int32, and string.
// If the input is a string, it attempts to parse it as a float64.
// If the conversion is successful, it returns the float64 value and a nil error.
// If the conversion fails or the type is unsupported, it returns 0 and an error.
//
// Parameters:
// - value: The input value of type interface{} to be converted.
//
// Returns:
// - float64: The converted float64 value.
// - error: An error if the conversion fails or the type is unsupported.
func ConvertToFloat64(value interface{}) (float64, error) {
	switch v := value.(type) {
	case float64:
		return v, nil
	case float32:
		return float64(v), nil
	case int:
		return float64(v), nil
	case int64:
		return float64(v), nil
	case int32:
		return float64(v), nil
	case string:
		return strconv.ParseFloat(v, 64)
	default:
		return 0, fmt.Errorf("unexpected type for conversion to float64: %T", value)
	}
}

// ValidateAddress validates a wallet address for this chain.
//
// Addresses here are the hex encoding of a SHA-256 hash, so 32 bytes. The doc
// comment used to describe these as Ethereum addresses, which are 20 bytes.
func ValidateAddress(address string) error {
	// Decode the test address
	addr, err := hex.DecodeString(address)
	if err != nil {
		return fmt.Errorf("failed to decode address: %w", err)
	}

	// Verify the address length
	if len(addr) != 32 {
		return fmt.Errorf("expected address length: %d, got: %d", 32, len(addr))
	}

	return nil
}

// testPasswordStrength tests the password strength. It checks that the password is between 12 and 24 characters
// long, and contains at least 2 uppercase letters, 2 lowercase letters, 2 digits, and 2 special characters. If
// the password does not meet these requirements, it returns an error.
func testPasswordStrength(password string) error {
	// Check password length
	if len(password) < 12 || len(password) > 24 {
		return fmt.Errorf("password length should be between 12 and 24 characters")
	}

	// Check for at least 2 uppercase letters
	uppercaseCount := countMatches(password, reUppercase)
	if uppercaseCount < 2 {
		return fmt.Errorf("password should contain at least 2 uppercase letters")
	}

	// Check for at least 2 lowercase letters
	lowercaseCount := countMatches(password, reLowercase)
	if lowercaseCount < 2 {
		return fmt.Errorf("password should contain at least 2 lowercase letters")
	}

	// Check for at least 2 digits
	digitCount := countMatches(password, reDigit)
	if digitCount < 2 {
		return fmt.Errorf("password should contain at least 2 digits")
	}

	// Check for at least 2 special characters
	specialCharCount := countMatches(password, reSpecial)
	if specialCharCount < 2 {
		return fmt.Errorf("password should contain at least 2 special characters (~!@#$%%^&*()=+[]{}|\\/<>?)")
	}

	return nil
}

// GenerateRandomPassword generates a random password that meets the following requirements:
// - Length between 12 and 24 characters
// - At least 2 uppercase letters
// - At least 2 lowercase letters
// - At least 2 digits
// - At least 2 special characters
//
// If the generated password does not meet these requirements, an error is returned.
func GenerateRandomPassword() (string, error) {
	const (
		uppercaseChars = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
		lowercaseChars = "abcdefghijklmnopqrstuvwxyz"
		digitChars     = "0123456789"
		specialChars   = "~!@#$%^&*()_+-=[]{}|;:,.<>?"
		allChars       = uppercaseChars + lowercaseChars + digitChars + specialChars
		passwordLength = 24
	)

	for attempts := 0; attempts < 100; attempts++ {
		password := make([]byte, passwordLength)

		// Ensure at least 2 characters from each category
		password[0] = uppercaseChars[SecureRandomInt(len(uppercaseChars))]
		password[1] = uppercaseChars[SecureRandomInt(len(uppercaseChars))]
		password[2] = lowercaseChars[SecureRandomInt(len(lowercaseChars))]
		password[3] = lowercaseChars[SecureRandomInt(len(lowercaseChars))]
		password[4] = digitChars[SecureRandomInt(len(digitChars))]
		password[5] = digitChars[SecureRandomInt(len(digitChars))]
		password[6] = specialChars[SecureRandomInt(len(specialChars))]
		password[7] = specialChars[SecureRandomInt(len(specialChars))]

		// Fill the rest with random characters
		for i := 8; i < passwordLength; i++ {
			password[i] = allChars[SecureRandomInt(len(allChars))]
		}

		// Shuffle the password
		for i := len(password) - 1; i > 0; i-- {
			j := SecureRandomInt(i + 1)
			password[i], password[j] = password[j], password[i]
		}

		// Test the password strength
		if testPasswordStrength(string(password)) == nil {
			return string(password), nil
		}
	}

	return "", errors.New("failed to generate a password meeting the strength criteria after 100 attempts")
}

// SecureRandomInt returns a cryptographically secure integer in [0, max).
//
// A non-positive max returns 0 instead of panicking: crypto/rand.Int panics on
// max <= 0, which made this a latent crash for any caller that computed its
// bound.
func SecureRandomInt(max int) int {
	if max <= 0 {
		return 0
	}

	n, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		// The only way crypto/rand fails is a broken system entropy source.
		LogInfof("crypto/rand failure: %v", err)
		return 0
	}
	return int(n.Int64())
}

// SecureRandomUint64 returns a cryptographically secure 64-bit value.
//
// Transaction nonces used to be SecureRandomInt(8) -- a value in 0..7, which
// gives essentially no replay protection at all.
func SecureRandomUint64() uint64 {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		LogInfof("crypto/rand failure: %v", err)
		return 0
	}
	return binary.BigEndian.Uint64(buf)
}

// Password-strength patterns, compiled once.
//
// countMatches used to call regexp.MustCompile on every invocation, so each
// testPasswordStrength call recompiled four patterns -- on every wallet create,
// unlock and lock.
var (
	reUppercase = regexp.MustCompile(`[A-Z]`)
	reLowercase = regexp.MustCompile(`[a-z]`)
	reDigit     = regexp.MustCompile(`[0-9]`)
	reSpecial   = regexp.MustCompile(`[~!@#$%^&*()=+\[\]{}|\\/?<>_.,;:-]`)
)

// countMatches returns the number of non-overlapping matches of re in s.
func countMatches(s string, re *regexp.Regexp) int {
	return len(re.FindAllString(s, -1))
}

// createFolder creates the folder if it does not exist.
// This function is currently unused but kept for potential future use
//
//nolint:unused
func createFolder(path string) {
	// Check if the folder exists, if not, create it
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			log.Fatalln(err)
		}
		log.Printf("[%s] directory '%s' created.\n", time.Now().Format(logDateTimeFormat), path)
	}
}

// SendGmail sends an email using the provided Gmail account configuration.
//
// The function takes the recipient email address, subject, and body of the email,
// as well as a Config struct containing the Gmail account email and password.
//
// It first validates the provided configuration and email addresses, then constructs
// the email message and sends it using the Gmail SMTP server.
//
// If any errors occur during the process, the function will return an error.
func SendGmail(to, subject, body string, cfg *Config) error {

	// Validate config settings
	if cfg == nil {
		return fmt.Errorf("config is nil")
	}

	// Validate recipient email address
	if !isValidEmail(to) {
		return fmt.Errorf("invalid email (TO) format")
	}

	// Sender data.
	from := cfg.GMailEmail
	if !isValidEmail(from) {
		return fmt.Errorf("invalid email (FROM) format")
	}
	password := cfg.GMailPassword

	// Message.
	message := []byte("To: " + to + "\r\n" +
		"Subject: " + subject + "\r\n" +
		"\r\n" +
		body + "\r\n")

	err := smtp.SendMail("smtp.gmail.com:587", smtp.PlainAuth("", from, password, "smtp.gmail.com"), from, []string{to}, message)
	if err != nil {
		return err
	}

	return nil
}

// isValidEmail checks if the provided email string is in a valid format.
// It uses a basic regular expression to validate the email address.
func isValidEmail(email string) bool {
	// Basic regex to check email format
	var emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// generateRandomToken generates a random 256-bit token encoded as a URL-safe base64 string.
// This function is used to generate unique identifiers or tokens, such as for authentication purposes.
// generateRandomToken returns a random 256-bit URL-safe token, or "" on failure.
//
// The error from rand.Read used to be discarded, so a failing entropy source
// produced a token of 32 zero bytes -- identical for every caller, and trivially
// guessable.
func generateRandomToken() string {
	b := make([]byte, 32) // 256 bits
	if _, err := rand.Read(b); err != nil {
		LogInfof("failed to generate random token: %v", err)
		return ""
	}
	return base64.URLEncoding.EncodeToString(b)
}

// configVerbose gates verbose logging. It is accessed from multiple goroutines
// (the miner, the API handlers, the menu), so it is atomic rather than a plain
// bool read and written without synchronisation.
var configVerbose atomic.Bool

// LogVerbosef logs only when verbose logging is enabled.
func LogVerbosef(format string, args ...interface{}) {
	if configVerbose.Load() {
		log.Printf("[VERBOSE] "+format, args...)
	}
}

// LogInfof logs unconditionally.
func LogInfof(format string, args ...interface{}) {
	log.Printf(format, args...)
}

// ConfigSetVerbose enables or disables verbose logging.
func ConfigSetVerbose(v bool) {
	configVerbose.Store(v)
}
