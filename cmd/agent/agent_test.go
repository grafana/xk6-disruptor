package agent

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

// BuildNoop returns a function that return the given error after a delay
func BuildNoop(delay time.Duration, err error) func(context.Context) error {
	return func(ctx context.Context) error {
		//TODO: handle context cancellation
		time.Sleep(delay)
		return err
	}
}

func Test_CancelContext(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title    string
		vars     map[string]string
		args     []string
		delay    time.Duration
		err      error
		config   *AgentConfig
		expected error
	}{
		{
			title: "Command is not canceled",
			vars:  map[string]string{},
			args:  []string{},
			delay: 0 * time.Second,
			err:   nil,
			config: &AgentConfig{
				Profiler: &runtime.ProfilerConfig{},
			},
			expected: nil,
		},
		{
			title: "Command is canceled",
			delay: 5 * time.Second,
			err:   nil,
			vars:  map[string]string{},
			args:  []string{},
			config: &AgentConfig{
				Profiler: &runtime.ProfilerConfig{},
			},
			expected: context.Canceled,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()
			env := runtime.NewFakeRuntime(tc.args, tc.vars)

			agent := BuildAgent(env, tc.config)

			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				time.Sleep(1 * time.Second)
				cancel()
			}()

			cmd := BuildNoop(tc.delay, tc.err)
			err := agent.do(ctx, cmd)
			if !errors.Is(err, tc.expected) {
				t.Errorf("expected %v got %v", tc.err, err)
			}
		})
	}
}

func Test_Signals(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title     string
		args      []string
		vars      map[string]string
		delay     time.Duration
		err       error
		config    *AgentConfig
		signal    syscall.Signal
		expectErr bool
	}{
		{
			title: "Command is canceled with interrupt",
			args:  []string{},
			vars:  map[string]string{},
			delay: 5 * time.Second,
			err:   nil,
			config: &AgentConfig{
				Profiler: &runtime.ProfilerConfig{},
			},
			signal:    syscall.SIGINT,
			expectErr: true,
		},
		{
			title: "Command is not canceled with interrupt",
			args:  []string{},
			vars:  map[string]string{},
			delay: 5 * time.Second,
			err:   nil,
			config: &AgentConfig{
				Profiler: &runtime.ProfilerConfig{},
			},
			signal:    0,
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()
			env := runtime.NewFakeRuntime(tc.args, tc.vars)

			agent := BuildAgent(env, tc.config)

			go func() {
				time.Sleep(1 * time.Second)
				if tc.signal != 0 {
					env.FakeSignal.Send(tc.signal)
				}
			}()

			cmd := BuildNoop(tc.delay, tc.err)
			err := agent.do(context.Background(), cmd)
			if tc.expectErr && err == nil {
				t.Errorf("should had failed")
				return
			}

			if !tc.expectErr && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
		})
	}
}
