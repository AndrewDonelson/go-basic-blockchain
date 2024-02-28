package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// User is a user
type User struct {
	ID        int
	Username  string
	Email     string
	FirstName string
	Password  string
}

// NewAPIKey generates an API key using the server's seed and the provided user object.
func (u *User) NewAPIKey() string {
	// Combine the server seed and the email
	combined := fmt.Sprintf("%s,%s,%s,%s,%s", serverSeed, u.Email, u.Username, u.FirstName, u.Password)

	// Hash the combined string
	hash := sha256.Sum256([]byte(combined))

	// Return the hex representation of the hash as the API key
	return hex.EncodeToString(hash[:])
}

// String returns a string representation of the user
func (u *User) String() string {
	return fmt.Sprintf("User: %s, Email: %s", u.Username, u.Email)
}
