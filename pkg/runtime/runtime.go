// Package runtime abstracts the execution environment of a process
package runtime

// Environment abstracts the execution environment of a process.
// It allows introduction mocks for testing.
type Environment interface {
	// Executor returns a process executor that abstracts os.Exec
	Executor() Executor
	// Lock returns an interface for managing the process execution
	Process() Process
}

// environment keeps the state of the execution environment
type environment struct {
	executor Executor
	process  Process
}

// DefaultEnvironment returns the default execution environment
func DefaultEnvironment() Environment {
	return environment{
		executor: DefaultExecutor(),
		process:  DefaultProcess(),
	}
}

func (e environment) Executor() Executor {
	return e.executor
}

func (e environment) Process() Process {
	return e.process
}
