package sdk_test

import (
	"os"
	"testing"

	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestArguments_Register tests the Register method
func TestArguments_Register(t *testing.T) {
	// Create a fresh Arguments instance for this test
	args := sdk.NewArguments()

	// Register a boolean flag
	err := args.Register("test-bool", "Test boolean flag", true)
	assert.NoError(t, err, "Register should not return an error for boolean flag")

	// Register a string flag
	err = args.Register("test-string", "Test string flag", "default")
	assert.NoError(t, err, "Register should not return an error for string flag")

	// Register an int flag
	err = args.Register("test-int", "Test int flag", 42)
	assert.NoError(t, err, "Register should not return an error for int flag")

	// Register a float64 flag
	err = args.Register("test-float", "Test float flag", 3.14)
	assert.NoError(t, err, "Register should not return an error for float flag")
}

// TestArguments_RegisterSubCommand tests the RegisterSubCommand method
func TestArguments_RegisterSubCommand(t *testing.T) {
	// Create a fresh Arguments instance for this test
	args := sdk.NewArguments()

	// Register a parent flag first
	err := args.Register("parent", "Parent flag", true)
	require.NoError(t, err, "Register should not return an error for parent flag")

	// Register a sub-command for the parent flag
	err = args.RegisterSubCommand("parent", "sub-bool", "Sub boolean flag", true, true)
	assert.NoError(t, err, "RegisterSubCommand should not return an error for boolean sub-command")

	// Register a string sub-command
	err = args.RegisterSubCommand("parent", "sub-string", "Sub string flag", "value", "default")
	assert.NoError(t, err, "RegisterSubCommand should not return an error for string sub-command")

	// Try to register a sub-command for a non-existent flag
	err = args.RegisterSubCommand("non-existent", "sub", "Sub flag", true, true)
	assert.Error(t, err, "RegisterSubCommand should return an error for non-existent flag")
}

// TestArguments_Parse tests the Parse method
func TestArguments_Parse(t *testing.T) {
	// Reset os.Args for each test case
	origArgs := os.Args
	defer func() { os.Args = origArgs }()

	tests := []struct {
		name     string
		args     []string
		wantErr  bool
		errValue string
	}{
		{
			name:     "Test No Arguments",
			args:     []string{"test"},
			wantErr:  true,
			errValue: "no arguments",
		},
		{
			name:     "Test Invalid Argument",
			args:     []string{"test", "--arg3"},
			wantErr:  true,
			errValue: "unknown argument: \"--arg3\"",
		},
		{
			name:     "Test Argument With No SubCommand",
			args:     []string{"test", "--arg1"},
			wantErr:  false,
			errValue: "",
		},
		{
			name:     "Test Argument With SubCommand",
			args:     []string{"test", "--arg1", "--strVal", "test string", "--intVal", "5", "--fltVal", "5.5", "--boolVal", "true"},
			wantErr:  false,
			errValue: "",
		},
		{
			name:     "Test Argument With Invalid SubCommand",
			args:     []string{"test", "--arg1", "--invalidSubCmd"},
			wantErr:  true,
			errValue: "unknown sub-command \"--invalidSubCmd\" for Argument \"--arg1\"",
		},
	}

	// Create a fresh Arguments instance for this test
	args := sdk.NewArguments()

	var (
		strVal1, strVal2   string
		intVal1, intVal2   int
		fltVal1, fltVal2   float64
		boolVal1, boolVal2 bool
	)

	// Register main arguments
	err := args.Register("arg1", "arg1 desc", "")
	require.NoError(t, err, "Failed to register arg1")

	err = args.Register("arg2", "arg2 desc", "")
	require.NoError(t, err, "Failed to register arg2")

	// Register sub-commands for main arguments
	err = args.RegisterSubCommand("arg1", "strVal", "strVal desc", &strVal1, "I am String #1")
	require.NoError(t, err, "Failed to register arg1 sub-command strVal")

	err = args.RegisterSubCommand("arg1", "intVal", "intVal desc", &intVal1, 1)
	require.NoError(t, err, "Failed to register arg1 sub-command intVal")

	err = args.RegisterSubCommand("arg1", "fltVal", "fltVal desc", &fltVal1, 1.23)
	require.NoError(t, err, "Failed to register arg1 sub-command fltVal")

	err = args.RegisterSubCommand("arg1", "boolVal", "boolVal desc", &boolVal1, true)
	require.NoError(t, err, "Failed to register arg1 sub-command boolVal")

	err = args.RegisterSubCommand("arg2", "strVal", "strVal desc", &strVal2, "I am String #2")
	require.NoError(t, err, "Failed to register arg2 sub-command strVal")

	err = args.RegisterSubCommand("arg2", "intVal", "intVal desc", &intVal2, 2)
	require.NoError(t, err, "Failed to register arg2 sub-command intVal")

	err = args.RegisterSubCommand("arg2", "fltVal", "fltVal desc", &fltVal2, 4.56)
	require.NoError(t, err, "Failed to register arg2 sub-command fltVal")

	err = args.RegisterSubCommand("arg2", "boolVal", "boolVal desc", &boolVal2, false)
	require.NoError(t, err, "Failed to register arg2 sub-command boolVal")

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			os.Args = test.args
			err := args.Parse()

			if test.wantErr {
				assert.Error(t, err, "Expected an error but got none")
				if err != nil {
					assert.Contains(t, err.Error(), test.errValue,
						"Error doesn't contain expected value. Got: %v, Want: %v", err.Error(), test.errValue)
				}
			} else {
				assert.NoError(t, err, "Expected no error but got: %v", err)
			}
		})
	}
}

