// Package runtime abstracts the execution environment of a process
package runtime

import (
	"os"
	"strings"
)

// Environment abstracts the execution environment of a process.
// It allows introduction mocks for testing.
type Environment interface {
	// Executor returns a process executor that abstracts os.Exec
	Executor() Executor
	// Lock returns an interface for managing the process execution
	Process() Process
	// Profiler return an execution profiler
	Profiler() Profiler
}

// environment keeps the state of the execution environment
type environment struct {
	executor Executor
	process  Process
	profiler Profiler
}

// returns a map with the environment variables
func getEnv() map[string]string {
	env := map[string]string{}
	for _, e := range os.Environ() {
		k, v, _ := strings.Cut(e, "=")
		env[k] = v
	}

	return env
}

// DefaultEnvironment returns the default execution environment
func DefaultEnvironment() Environment {
	return &environment{
		executor: DefaultExecutor(),
		process:  DefaultProcess(os.Args, getEnv()),
		profiler: DefaultProfiler(),
	}
}

func (e *environment) Executor() Executor {
	return e.executor
}

func (e *environment) Process() Process {
	return e.process
}

func (e *environment) Profiler() Profiler {
	return e.profiler
}
