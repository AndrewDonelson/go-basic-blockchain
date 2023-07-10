package sdk

import (
	"errors"
	"fmt"
	"os"
	"reflect"
)

var (
	// ErrNoArgs is returned when the program is started with no arguments
	ErrNoArgs = errors.New("no arguments")
	Args      *Arguments
)

// Arguments handles parsing arguments and displaying usage information
type Arguments struct {
	Arguments map[string]*Argument
}

// Argument contains information for every argument
type Argument struct {
	name         string                 // name of the argument
	desc         string                 // description of the argument
	defaultVal   interface{}            // default value of the argument
	target       interface{}            // target of the argument (where to store the value)
	parent       string                 // Parent argument name. ie. --wallet create || lock || delete || ect...
	isRegistered bool                   // is the argument registered
	subCommands  map[string]*SubCommand // sub-commands for this argument
}

// SubCommand contains information for a sub command
type SubCommand struct {
	name       string       // name of the sub-command
	desc       string       // description of the sub-command
	defaultVal interface{}  // default value of the sub-command
	target     *interface{} // target of the sub-command (where to store the value)
}

func init() {
	Args = NewArguments()
}

func NewArguments() *Arguments {
	return &Arguments{
		Arguments: make(map[string]*Argument),
	}
}

// Register registers argument Arguments
func (a *Arguments) Register(name string, desc string, target interface{}, defaultVal interface{}) error {
	if _, ok := a.Arguments[name]; ok {
		return fmt.Errorf("Argument %q is already registered", name)
	}

	a.Arguments[name] = &Argument{
		name:         name,
		desc:         desc,
		defaultVal:   defaultVal,
		target:       target,
		isRegistered: true,
		subCommands:  make(map[string]*SubCommand),
	}

	return nil
}

// RegisterSubCommand registers a sub-command to a main command
func (a *Arguments) RegisterSubCommand(parent string, name string, desc string, target interface{}, defaultVal interface{}) error {
	if mainCmd, ok := a.Arguments[parent]; ok {
		if _, ok := mainCmd.subCommands[name]; ok {
			return fmt.Errorf("SubCommand %q is already registered under Argument %q", name, parent)
		}

		mainCmd.subCommands[name] = &SubCommand{
			name:       name,
			desc:       desc,
			defaultVal: defaultVal,
			target:     &target,
		}

		return nil
	}

	return fmt.Errorf("parent Argument %q is not registered", parent)
}

// Parse parses all registered arguments and handles assigning values or displaying usage info
func (a *Arguments) Parse() error {
	// Check if arguments were provided
	if len(os.Args) == 1 {
		// No arguments given, print usage
		a.printUsage()
		return ErrNoArgs
	}

	// Iterate over all the arguments
	for i := 1; i < len(os.Args); i++ {
		arg := os.Args[i]
		if f, ok := a.Arguments[arg]; ok {
			// Argument is registered, parse it
			f.target = os.Args[i+1]
			// Use reflect package to write variable
			reflect.ValueOf(f.target).Elem().Set(reflect.ValueOf(f.defaultVal))
			// Parse any sub commands if they exist
			if len(f.subCommands) > 0 {
				if len(os.Args) > i+1 {
					a.ParseSubCommands(f, os.Args[i+2:])
				}
				break
			}
		} else {
			// Unknown Argument, print usage and return error
			a.printUsage()
			return fmt.Errorf("unknown Argument %q", arg)
		}
	}

	return nil
}

// ParseSubCommands parses all registered sub-commands for a given command
func (a *Arguments) ParseSubCommands(cmd *Argument, args []string) error {
	for i := 0; i < len(args); i += 2 {
		arg := args[i]
		if f, ok := cmd.subCommands[arg]; ok {
			// Argument is registered, parse it
			*f.target = args[i+1]
			// Use reflect package to write variable
			reflect.ValueOf(f.target).Elem().Set(reflect.ValueOf(f.defaultVal))
		} else {
			// Unknown Argument, print usage and return error
			a.printUsage()
			return fmt.Errorf("unknown SubCommand %q for Argument %q", arg, cmd.name)
		}
	}

	return nil
}

// printUsage prints usage information for all registered arguments
func (a *Arguments) printUsage() {
	for _, f := range a.Arguments {
		fmt.Printf("-%s\t\t%s (default: %v)\n", f.name, f.desc, f.defaultVal)
		for _, sc := range f.subCommands {
			fmt.Printf("\t-%s\t\t%s (default: %v)\n", sc.name, sc.desc, sc.defaultVal)
		}
	}
}
