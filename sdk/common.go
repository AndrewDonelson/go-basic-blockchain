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
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"net/smtp"
	"os"
	"reflect"
	"regexp"
	"strings"
	"time"
)

// VerifySignature verifies the provided signature against the given message and public key.
// It returns true if the signature is valid, and false otherwise.
func VerifySignature(message []byte, signature []byte, publicKey *ecdsa.PublicKey) bool {
	// Verify the signature by recovering the public key from the signature and comparing it with the provided public key.
	// Use the Verify function from the elliptic package to perform the verification.
	// The Verify function returns true if the signature is valid, and false otherwise.

	// Prepare the hashed message
	hash := sha256.Sum256(message)

	// Extract the r and s components from the signature
	r := big.Int{}
	s := big.Int{}
	sigLen := len(signature)
	r.SetBytes(signature[:(sigLen / 2)])
	s.SetBytes(signature[(sigLen / 2):])

	// Verify the signature using the public key
	return ecdsa.Verify(publicKey, hash[:], &r, &s)
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
func GetUserIP(r *http.Request) string {
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

// IntToBytes converts an integer to a byte slice in big-endian encoding.
// The resulting byte slice will always be 4 bytes long, regardless of the
// value of the input integer.
func IntToBytes(n int) []byte {
	// Create a byte slice with a fixed size to hold the converted int.
	byteSlice := make([]byte, 4) // Assuming int is 32 bits (4 bytes)

	// Convert the int to bytes using big-endian encoding and store it in the byte slice.
	binary.BigEndian.PutUint32(byteSlice, uint32(n))

	return byteSlice
}

// ValidateAddress validates the provided Ethereum address string. It decodes the address
// and verifies that the length of the decoded bytes is 32. If the address is invalid,
// it returns an error.
func ValidateAddress(address string) error {
	// Decode the test address
	addr, err := hex.DecodeString(address)
	if err != nil {
		return fmt.Errorf("failed to decode address: %v", err)
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
	uppercaseCount := countMatches(password, "[A-Z]")
	if uppercaseCount < 2 {
		return fmt.Errorf("password should contain at least 2 uppercase letters")
	}

	// Check for at least 2 lowercase letters
	lowercaseCount := countMatches(password, "[a-z]")
	if lowercaseCount < 2 {
		return fmt.Errorf("password should contain at least 2 lowercase letters")
	}

	// Check for at least 2 digits
	digitCount := countMatches(password, "[0-9]")
	if digitCount < 2 {
		return fmt.Errorf("password should contain at least 2 digits")
	}

	// Check for at least 2 special characters
	specialCharCount := countMatches(password, `[~!@#$%^&*()=+\[\]{}|\\/?<>]`)
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
	// Define the character sets for each requirement
	uppercaseChars := "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	lowercaseChars := "abcdefghijklmnopqrstuvwxyz"
	digitChars := "0123456789"
	specialChars := "~!@#$%^&*()=+[]{}|\\/?<>"

	// Initialize the password
	password := ""

	// Generate random characters for each requirement
	uppercaseCount := 0
	lowercaseCount := 0
	digitCount := 0
	specialCharCount := 0

	for len(password) < 24 {
		// Generate a random index for the character set
		index, err := rand.Int(rand.Reader, big.NewInt(4))
		if err != nil {
			return "", fmt.Errorf("failed to generate random password: %v", err)
		}

		// Get the character set based on the index
		var charSet string
		switch index.Int64() {
		case 0:
			charSet = uppercaseChars
			uppercaseCount++
		case 1:
			charSet = lowercaseChars
			lowercaseCount++
		case 2:
			charSet = digitChars
			digitCount++
		case 3:
			charSet = specialChars
			specialCharCount++
		}

		// Generate a random index for the character set
		charIndex, err := rand.Int(rand.Reader, big.NewInt(int64(len(charSet))))
		if err != nil {
			return "", fmt.Errorf("failed to generate random password: %v", err)
		}

		// Add the random character to the password
		password += string(charSet[charIndex.Int64()])
	}

	// Check password strength
	if uppercaseCount < 2 || lowercaseCount < 2 || digitCount < 2 || specialCharCount < 2 {
		return "", fmt.Errorf("generated password does not meet the strength criteria")
	}

	return password, nil
}

// countMatches returns the number of non-overlapping matches of the regular expression pattern in the string s.
func countMatches(s, pattern string) int {
	re := regexp.MustCompile(pattern)
	matches := re.FindAllString(s, -1)
	return len(matches)
}

// createFolder creates the folder if it does not exist.
func createFolder(path string) {
	// Check if the folder exists, if not, create it
	if _, err := os.Stat(path); os.IsNotExist(err) {
		err := os.MkdirAll(path, 0755)
		if err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("[%s] directory '%s' created.\n", time.Now().Format(logDateTimeFormat), path)
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
func generateRandomToken() string {
	b := make([]byte, 32) // 256 bits
	rand.Read(b)
	return base64.URLEncoding.EncodeToString(b)
}

// isBase64Encoded checks if the given string is a valid base64-encoded string.
// It does this by attempting to decode the string using the standard base64 encoding,
// and returning true if the decoding is successful (i.e. no error is returned).
func isBase64Encoded(s string) bool {
	_, err := base64.StdEncoding.DecodeString(s)
	return err == nil
}
