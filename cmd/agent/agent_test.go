package main

import (
	"context"
	"errors"
	"syscall"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/spf13/cobra"
)

// BuildNoopCmd returns a corba.Command that returns after the given delay
func BuildNoopCmd(args []string) *cobra.Command {
	var delay time.Duration

	cmd := &cobra.Command{
		Use: "noop",
		RunE: func(cmd *cobra.Command, args []string) error {
			time.Sleep(delay)
			return nil
		},
	}

	cmd.Flags().DurationVarP(&delay, "delay", "d", 0, "delay of the disruptions")
	cmd.SetArgs(args[1:])
	return cmd
}

func Test_CancelContext(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title  string
		args   []string
		vars   map[string]string
		config *AgentConfig
		err    error
	}{
		{
			title: "Command is not canceled",
			args:  []string{"noop", "-d", "0s"},
			vars:  map[string]string{},
			config: &AgentConfig{
				profiler: &runtime.ProfilerConfig{},
			},
			err: nil,
		},
		{
			title: "Command is canceled",
			args:  []string{"noop", "-d", "5s"},
			vars:  map[string]string{},
			config: &AgentConfig{
				profiler: &runtime.ProfilerConfig{},
			},
			err: context.Canceled,
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

			cmd := BuildNoopCmd(tc.args)
			err := agent.Do(ctx, cmd)
			if !errors.Is(err, tc.err) {
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
		config    *AgentConfig
		signal    syscall.Signal
		expectErr bool
	}{
		{
			title: "Command is canceled with interrupt",
			args:  []string{"noop", "-d", "5s"},
			vars:  map[string]string{},
			config: &AgentConfig{
				profiler: &runtime.ProfilerConfig{},
			},
			signal:    syscall.SIGINT,
			expectErr: true,
		},
		{
			title: "Command is not canceled with interrupt",
			args:  []string{"noop", "-d", "5s"},
			vars:  map[string]string{},
			config: &AgentConfig{
				profiler: &runtime.ProfilerConfig{},
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

			cmd := BuildNoopCmd(tc.args)
			err := agent.Do(context.Background(), cmd)
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
