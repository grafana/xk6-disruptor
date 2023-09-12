// Package k3sutils implements helper functions for tests using TestContainer's k3s module
package k3sutils

import (
	"context"
	"fmt"
	"regexp"
	"time"

	"github.com/testcontainers/testcontainers-go"
)

// regexMatcher process lines from logs and notifies when a match is found
type regexMatcher struct {
	exp   *regexp.Regexp
	found chan (bool)
}

// Accept implements the LogConsumer interface
func (a *regexMatcher) Accept(log testcontainers.Log) {
	if a.exp.MatchString(string(log.Content)) {
		a.found <- true
	}
}

// WaitForRegex waits until a match for the given regex is found in the log produced by the container
// This utility function is a workaround for https://github.com/testcontainers/testcontainers-go/issues/1541
func WaitForRegex(ctx context.Context, container testcontainers.Container, exp string, timeout time.Duration) error {
	regexp, err := regexp.Compile(exp)
	if err != nil {
		return err
	}

	found := make(chan bool)
	container.FollowOutput(&regexMatcher{
		exp:   regexp,
		found: found,
	})

	err = container.StartLogProducer(ctx)
	if err != nil {
		return err
	}
	defer func() {
		_ = container.StopLogProducer()
	}()

	expired := time.After(timeout)
	select {
	case <-found:
		return nil
	case <-expired:
		return fmt.Errorf("timeout waiting for a match")
	case <-ctx.Done():
		return ctx.Err()
	}
}
