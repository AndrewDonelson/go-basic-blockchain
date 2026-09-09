// Package sdk is a software development kit for building blockchain applications.
// File sdk/bigint.go - wrapper for big integers with custom methods suchs as String(), Bytes() and Base64()

package sdk

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"fmt"
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
//
// Short input is left-padded rather than panicking: binary.BigEndian.Uint64
// requires exactly 8 bytes and used to panic with "index out of range" on
// anything shorter, which was reachable from NewBigIntFromString.
func NewBigIntFromBytes(b []byte) *BigInt {
	var buf [8]byte
	if len(b) >= 8 {
		copy(buf[:], b[len(b)-8:])
	} else {
		copy(buf[8-len(b):], b)
	}
	// Reinterpreting the bits, not converting the value: Bytes() writes the same
	// eight bytes back, so the round trip is exact for every input.
	return &BigInt{Val: int64(binary.BigEndian.Uint64(buf[:]))} //nolint:gosec // exact bit round trip
}

// NewBigIntFromString creates a new BigInt instance from a string representation.
// If the input string is base64-encoded, it will be decoded first.
func NewBigIntFromString(s string) (*BigInt, error) {
	// Try the decimal form first. The old code called isBase64Encoded() first,
	// which returns true for any string that happens to decode -- "1234" does --
	// so ordinary decimal input was reinterpreted as base64 and corrupted (and,
	// being shorter than 8 bytes, panicked in NewBigIntFromBytes).
	if val, err := parseInt64(s); err == nil {
		return &BigInt{Val: val}, nil
	}

	decoded, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		return nil, fmt.Errorf("value %q is neither a decimal integer nor base64: %w", s, err)
	}
	return NewBigIntFromBytes(decoded), nil
}

// String returns the string representation of the BigInt.
func (b *BigInt) String() string {
	return formatInt64(b.Val)
}

// Bytes returns the byte representation of the BigInt.
func (b *BigInt) Bytes() []byte {
	bytes := make([]byte, 8)
	//nolint:gosec // bit reinterpretation; NewBigIntFromBytes reverses it exactly
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
	//nolint:gosec // any 64-bit pattern is a valid random value here
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
// Division by zero returns zero rather than panicking.
func (b *BigInt) Divide(other *BigInt) *BigInt {
	if other == nil || other.Val == 0 {
		return &BigInt{Val: 0}
	}
	// math.MinInt64 / -1 overflows and panics on some architectures.
	if b.Val == math.MinInt64 && other.Val == -1 {
		return &BigInt{Val: math.MinInt64}
	}
	return &BigInt{Val: b.Val / other.Val}
}

// Modulo returns the modulo of the current BigInt by the given BigInt.
// A zero modulus returns zero rather than panicking.
func (b *BigInt) Modulo(other *BigInt) *BigInt {
	if other == nil || other.Val == 0 {
		return &BigInt{Val: 0}
	}
	if b.Val == math.MinInt64 && other.Val == -1 {
		return &BigInt{Val: 0}
	}
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
//
// It no longer routes through float64, which silently lost precision above 2^53
// and returned the wrong answer for math.MinInt64. MinInt64 has no positive
// counterpart in int64, so it is clamped to MaxInt64.
func (b *BigInt) Abs() *BigInt {
	if b.Val == math.MinInt64 {
		return &BigInt{Val: math.MaxInt64}
	}
	if b.Val < 0 {
		return &BigInt{Val: -b.Val}
	}
	return &BigInt{Val: b.Val}
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
