// Package process offers abstractions for the execution of processes
// It facilitates the testing of components that execute processes by providing
// mock implementations.
package process

import (
	"os/exec"
)

// ProcessExecutor offers methods for running processes
type ProcessExecutor interface {
	// Exec executes a process and waits for its completion, returning
	// the combined stdout and stdout
	Exec(cmd string, args ...string) ([]byte, error)
}

// An instance of a process executor that uses the os/exec package for
// executing processes
type executor struct{}

// DefaultProcessExecutor returns a default process executor
func DefaultProcessExecutor() ProcessExecutor {
	return &executor{}
}

// Exec executes a process and returns the combined stdout and stdin
func (e *executor) Exec(cmd string, args ...string) ([]byte, error) {
	return exec.Command(cmd, args...).CombinedOutput()
}
