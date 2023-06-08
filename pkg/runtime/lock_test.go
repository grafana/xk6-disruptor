package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// execute a shell and return its pid. On return, the child should had finished
func getDeadProcessPid() string {
	ls := exec.Command("sh", "-c", "cat /proc/self/stat | cut -d' ' -f 1")
	pid, err := ls.Output()
	if err != nil {
		panic("")
	}

	return string(pid)
}

func Test_Lock(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title         string
		createLock    bool
		ownerPid      string
		expectError   bool
		expectedValue bool
	}{
		{
			title:         "lock does not exist",
			createLock:    false,
			ownerPid:      "",
			expectError:   false,
			expectedValue: true,
		},
		{
			title:         "lock with empty owner",
			createLock:    true,
			ownerPid:      "",
			expectError:   false,
			expectedValue: true,
		},
		{
			title:         "process is already owner",
			createLock:    true,
			ownerPid:      fmt.Sprintf("%d", os.Getpid()),
			expectError:   false,
			expectedValue: true,
		},
		{
			title:         "lock with other running owner",
			createLock:    true,
			ownerPid:      fmt.Sprintf("%d", os.Getppid()),
			expectError:   false,
			expectedValue: false,
		},
		{
			title:         "lock with owner not running",
			createLock:    true,
			ownerPid:      getDeadProcessPid(),
			expectError:   false,
			expectedValue: true,
		},
	}

	for i, tc := range testCases {
		tc := tc
		i := i
		tmpDir := t.TempDir()

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			// create file name for test lock file
			testLock := filepath.Join(tmpDir, fmt.Sprintf("test-lockfile.%d", i))
			defer func() {
				_ = os.Remove(testLock)
			}()

			if tc.createLock {
				lockFile, err := os.Create(testLock)
				if err != nil {
					t.Errorf("error in test setup: %t", err)
					return
				}

				_, err = lockFile.Write([]byte(tc.ownerPid))
				if err != nil {
					t.Errorf("error in test setup: %t", err)
					return
				}
			}

			locked, err := Lock(testLock)
			if err != nil && !tc.expectError {
				t.Errorf("failed: %t", err)
				return
			}

			if err == nil && tc.expectError {
				t.Errorf("Should had failed")
				return
			}

			if err != nil && locked != tc.expectedValue {
				t.Errorf("expected %t but received %t", tc.expectedValue, locked)
			}
		})
	}
}

func Test_Unlock(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		createLock  bool
		ownerPid    string
		expectError bool
	}{
		{
			title:       "process is owner",
			createLock:  true,
			ownerPid:    fmt.Sprintf("%d", os.Getpid()),
			expectError: false,
		},
		{
			title:       "lock does not exist",
			createLock:  false,
			ownerPid:    "",
			expectError: true,
		},
		{
			title:       "lock with empty owner",
			createLock:  true,
			ownerPid:    "",
			expectError: true,
		},
		{
			title:       "lock with other owner",
			createLock:  true,
			ownerPid:    fmt.Sprintf("%d", os.Getppid()),
			expectError: true,
		},
	}

	for i, tc := range testCases {
		tc := tc
		i := i
		tmpDir := t.TempDir()

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			// create file name for test lock file
			testLock := filepath.Join(tmpDir, fmt.Sprintf("test-lockfile.%d", i))
			defer func() {
				_ = os.Remove(testLock)
			}()

			if tc.createLock {
				lockFile, err := os.Create(testLock)
				if err != nil {
					t.Errorf("error in test setup: %t", err)
					return
				}

				_, err = lockFile.Write([]byte(tc.ownerPid))
				if err != nil {
					t.Errorf("error in test setup: %t", err)
					return
				}
			}

			err := Unlock(testLock)
			if err != nil && !tc.expectError {
				t.Errorf("failed: %t", err)
				return
			}

			if err == nil && tc.expectError {
				t.Errorf("Should had failed")
				return
			}
		})
	}
}
