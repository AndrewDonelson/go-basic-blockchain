// Package sdk is a software development kit for building blockchain applications.
// File sdk/flags.go - Flags for the blockchain
package sdk

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"strings"
)

var (
	// ErrNoArgs is returned when the program is started with no arguments
	ErrNoArgs = errors.New("no arguments")

	// ErrUnknownArg is returned when an unknown argument is specified
	ErrUnknownArg = errors.New("unknown argument")

	// ErrUnknownSubCmd is returned when an unknown sub-command is specified
	ErrUnknownSubCmd = errors.New("unknown sub-command")

	// Args is the global Arguments instance
	Args *Arguments
)

// SubCommand contains information for sub-commands of an argument
type SubCommand struct {
	Name        string      // name of the sub-command
	Description string      // description of the sub-command
	Value       interface{} // value of the sub-command
	DefaultVal  interface{} // default value of the sub-command
}

// Flag contains information for every argument
type Flag struct {
	Name        string                 // name of the argument
	Description string                 // description of the argument
	Value       interface{}            // value of the argument
	DefaultVal  interface{}            // default value of the argument
	SubCommands map[string]*SubCommand // sub-commands for this argument

	// set is the FlagSet this flag was registered into.
	set *flag.FlagSet
}

// Arguments handles parsing arguments and displaying usage information
//
// Each instance owns a private FlagSet rather than registering into the global
// flag.CommandLine. Two things went wrong with the global:
//
//  1. Registering the same name twice panics, so a second Arguments in one
//     process -- which is what a test does -- brought the process down. That is
//     why TestArguments_Parse was skipped.
//  2. The testing package registers its own -test.* flags on the same global set,
//     so a library parsing it either choked on them or consumed them.
//
// The set also uses ContinueOnError. flag.CommandLine defaults to ExitOnError,
// which calls os.Exit on a bad argument -- unacceptable in a library.
type Arguments struct {
	Flags map[string]*Flag
	set   *flag.FlagSet
}

func init() {
	Args = NewArguments()

	// Register new command-line flags for seed node functionality
	// Default false. This was registered with a default of `true`, so every node
	// ran as a seed node unless explicitly told otherwise.
	if err := Args.Register("seed", "Run as a seed node", false); err != nil {
		// Log error but continue
		_ = err // Suppress unused variable warning
	}
	if err := Args.Register("seed-address", "Address of the seed node to connect to", ""); err != nil {
		// Log error but continue
		_ = err // Suppress unused variable warning
	}

	// Add new flag for environment file path
	if err := Args.Register("env", "Path to the .env file", ""); err != nil {
		// Log error but continue
		_ = err // Suppress unused variable warning
	}

	// Add verbose flag
	if err := Args.Register("verbose", "Enable verbose logging", false); err != nil {
		// Log error but continue
		_ = err // Suppress unused variable warning
	}
}

// NewArguments creates a new Arguments instance with its own FlagSet.
func NewArguments() *Arguments {
	name := "chaind"
	if len(os.Args) > 0 {
		name = os.Args[0]
	}
	set := flag.NewFlagSet(name, flag.ContinueOnError)
	// Silence the set's own error printing; Parse returns the error and the
	// caller decides what to show.
	set.SetOutput(io.Discard)

	return &Arguments{
		Flags: make(map[string]*Flag),
		set:   set,
	}
}

// flagSet returns the instance's set, creating one if the Arguments was built as
// a bare struct literal.
func (a *Arguments) flagSet() *flag.FlagSet {
	if a.set == nil {
		a.set = flag.NewFlagSet("chaind", flag.ContinueOnError)
		a.set.SetOutput(io.Discard)
	}
	return a.set
}

// Register registers a new flag with an optional default value
func (a *Arguments) Register(name string, desc string, defaultVal interface{}) error {
	if _, exists := a.Flags[name]; exists {
		return fmt.Errorf("flag %q is already registered", name)
	}

	flag := &Flag{
		Name:        name,
		Description: desc,
		DefaultVal:  defaultVal,
		SubCommands: make(map[string]*SubCommand),
		set:         a.flagSet(),
	}
	a.Flags[name] = flag

	switch v := defaultVal.(type) {
	case bool:
		flag.Value = flag.Bool(name, v, desc)
	case string:
		flag.Value = flag.String(name, v, desc)
	case int:
		flag.Value = flag.Int(name, v, desc)
	case int64:
		flag.Value = flag.Int64(name, v, desc)
	case float64:
		flag.Value = flag.Float64(name, v, desc)
	default:
		return fmt.Errorf("unsupported type for default value")
	}

	return nil
}

// RegisterSubCommand registers a sub-command for an existing flag
func (a *Arguments) RegisterSubCommand(flagName string, name string, desc string, value interface{}, defaultVal interface{}) error {
	flag, ok := a.Flags[flagName]
	if !ok {
		return fmt.Errorf("flag %s does not exist", flagName)
	}

	subCommand := &SubCommand{
		Name:        name,
		Description: desc,
		DefaultVal:  defaultVal,
	}
	flag.SubCommands[name] = subCommand

	switch v := defaultVal.(type) {
	case bool:
		subCommand.Value = flag.Bool(name, v, desc)
	case string:
		subCommand.Value = flag.String(name, v, desc)
	case int:
		subCommand.Value = flag.Int(name, v, desc)
	case int64:
		subCommand.Value = flag.Int64(name, v, desc)
	case float64:
		subCommand.Value = flag.Float64(name, v, desc)
	default:
		return fmt.Errorf("unsupported type for default value")
	}

	return nil
}

