// Package util implements misc utilities for network tests
package util

import (
	"bufio"
	"context"
	"strings"
	"testing"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/exec"
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

// TCExec runs a command on a container in a shell, echoing the output and failing the test if it cannot be run.
func TCExec(t *testing.T, c testcontainers.Container, shellcmd string) {
	t.Helper()

	cmd := []string{"/bin/sh", "-c", shellcmd}

	t.Logf("%s: running %q", c.GetContainerID(), shellcmd)
	rc, out, err := c.Exec(context.TODO(), cmd, exec.Multiplexed())
	if err != nil {
		t.Fatalf("running command on %s: %v", c.GetContainerID(), err)
	}

	if rc != 0 {
		t.Errorf("%s:%s exited with %d", c.GetContainerID(), cmd, rc)
	}

	go func() {
		buf := bufio.NewReader(out)
		for {
			line, err := buf.ReadString('\n')
			if err != nil {
				return
			}

			t.Logf("%s:%s: %s", c.GetContainerID(), cmd, strings.TrimSpace(line))
		}
	}()
}
