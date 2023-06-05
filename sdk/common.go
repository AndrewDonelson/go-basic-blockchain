// file: sdk/common.go
// package: sdk
// description: common functions
package sdk

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"reflect"
	"strings"
)

// VerifySignature verifies the signature of a message using the provided public key.
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

// PrettyPrint is used to display any type nicely in the log output
func PrettyPrint(v interface{}) string {

	name := GetType(v)
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}

	return fmt.Sprintf("Dump of [%s]:\n%s\n", name, string(b))
}

// GetType will return the name of the provided interface using reflection
func GetType(i interface{}) string {
	t := reflect.TypeOf(i)
	if t.Kind() == reflect.Ptr {
		return "*" + t.Elem().Name()
	}

	return t.Name()
}

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

func IntToBytes(n int) []byte {
	// Create a byte slice with a fixed size to hold the converted int.
	byteSlice := make([]byte, 4) // Assuming int is 32 bits (4 bytes)

	// Convert the int to bytes using big-endian encoding and store it in the byte slice.
	binary.BigEndian.PutUint32(byteSlice, uint32(n))

	return byteSlice
}

// ValidateAddress validates the provided address
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
