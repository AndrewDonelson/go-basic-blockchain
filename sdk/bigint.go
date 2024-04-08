// Package sdk is a software development kit for building blockchain applications.
// File sdk/bigint.go - wrapper for big integers with custom methods suchs as String(), Bytes() and Base64()

package sdk

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"math"
	"strconv"
	"time"
)

// bigint -9,223,372,036,854,775,808 to 9,223,372,036,854,775,807 (64-bit)
type BigInt struct {
	Val int64
}

// NewBigInt creates a new BigInt instance.
func NewBigInt(val int64) *BigInt {
	return &BigInt{Val: val}
}

// NewRandomBigInt creates a new BigInt instance with a random value.
func NewRandomBigInt() (*BigInt, error) {
	return NewBigInt(0).Random()
}

// NewBigIntFromBytes creates a new BigInt instance from a byte slice.
func NewBigIntFromBytes(bytes []byte) *BigInt {
	return &BigInt{Val: int64(binary.BigEndian.Uint64(bytes))}
}

// NewBigIntFromString creates a new BigInt instance from a string representation.
// If the input string is base64-encoded, it will be decoded first.
func NewBigIntFromString(s string) (*BigInt, error) {
	// Check if the input string is base64-encoded
	if isBase64Encoded(s) {
		// Decode the base64 string
		decoded, err := base64.StdEncoding.DecodeString(s)
		if err != nil {
			return nil, err
		}

		// Create a BigInt from the decoded byte slice
		return NewBigIntFromBytes(decoded), nil
	}

	// Parse the string as a decimal integer
	val, err := parseInt64(s)
	if err != nil {
		return nil, err
	}
	return &BigInt{Val: val}, nil
}

// String returns the string representation of the BigInt.
func (b *BigInt) String() string {
	return formatInt64(b.Val)
}

// Bytes returns the byte representation of the BigInt.
func (b *BigInt) Bytes() []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, uint64(b.Val))
	return bytes
}

// Base64 returns the base64 representation of the BigInt.
func (b *BigInt) Base64() string {
	return encodeToBase64(b.Bytes())
}

// RandomBigInt returns a cryptographically secure random BigInt incorporating the current time in nanoseconds.
// It generates a random 64-bit integer and combines it with the current time in nanoseconds to create a new BigInt.
// This function is useful for generating unique, random BigInt values.
func (b *BigInt) Random() (*BigInt, error) {
	// Generate a random 64-bit integer
	var randomInt int64
	buf := make([]byte, 8)
	_, err := rand.Read(buf)
	if err != nil {
		return nil, err
	}
	randomInt = int64(binary.BigEndian.Uint64(buf))

	// Get the current time in nanoseconds
	currentTime := time.Now().UnixNano()

	// Combine the random number and current time
	combined := currentTime ^ randomInt

	// Create a new BigInt from the combined value
	return NewBigInt(combined), nil
}

// Add adds the given BigInt to the current BigInt.
func (b *BigInt) Add(other *BigInt) *BigInt {
	return &BigInt{Val: b.Val + other.Val}
}

// Subtract subtracts the given BigInt from the current BigInt.
func (b *BigInt) Subtract(other *BigInt) *BigInt {
	return &BigInt{Val: b.Val - other.Val}
}

// Multiply multiplies the given BigInt with the current BigInt.
func (b *BigInt) Multiply(other *BigInt) *BigInt {
	return &BigInt{Val: b.Val * other.Val}
}

// Divide divides the current BigInt by the given BigInt.
func (b *BigInt) Divide(other *BigInt) *BigInt {
	return &BigInt{Val: b.Val / other.Val}
}

// Modulo returns the modulo of the current BigInt by the given BigInt.
func (b *BigInt) Modulo(other *BigInt) *BigInt {
	return &BigInt{Val: b.Val % other.Val}
}

// Compare compares the current BigInt with the given BigInt.
func (b *BigInt) Compare(other *BigInt) int {
	switch {
	case b.Val > other.Val:
		return 1
	case b.Val < other.Val:
		return -1
	default:
		return 0
	}
}

// IsZero returns true if the BigInt is zero.
func (b *BigInt) IsZero() bool {
	return b.Val == 0
}

// IsNegative returns true if the BigInt is negative.
func (b *BigInt) IsNegative() bool {
	return b.Val < 0
}

// IsPositive returns true if the BigInt is positive.
func (b *BigInt) IsPositive() bool {
	return b.Val > 0
}

// IsEqual returns true if the current BigInt is equal to the given BigInt.
func (b *BigInt) IsEqual(other *BigInt) bool {
	return b.Val == other.Val
}

// IsGreaterThan returns true if the current BigInt is greater than the given BigInt.
func (b *BigInt) IsGreaterThan(other *BigInt) bool {
	return b.Val > other.Val
}

// IsLessThan returns true if the current BigInt is less than the given BigInt.
func (b *BigInt) IsLessThan(other *BigInt) bool {
	return b.Val < other.Val
}

// Abs returns the absolute value of the BigInt.
func (b *BigInt) Abs() *BigInt {
	return &BigInt{Val: int64(math.Abs(float64(b.Val)))}
}

// Neg returns the negation of the BigInt.
func (b *BigInt) Neg() *BigInt {
	return &BigInt{Val: -b.Val}
}

// Bitwise AND operation
func (b *BigInt) And(other *BigInt) *BigInt {
	return &BigInt{Val: b.Val & other.Val}
}

// Bitwise OR operation
func (b *BigInt) Or(other *BigInt) *BigInt {
	return &BigInt{Val: b.Val | other.Val}
}

// Bitwise XOR operation
func (b *BigInt) Xor(other *BigInt) *BigInt {
	return &BigInt{Val: b.Val ^ other.Val}
}

// Not returns the bitwise complement of the BigInt.
func (b *BigInt) Not() *BigInt {
	return &BigInt{Val: ^b.Val}
}

// Shl shifts the BigInt left by the given number of bits.
func (b *BigInt) Shl(n uint) *BigInt {
	return &BigInt{Val: b.Val << n}
}

// Bitwise shift right operation
// Shr shifts the bits of the BigInt to the right by the given number of positions.
// It returns a new BigInt with the shifted bits.
func (b *BigInt) Shr(n uint) *BigInt {
	return &BigInt{Val: b.Val >> n}
}

// Helper functions

// parseInt64 parses the given string as a 64-bit integer in base 10.
// It returns the parsed integer and any error that occurred during parsing.
func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}

// formatInt64 converts an int64 value to a string representation.
func formatInt64(val int64) string {
	return strconv.FormatInt(val, 10)
}

// encodeToBase64 encodes the given byte slice to a base64 string.
func encodeToBase64(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
