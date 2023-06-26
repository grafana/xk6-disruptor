package utils

import (
	"os"
	"strconv"
)

// GetBooleanEnvVar returns a boolean environment variable.
// If variable is not set or invalid value, returns the default value
func GetBooleanEnvVar(envVar string, defaultValue bool) bool {
	autoCleanup, err := strconv.ParseBool(os.Getenv(envVar))
	if err != nil {
		return defaultValue
	}
	return autoCleanup
}
