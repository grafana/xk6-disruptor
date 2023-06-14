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
	// Lock returns an interface for a process lock
	Lock() Lock
	// Profiler return an execution profiler
	Profiler() Profiler
	// Vars returns the environment variables
	Vars() map[string]string
	// Args returns the arguments passed to the process
	Args() []string
}

// environment keeps the state of the execution environment
type environment struct {
	executor Executor
	lock     Lock
	profiler Profiler
	vars     map[string]string
	args     []string
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
	args := os.Args
	vars := getEnv()
	return &environment{
		executor: DefaultExecutor(),
		profiler: DefaultProfiler(),
		lock:     DefaultLock(),
		vars:     vars,
		args:     args,
	}
}

func (e *environment) Executor() Executor {
	return e.executor
}

func (e *environment) Lock() Lock {
	return e.lock
}

func (e *environment) Profiler() Profiler {
	return e.profiler
}

func (e *environment) Vars() map[string]string {
	return e.vars
}

func (e *environment) Args() []string {
	return e.args
}
