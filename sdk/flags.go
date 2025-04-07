// Package sdk is a software development kit for building blockchain applications.
// File sdk/flags.go - Flags for the blockchain
package sdk

import (
	"errors"
	"flag"
	"fmt"
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
}

// Arguments handles parsing arguments and displaying usage information
type Arguments struct {
	Flags map[string]*Flag
}

func init() {
	Args = NewArguments()

	// Register new command-line flags for seed node functionality
	Args.Register("seed", "Run as a seed node", true)
	Args.Register("seed-address", "Address of the seed node to connect to", "")
}

// NewArguments creates a new Arguments instance
func NewArguments() *Arguments {
	return &Arguments{
		Flags: make(map[string]*Flag),
	}
}

// Register registers a new flag with an optional default value
func (a *Arguments) Register(name string, desc string, defaultVal interface{}) error {
	flag := &Flag{
		Name:        name,
		Description: desc,
		DefaultVal:  defaultVal,
		SubCommands: make(map[string]*SubCommand),
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
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NFlag() == 0 {
		return ErrNoArgs
	}

	// Check for unknown arguments
	seen := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		seen[f.Name] = true
	})

	// Check if any of the flags are unknown
	for name := range seen {
		if _, ok := a.Flags[name]; !ok {
			return fmt.Errorf("%w: %q", ErrUnknownArg, "--"+name)
		}
	}

	// Check for sub-commands
	args := flag.Args()
	if len(args) > 0 {
		// Try to find a flag that has this sub-command
		for _, arg := range args {
			if !strings.HasPrefix(arg, "--") {
				continue
			}

			found := false
			for flagName, flag := range a.Flags {
				if seen[flagName] {
					if _, ok := flag.SubCommands[arg]; ok {
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

// PrintUsage prints usage information for all registered arguments
func (a *Arguments) PrintUsage() {
	log.Println("Usage:")
	flag.PrintDefaults()
}

// GetBool returns the boolean value of the named flag
func (a *Arguments) GetBool(name string) bool {
	if f, ok := a.Flags[name]; ok {
		return *(f.Value.(*bool))
	}
	return false
}

// GetString returns the string value of the named flag
func (a *Arguments) GetString(name string) string {
	if f, ok := a.Flags[name]; ok {
		return *f.Value.(*string)
	}
	return ""
}

// GetInt returns the int value of the named flag
func (a *Arguments) GetInt(name string) int {
	if f, ok := a.Flags[name]; ok {
		return *f.Value.(*int)
	}
	return 0
}

// GetInt64 returns the int64 value of the named flag
func (a *Arguments) GetInt64(name string) int64 {
	if f, ok := a.Flags[name]; ok {
		return *f.Value.(*int64)
	}
	return 0
}

// GetFloat64 returns the float64 value of the named flag
func (a *Arguments) GetFloat64(name string) float64 {
	if f, ok := a.Flags[name]; ok {
		return *f.Value.(*float64)
	}
	return 0
}

// Flag methods for different types

func (f *Flag) Bool(name string, value bool, usage string) *bool {
	return flag.Bool(name, value, usage)
}

func (f *Flag) String(name string, value string, usage string) *string {
	return flag.String(name, value, usage)
}

func (f *Flag) Int(name string, value int, usage string) *int {
	return flag.Int(name, value, usage)
}

func (f *Flag) Int64(name string, value int64, usage string) *int64 {
	return flag.Int64(name, value, usage)
}

func (f *Flag) Float64(name string, value float64, usage string) *float64 {
	return flag.Float64(name, value, usage)
}
