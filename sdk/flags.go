// Package sdk is a software development kit for building blockchain applications.
// File sdk/flags.go - Flags for the blockchain
package sdk

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

var (
	// ErrNoArgs is returned when the program is started with no arguments
	ErrNoArgs = errors.New("no arguments")

	// Args is the global Arguments instance
	Args *Arguments
)

// Arguments handles parsing arguments and displaying usage information
type Arguments struct {
	Flags map[string]*Flag
}

// Flag contains information for every argument
type Flag struct {
	Name        string      // name of the argument
	Description string      // description of the argument
	Value       interface{} // value of the argument
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

// Register registers a new flag
func (a *Arguments) Register(name string, desc string, defaultVal interface{}) {
	flag := &Flag{
		Name:        name,
		Description: desc,
		Value:       defaultVal,
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
	}
}

func (a *Arguments) Parse() error {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()

	if flag.NFlag() == 0 {
		return ErrNoArgs
	}

	return nil
}

// PrintUsage prints usage information for all registered arguments
func (a *Arguments) PrintUsage() {
	fmt.Println("Usage:")
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
