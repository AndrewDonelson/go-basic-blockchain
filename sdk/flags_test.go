package sdk_test

import (
	"os"
	"testing"

	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
)

// This test will:
//
// - Check if no arguments are passed.
// - Check if an invalid argument is passed.
// - Check if a valid argument without any sub-command is passed.
// - Check if a valid argument with valid sub-commands is passed.
// - Check if a valid argument with an invalid sub-command is passed.
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
			errValue: "unknown Argument \"--arg3\"",
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
			errValue: "unknown SubCommand \"--invalidSubCmd\" for Argument \"--arg1\"",
		},
	}

	var (
		strVal1, strVal2   string
		intVal1, intVal2   int
		fltVal1, fltVal2   float64
		boolVal1, boolVal2 bool
	)

	// Register main arguments
	if err := sdk.Args.Register("--arg1", "arg1 desc", &strVal1, ""); err != nil {
		t.Fatalf("Failed to register arg1: %v", err)
	}

	if err := sdk.Args.Register("--arg2", "arg2 desc", &strVal2, ""); err != nil {
		t.Fatalf("Failed to register arg2: %v", err)
	}

	// Register sub-commands for main arguments
	if err := sdk.Args.RegisterSubCommand("--arg1", "--strVal", "strVal desc", &strVal1, "I am String #1"); err != nil {
		t.Fatalf("Failed to register arg1 sub-command strVal: %v", err)
	}
	if err := sdk.Args.RegisterSubCommand("--arg1", "--intVal", "intVal desc", &intVal1, 1); err != nil {
		t.Fatalf("Failed to register arg1 sub-command intVal: %v", err)
	}
	if err := sdk.Args.RegisterSubCommand("--arg1", "--fltVal", "fltVal desc", &fltVal1, 1.23); err != nil {
		t.Fatalf("Failed to register arg1 sub-command fltVal: %v", err)
	}
	if err := sdk.Args.RegisterSubCommand("--arg1", "--boolVal", "boolVal desc", &boolVal1, true); err != nil {
		t.Fatalf("Failed to register arg1 sub-command boolVal: %v", err)
	}

	if err := sdk.Args.RegisterSubCommand("--arg2", "--strVal", "strVal desc", &strVal2, "I am String #2"); err != nil {
		t.Fatalf("Failed to register arg2 sub-command strVal: %v", err)
	}
	if err := sdk.Args.RegisterSubCommand("--arg2", "--intVal", "intVal desc", &intVal2, 2); err != nil {
		t.Fatalf("Failed to register arg2 sub-command intVal: %v", err)
	}
	if err := sdk.Args.RegisterSubCommand("--arg2", "--fltVal", "fltVal desc", &fltVal2, 4.56); err != nil {
		t.Fatalf("Failed to register arg2 sub-command fltVal: %v", err)
	}
	if err := sdk.Args.RegisterSubCommand("--arg2", "--boolVal", "boolVal desc", &boolVal2, false); err != nil {
		t.Fatalf("Failed to register arg2 sub-command boolVal: %v", err)
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			os.Args = test.args
			err := sdk.Args.Parse()

			if (err != nil) != test.wantErr {
				t.Errorf("Arguments.Parse() error = %v, wantErr %v", err, test.wantErr)
				return
			}

			if err != nil && err.Error() != test.errValue {
				t.Errorf("Arguments.Parse() error value = %v, wantErrorValue %v", err.Error(), test.errValue)
			}
		})
	}
}
