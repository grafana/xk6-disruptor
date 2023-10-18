// Package intstr implements custom types
package intstr

import (
	"fmt"
	"strconv"
)

// ValueType defines the type of a IntOrString value
type ValueType int

const (
	// IntValue is a IntOrString that represents an integer value
	IntValue ValueType = iota
	// StringValue is a IntOrString 	that represents a string value
	StringValue
)

// IntOrString holds a value that can be either a string or a int
type IntOrString string

// NullValue is an empty IntOrString value
const NullValue = IntOrString("")

// Type returns the ValueType of a IntOrString value
func (value IntOrString) Type() ValueType {
	if _, err := strconv.Atoi(string(value)); err == nil {
		return IntValue
	}
	return StringValue
}

// IsInt returns true if the value is an integer
func (value IntOrString) IsInt() bool {
	_, err := strconv.Atoi(string(value))
	return err == nil
}


// Int32 returns the value of the IntOrString as an int32.
// If the current value is not an string, 0 is returned
func (value IntOrString) Int32() int32 {
	int64Value, err := strconv.ParseInt(string(value), 10, 32)
	if err != nil {
		panic(fmt.Errorf("invalid int32 value %s", value))
	}

	return int32(int64Value)
}


// Str returns the value of the IntOrString as a string.
func (value IntOrString) Str() string {
	return string(value)
}

// FromInt return a IntOrString from a int
func FromInt(value int) IntOrString {
	strValue := fmt.Sprintf("%d", value)
	return IntOrString(strValue)
}

// FromInt32 return a IntOrString from a int32
func FromInt32(value int32) IntOrString {
	strValue := fmt.Sprintf("%d", value)
	return IntOrString(strValue)
}


// FromString return a IntOrString from a string
func FromString(value string) IntOrString {
	return IntOrString(value)
}
