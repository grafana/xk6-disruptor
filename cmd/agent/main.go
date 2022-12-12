// Package main implements the root level command for the disruptor agent CLI
package main

import (
	"fmt"
	"os"

	"github.com/grafana/xk6-disruptor/cmd/agent/commands"
	"github.com/grafana/xk6-disruptor/pkg/utils/process"
	"github.com/spf13/cobra"
)

const lockFile = "/var/run/xk6-disruptor"

func main() {
	rootCmd := &cobra.Command{
		Use:   "xk6-disruptor-agent",
		Short: "Inject disruptions in a system",
		Long: "A command for injecting disruptions in a target system.\n" +
			"It can run as stand-alone process or in a container",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			acquired, err := process.Lock(lockFile)
			if err != nil {
				return fmt.Errorf("error creating lock file: %w", err)
			}
			if !acquired {
				return fmt.Errorf("another disruptor command is already in execution")
			}

			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			_ = process.Unlock(lockFile)
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(commands.BuildHTTPCmd())
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
