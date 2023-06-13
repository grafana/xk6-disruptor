package commands

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/spf13/cobra"
)

// BuildNoopCmd returns a corba.Command that returns after the given delay
func BuildNoopCmd() *cobra.Command {
	var delay time.Duration

	cmd := &cobra.Command{
		Use: "noop",
		RunE: func(cmd *cobra.Command, args []string) error {
			time.Sleep(delay)
			return nil
		},
	}

	cmd.Flags().DurationVarP(&delay, "delay", "d", 0, "delay of the disruptions")
	return cmd
}

func Test_CancelContext(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title   string
		args    []string
		vars    map[string]string
		subcmds []*cobra.Command
		err     error
	}{
		{
			title: "Command is not canceled",
			args:  []string{"xk6-disruptor", "noop", "-d", "0s"},
			vars:  map[string]string{},
			subcmds: []*cobra.Command{
				BuildNoopCmd(),
			},
			err: nil,
		},
		{
			title: "Command is canceled",
			args:  []string{"xk6-disruptor", "noop", "-d", "5s"},
			vars:  map[string]string{},
			subcmds: []*cobra.Command{
				BuildNoopCmd(),
			},
			err: context.Canceled,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()
			env := runtime.NewFakeRuntime(tc.args, tc.vars)

			rootCmd := BuildRootCmd(env, tc.subcmds)

			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				time.Sleep(1 * time.Second)
				cancel()
			}()

			err := rootCmd.Do(ctx)
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
		subcmds   []*cobra.Command
		signal    os.Signal
		expectErr bool
	}{
		{
			title: "Command is canceled with interrupt",
			args:  []string{"xk6-disruptor", "noop", "-d", "5s"},
			vars:  map[string]string{},
			subcmds: []*cobra.Command{
				BuildNoopCmd(),
			},
			signal:    os.Interrupt,
			expectErr: true,
		},
		{
			title: "Command is not canceled with interrupt",
			args:  []string{"xk6-disruptor", "noop", "-d", "5s"},
			vars:  map[string]string{},
			subcmds: []*cobra.Command{
				BuildNoopCmd(),
			},
			signal:    nil,
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()
			env := runtime.NewFakeRuntime(tc.args, tc.vars)

			rootCmd := BuildRootCmd(env, tc.subcmds)

			go func() {
				time.Sleep(1 * time.Second)
				if tc.signal != nil {
					env.FakeSignal.SendSignal(tc.signal)
				}
			}()

			err := rootCmd.Do(context.Background())
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
