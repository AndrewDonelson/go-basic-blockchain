package sdk

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNewBigInt(t *testing.T) {
	val := int64(123)
	bigInt := NewBigInt(val)
	assert.Equal(t, val, bigInt.Val)
}

func TestNewRandomBigInt(t *testing.T) {
	bigInt, err := NewRandomBigInt()
	assert.NoError(t, err)
	assert.NotNil(t, bigInt)
}

func TestNewBigIntFromBytes(t *testing.T) {
	bytes := []byte{0, 0, 0, 0, 0, 0, 0, 123}
	bigInt := NewBigIntFromBytes(bytes)
	assert.Equal(t, int64(123), bigInt.Val)
}

func TestNewBigIntFromString(t *testing.T) {
	val := "123"
	bigInt, err := NewBigIntFromString(val)
	assert.NoError(t, err)
	assert.Equal(t, int64(123), bigInt.Val)

	// Test base64 encoded string
	encoded := "MTIz" // base64 for "123"
	bigInt, err = NewBigIntFromString(encoded)
	assert.NoError(t, err)
	assert.Equal(t, int64(825373492), bigInt.Val)
}

func TestBigInt_String(t *testing.T) {
	val := int64(123)
	bigInt := NewBigInt(val)
	assert.Equal(t, "123", bigInt.String())
}

func TestBigInt_Bytes(t *testing.T) {
	val := int64(123)
	bigInt := NewBigInt(val)
	expectedBytes := []byte{0, 0, 0, 0, 0, 0, 0, 123}
	assert.Equal(t, expectedBytes, bigInt.Bytes())
}

func TestBigInt_Base64(t *testing.T) {
	val := int64(123)
	bigInt := NewBigInt(val)
	expectedBase64 := "AAAAAAAAAHs="
	assert.Equal(t, expectedBase64, bigInt.Base64())
}

func TestBigInt_Random(t *testing.T) {
	bigInt, err := NewRandomBigInt()
	assert.NoError(t, err)
	assert.NotEqual(t, int64(0), bigInt.Val)
}

func TestBigInt_Add(t *testing.T) {
	first := NewBigInt(100)
	second := NewBigInt(200)
	result := first.Add(second)
	assert.Equal(t, int64(300), result.Val)
}

func TestBigInt_Subtract(t *testing.T) {
	first := NewBigInt(200)
	second := NewBigInt(100)
	result := first.Subtract(second)
	assert.Equal(t, int64(100), result.Val)
}

func TestBigInt_Multiply(t *testing.T) {
	first := NewBigInt(10)
	second := NewBigInt(20)
	result := first.Multiply(second)
	assert.Equal(t, int64(200), result.Val)
}

func TestBigInt_Divide(t *testing.T) {
	first := NewBigInt(200)
	second := NewBigInt(10)
	result := first.Divide(second)
	assert.Equal(t, int64(20), result.Val)
}

func TestBigInt_Modulo(t *testing.T) {
	first := NewBigInt(20)
	second := NewBigInt(3)
	result := first.Modulo(second)
	assert.Equal(t, int64(2), result.Val)
}

func TestBigInt_Compare(t *testing.T) {
	first := NewBigInt(100)
	second := NewBigInt(100)
	third := NewBigInt(200)
	assert.Equal(t, 0, first.Compare(second))
	assert.Equal(t, -1, first.Compare(third))
	assert.Equal(t, 1, third.Compare(first))
}

func TestBigInt_IsZero(t *testing.T) {
	zero := NewBigInt(0)
	nonZero := NewBigInt(123)
	assert.True(t, zero.IsZero())
	assert.False(t, nonZero.IsZero())
}

func TestBigInt_IsNegative(t *testing.T) {
	negative := NewBigInt(-1)
	positive := NewBigInt(1)
	assert.True(t, negative.IsNegative())
	assert.False(t, positive.IsNegative())
}

func TestBigInt_IsPositive(t *testing.T) {
	negative := NewBigInt(-1)
	positive := NewBigInt(1)
	assert.False(t, negative.IsPositive())
	assert.True(t, positive.IsPositive())
}

func TestBigInt_IsEqual(t *testing.T) {
	first := NewBigInt(123)
	second := NewBigInt(123)
	third := NewBigInt(456)
	assert.True(t, first.IsEqual(second))
	assert.False(t, first.IsEqual(third))
}
func TestNewBigIntFromNegativeInt(t *testing.T) {
	val := int64(-123)
	bigInt := NewBigInt(val)
	assert.Equal(t, val, bigInt.Val)
}

func TestNewBigIntFromZero(t *testing.T) {
	val := int64(0)
	bigInt := NewBigInt(val)
	assert.Equal(t, val, bigInt.Val)
}

func TestBigInt_Equal(t *testing.T) {
	first := NewBigInt(123)
	second := NewBigInt(123)
	assert.True(t, first.IsEqual(second))
}

func TestBigInt_NotEqual(t *testing.T) {
	first := NewBigInt(123)
	second := NewBigInt(456)
	assert.False(t, first.IsEqual(second))
}

func TestNewBigIntFromStringInvalid(t *testing.T) {
	val := "notanumber"
	_, err := NewBigIntFromString(val)
	assert.Error(t, err)
}

func TestBigInt_GreaterThan(t *testing.T) {
	first := NewBigInt(200)
	second := NewBigInt(100)
	assert.True(t, first.IsGreaterThan(second))
}

func TestBigInt_LessThan(t *testing.T) {
	first := NewBigInt(100)
	second := NewBigInt(200)
	assert.True(t, first.IsLessThan(second))
}