// Parse parses the command-line arguments
func (a *Arguments) Parse() error {
	args := os.Args
	if len(args) > 1 {
		args = args[1:]
	} else {
		args = nil
	}
	return a.ParseArgs(args)
}

// ParseArgs parses an explicit argument list.
//
// Separated from Parse so a caller -- a test, or a program embedding this -- can
// supply arguments without rewriting the process-wide os.Args.
func (a *Arguments) ParseArgs(argv []string) error {
	set := a.flagSet()
	set.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", set.Name())
		set.PrintDefaults()
	}

	if err := set.Parse(argv); err != nil {
		// An unrecognised flag is reported here rather than by the loop below;
		// the set refuses to parse past it.
		if strings.Contains(err.Error(), "not defined") {
			return fmt.Errorf("%w: %v", ErrUnknownArg, err)
		}
		return err
	}

	if set.NFlag() == 0 {
		return ErrNoArgs
	}

	// Check for unknown arguments
	seen := make(map[string]bool)
	set.Visit(func(f *flag.Flag) {
		seen[f.Name] = true
	})

	// Check if any of the flags are unknown.
	//
	// A registered sub-command counts as known. Sub-commands live in their
	// parent's SubCommands map rather than a.Flags, so checking only a.Flags
	// meant every sub-command that was actually used on the command line came
	// back as an unknown argument -- the one thing registering it was supposed
	// to prevent.
	for name := range seen {
		if a.isKnownLocked(name) {
			continue
		}
		return fmt.Errorf("%w: %q", ErrUnknownArg, "--"+name)
	}

	// Check for sub-commands
	subArgs := set.Args()
	if len(subArgs) > 0 {
		// Try to find a flag that has this sub-command
		for _, arg := range subArgs {
			if !strings.HasPrefix(arg, "--") {
				continue
			}

			// Sub-commands are registered under their bare name; the token
			// carries a "--" prefix. Comparing them unstripped meant even a
			// valid sub-command was reported as unknown.
			name := strings.TrimPrefix(arg, "--")

			found := false
			for flagName, flag := range a.Flags {
				if seen[flagName] {
					if _, ok := flag.SubCommands[name]; ok {
						found = true
						break
					}
				}
			}

			if !found {
				for flagName := range seen {
					return fmt.Errorf("%w %q for Argument %q", ErrUnknownSubCmd, arg, "--"+flagName)
				}
			}
		}
	}

	return nil
}

// isKnownLocked reports whether a name is a registered flag or sub-command.
func (a *Arguments) isKnownLocked(name string) bool {
	if _, ok := a.Flags[name]; ok {
		return true
	}
	for _, f := range a.Flags {
		if _, ok := f.SubCommands[name]; ok {
			return true
		}
	}
	return false
}

// PrintUsage prints usage information for all registered arguments
func (a *Arguments) PrintUsage() {
	log.Println("Usage:")
	a.flagSet().SetOutput(os.Stderr)
	a.flagSet().PrintDefaults()
	a.flagSet().SetOutput(io.Discard)
}

// GetBool returns the boolean value of the named flag
func (a *Arguments) GetBool(name string) bool {
	if f, ok := a.Flags[name]; ok {
		// Comma-ok: a mistyped flag registration used to panic here rather than
		// falling back to the zero value.
		if v, ok := f.Value.(*bool); ok {
			return *v
		}
	}
	return false
}

// GetString returns the string value of the named flag
func (a *Arguments) GetString(name string) string {
	if f, ok := a.Flags[name]; ok {
		if v, ok := f.Value.(*string); ok {
			return *v
		}
	}
	return ""
}

// GetInt returns the int value of the named flag
func (a *Arguments) GetInt(name string) int {
	if f, ok := a.Flags[name]; ok {
		if v, ok := f.Value.(*int); ok {
			return *v
		}
	}
	return 0
}

// GetInt64 returns the int64 value of the named flag
func (a *Arguments) GetInt64(name string) int64 {
	if f, ok := a.Flags[name]; ok {
		if v, ok := f.Value.(*int64); ok {
			return *v
		}
	}
	return 0
}

// GetFloat64 returns the float64 value of the named flag
func (a *Arguments) GetFloat64(name string) float64 {
	if f, ok := a.Flags[name]; ok {
		if v, ok := f.Value.(*float64); ok {
			return *v
		}
	}
	return 0
}

// Flag methods for different types

// These register into the flag's own set, never the global flag.CommandLine.

func (f *Flag) flagSet() *flag.FlagSet {
	if f.set == nil {
		f.set = flag.NewFlagSet("chaind", flag.ContinueOnError)
		f.set.SetOutput(io.Discard)
	}
	return f.set
}

func (f *Flag) Bool(name string, value bool, usage string) *bool {
	return f.flagSet().Bool(name, value, usage)
}

func (f *Flag) String(name string, value string, usage string) *string {
	return f.flagSet().String(name, value, usage)
}

func (f *Flag) Int(name string, value int, usage string) *int {
	return f.flagSet().Int(name, value, usage)
}

func (f *Flag) Int64(name string, value int64, usage string) *int64 {
	return f.flagSet().Int64(name, value, usage)
}

func (f *Flag) Float64(name string, value float64, usage string) *float64 {
	return f.flagSet().Float64(name, value, usage)
}
