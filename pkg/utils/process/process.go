// Package process offers abstractions for the execution of processes
package process

import (
	"os/exec"
)

// Executor offers methods for running processes
// It facilitates the testing of components that execute processes by providing
// mock implementations.
type Executor interface {
	// Exec executes a process and waits for its completion, returning
	// the combined stdout and stdout
	Exec(cmd string, args ...string) ([]byte, error)
}

// An instance of an executor that uses the os/exec package for
// executing processes
type executor struct{}

// DefaultExecutor returns a default executor
func DefaultExecutor() Executor {
	return &executor{}
}

// Exec executes a process and returns the combined stdout and stdin
func (e *executor) Exec(cmd string, args ...string) ([]byte, error) {
	return exec.Command(cmd, args...).CombinedOutput()
}
