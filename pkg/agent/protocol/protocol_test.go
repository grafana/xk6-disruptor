package protocol

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

// FakeTrafficRedirector is fake TrafficRedirector for testing
// it keeps tracks of method invocations
type FakeTrafficRedirector struct {
	err     error
	Started bool
	Stopped bool
}

// FakeTrafficRedirector returns a fake TrafficRedirector for testing
func NewFakeTrafficRedirector() *FakeTrafficRedirector {
	return &FakeTrafficRedirector{}
}

// FakeTrafficRedirector returns a fake TrafficRedirector for testing that returns an error on Start
func NewFakeTrafficRedirectorWithError(err error) *FakeTrafficRedirector {
	return &FakeTrafficRedirector{
		err: err,
	}
}

func (f *FakeTrafficRedirector) Start() error {
	f.Started = true
	return f.err
}

func (f *FakeTrafficRedirector) Stop() error {
	f.Stopped = true
	return nil
}

// FakeProxy is fake Proxy for testing
// it keeps tracks of method invocations
type FakeProxy struct {
	err     error
	Started bool
	Stopped bool
	Forced  bool
}

// FakeTrafficRedirector returns a fake TrafficRedirector for testing
func NewFakeProxy() *FakeProxy {
	return &FakeProxy{}
}

// FakeTrafficRedirector returns a fake TrafficRedirector for testing that returns an error on Start
func NewFakeFakeProxyWithError(err error) *FakeProxy {
	return &FakeProxy{
		err: err,
	}
}

func (f *FakeProxy) Start() error {
	f.Started = true
	return f.err
}

func (f *FakeProxy) Stop() error {
	f.Stopped = true
	return nil
}

func (f *FakeProxy) Force() error {
	f.Forced = true
	return f.err
}

func Test_ContextHandling(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		test     string
		deadline time.Duration
		expected error
	}{
		{
			test:     "context cancelled (deadline)",
			deadline: time.Second,
			expected: context.DeadlineExceeded,
		},
		{
			test:     "context not cancelled",
			deadline: 5 * time.Second,
			expected: nil,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			executor := runtime.NewFakeRuntime(nil, nil).Executor()
			tr := NewFakeTrafficRedirector()
			proxy := NewFakeProxy()

			disruptor, err := NewDisruptor(executor, proxy, tr)
			if err != nil {
				t.Errorf("could not initialize test %v", err)
				return
			}

			//nolint:govet // in a text it is not necessary to cancel this context
			ctx, _ := context.WithTimeout(context.TODO(), tc.deadline)
			err = disruptor.Apply(ctx, 2*time.Second)

			if !errors.Is(err, tc.expected) {
				t.Errorf("expected %v got %v", tc.expected, err)
				return
			}

			if !tr.Stopped {
				t.Errorf("should had stopped traffic redirector")
				return
			}

			if !proxy.Stopped {
				t.Errorf("should had stopped proxy")
				return
			}
		})
	}
}
