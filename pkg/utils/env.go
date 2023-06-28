package utils

import (
	"os"
	"strconv"
)

// GetBooleanEnvVar returns a boolean environment variable.
// If variable is not set or invalid value, returns the default value
func GetBooleanEnvVar(envVar string, defaultValue bool) bool {
	value, err := strconv.ParseBool(os.Getenv(envVar))
	if err != nil {
		return defaultValue
	}
	return value
}

// GetEnvVar returns a string environment variable.
// If variable is not set or has an empty value, returns the default value
func GetEnvVar(envVar string, defaultValue string) string {
	value := os.Getenv(envVar)
	if value == "" {
		return defaultValue
	}
	return value
}