// TestArguments_GetMethods tests the various Get methods
func TestArguments_GetMethods(t *testing.T) {
	// Create a fresh Arguments instance for this test
	args := sdk.NewArguments()

	// Register flags of different types
	err := args.Register("bool-flag", "Boolean flag", true)
	require.NoError(t, err)

	err = args.Register("string-flag", "String flag", "test-value")
	require.NoError(t, err)

	err = args.Register("int-flag", "Int flag", 42)
	require.NoError(t, err)

	err = args.Register("int64-flag", "Int64 flag", int64(9223372036854775807))
	require.NoError(t, err)

	err = args.Register("float-flag", "Float flag", 3.14159)
	require.NoError(t, err)

	// Test GetBool
	boolValue := args.GetBool("bool-flag")
	assert.True(t, boolValue, "GetBool should return true for bool-flag")

	// Test GetString
	stringValue := args.GetString("string-flag")
	assert.Equal(t, "test-value", stringValue, "GetString should return 'test-value' for string-flag")

	// Test GetInt
	intValue := args.GetInt("int-flag")
	assert.Equal(t, 42, intValue, "GetInt should return 42 for int-flag")

	// Test GetInt64
	int64Value := args.GetInt64("int64-flag")
	assert.Equal(t, int64(9223372036854775807), int64Value, "GetInt64 should return 9223372036854775807 for int64-flag")

	// Test GetFloat64
	floatValue := args.GetFloat64("float-flag")
	assert.Equal(t, 3.14159, floatValue, "GetFloat64 should return 3.14159 for float-flag")

	// Test getting values for non-existent flags
	assert.False(t, args.GetBool("non-existent"), "GetBool should return false for non-existent flag")
	assert.Equal(t, "", args.GetString("non-existent"), "GetString should return empty string for non-existent flag")
	assert.Equal(t, 0, args.GetInt("non-existent"), "GetInt should return 0 for non-existent flag")
	assert.Equal(t, int64(0), args.GetInt64("non-existent"), "GetInt64 should return 0 for non-existent flag")
	assert.Equal(t, float64(0), args.GetFloat64("non-existent"), "GetFloat64 should return 0.0 for non-existent flag")
}
