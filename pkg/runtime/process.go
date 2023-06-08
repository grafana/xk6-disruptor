package runtime

import (
	"fmt"
	"os"
	"path/filepath"
)

// Process controls the process execution
type Process interface {
	// Name returns the name of the process
	Name() string
	// Lock tries to acquire an execution lock to prevent concurrent executions.
	// Returns error if lock is already acquired by another process.
	Lock() error
	// Unlock releases the execution lock
	Unlock() error
}

// process maintains the state of the process
type process struct {
	name string
	lock string
	env  map[string]string
}

// DefaultProcess create a new Process for the currently running process.
// When the process exits, the onExit function is executed.
func DefaultProcess(args []string, env map[string]string) Process {
	return &process{
		name: filepath.Base(args[0]),
		env:  env,
	}
}

func (p *process) Name() string {
	return p.name
}

func (p *process) getLockDir() string {
	// get runtime directory for user
	lockDir := p.env["XDG_RUNTIME_DIR"]
	if lockDir == "" {
		lockDir = os.TempDir()
	}

	return lockDir
}

func (p *process) Lock() error {
	if p.lock == "" {
		p.lock = filepath.Join(p.getLockDir(), p.name)
	}

	acquired, err := Lock(p.lock)
	if err != nil {
		return fmt.Errorf("failed to acquire lock file for process %q: %w", p.name, err)
	}
	if !acquired {
		return fmt.Errorf("another process %q is already in execution", p.name)
	}

	return nil
}

func (p *process) Unlock() error {
	return Unlock(p.lock)
}
