// main is the entry point for the application. It registers a command-line argument
// named "aString" with a default value of "Hello World!" and then parses the
// command-line arguments.
package main

import (
	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
)

func main() {
	aString := ""
	sdk.Args.Register("aString", "a test string", aString, "Hello World!")
	sdk.Args.Parse()
}
