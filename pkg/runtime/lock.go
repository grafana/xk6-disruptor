package runtime

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"syscall"
)

// Lock defines a process lock
type Lock interface {
	// Acquire tries to acquire an execution lock to prevent concurrent executions.
	// Returns false if lock is already acquired by another process.
	Acquire() (bool, error)
	// Release releases the execution lock
	Release() error
}

// filelock maintains the state of a file based lock
type filelock struct {
	path string
}

// DefaultLock create a new Lock for the currently running process
func DefaultLock() Lock {
	name := filepath.Base(os.Args[0])

	// get runtime directory for user
	lockDir := os.Getenv("XDG_RUNTIME_DIR")
	if lockDir == "" {
		lockDir = os.TempDir()
	}

	return &filelock{
		path: filepath.Join(lockDir, name),
	}
}

// NewFileLock returns a file lock for the given path
func NewFileLock(path string) Lock {
	return &filelock{
		path: path,
	}
}

// Acquire tries to acquire an execution lock for the given file.
// Returns true if lock is acquired.
func (l *filelock) Acquire() (bool, error) {
	tempLock, err := createTempLock(l.path)
	if err != nil {
		return false, err
	}

	// clean up
	defer func() {
		tempLockFile, errDefer := os.Stat(tempLock)
		// we did not create the temp lock file, nothing to do here
		if os.IsNotExist(errDefer) {
			return
		}

		// unexpected, abort
		if errDefer != nil {
			panic("unexpected error cleaning up lock file")
		}

		lockFile, errDefer := os.Stat(l.path)
		// if the lock was not created or we did not acquire the lock, remove the temp lock
		if os.IsNotExist(errDefer) || !os.SameFile(lockFile, tempLockFile) {
			_ = os.Remove(tempLock)
		}
	}()

	err = os.Link(tempLock, l.path)

	// some other process already own the lock, let's check this is a legit lock
	if os.IsExist(err) {
		owner, errOwner := getOwner(l.path)
		if errOwner != nil {
			return false, fmt.Errorf("could not get lock owner: %w", err)
		}

		// process is the current owner
		if owner == os.Getpid() {
			return true, nil
		}

		alive := isAlive(owner)
		if alive {
			return false, nil
		}

		// owner is not alive, remove file and try again
		err = os.Remove(l.path)
		if err != nil {
			return false, err
		}
		err = os.Link(tempLock, l.path)
	}

	if err != nil {
		return false, err
	}

	return true, nil
}

// Unlock releases the ownership of a lock.
// Returns an error if the invoking process is not the current owner
func (l *filelock) Release() error {
	owner, err := getOwner(l.path)
	if err != nil {
		return err
	}

	if owner != os.Getpid() {
		return fmt.Errorf("process is not owner of lock file")
	}

	return os.Remove(l.path)
}

// getOwner returns the owner of the lockfile.
// return -1 if the owner is invalid (e.g the file is empty)
func getOwner(path string) (int, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return -1, err
	}

	if len(content) == 0 {
		return -1, nil
	}

	var pid int
	_, err = fmt.Sscanf(string(content), "%d", &pid)
	if err != nil {
		//nolint:nilerr  // return value -1 covers case of error scanning pid
		return -1, nil
	}

	return pid, nil
}

// isAlive checks if the process with the given pid is running
// a non-existing process (-1) is considered not running
func isAlive(pid int) bool {
	if pid == -1 {
		return false
	}
	// get process, ignore error it is always nil
	process, _ := os.FindProcess(pid)

	// send fake signal just to check if process exists
	err := process.Signal(syscall.Signal(0))

	return err == nil
}

// createTempLock creates a temporary lock file
func createTempLock(path string) (string, error) {
	pid := os.Getpid()
	tempLockFile := fmt.Sprintf("%s.%d", path, pid)
	tempLock, err := os.Create(tempLockFile)
	if err != nil {
		return "", err
	}

	_, err = io.WriteString(tempLock, fmt.Sprintf("%d", pid))
	if err != nil {
		_ = tempLock.Close()
		_ = os.Remove(tempLockFile)
		return "", err
	}

	return tempLockFile, nil
}
