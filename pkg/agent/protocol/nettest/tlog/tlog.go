// Package tlog implements a testcontainers log handler that mirrors logs to a test logger.
package tlog

import (
	"testing"

	"github.com/testcontainers/testcontainers-go"
)

// Mirror is a testcontainers log adapter that mirrors container output to testing.T.Log.
type Mirror struct {
	T    *testing.T
	Name string
}

// Accept implements the testcontainers adapter interface by writing received output to the test logger.
func (m Mirror) Accept(log testcontainers.Log) {
	prefix := ""
	if m.Name != "" {
		prefix += m.Name + "/"
	}
	prefix += log.LogType

	m.T.Logf("%s: %s", prefix, log.Content)
}
