package main

import (
	"github.com/AndrewDonelson/go-basic-blockchain/sdk"
)

func main() {
	aString := ""
	sdk.Args.Register("aString", "a test string", aString, "Hellow World!")
	sdk.Args.Parse()
}
