package api

import "strings"

// toGoCase transforms an identifier to its camel case
// maps 'fieldName' and 'field_name' to 'FieldName'
func toGoCase(name string) string {
	goCase := ""
	for _, world := range strings.Split(name, "_") {
		runes := []rune(world)
		first := strings.ToUpper(string(runes[0]))
		goCase = goCase + first + string(runes[1:])
	}

	return goCase
}