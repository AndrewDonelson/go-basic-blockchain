package sdk

import (
	"encoding/json"
	"fmt"
	"reflect"
)

// PrettyPrint is used to display any type nicely in the log output
func PrettyPrint(v interface{}) string {

	name := GetType(v)
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return ""
	}

	return fmt.Sprintf("Dump of [%s]:\n%s\n", name, string(b))
}

// GetType will return the name of the provided interface using reflection
func GetType(i interface{}) string {
	t := reflect.TypeOf(i)
	if t.Kind() == reflect.Ptr {
		return "*" + t.Elem().Name()
	}

	return t.Name()
}
