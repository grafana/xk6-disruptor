// Package main implements the root level command for the disruptor agent CLI
package main

import (
	"fmt"
	"os"

	"github.com/grafana/xk6-disruptor/cmd/agent/commands"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "xk6-disruptor-agent",
		Short: "Inject disruptions in a system",
		Long: "A command for injecting disruptions in a target system.\n" +
			"It can run as stand-alone process or in a container",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.AddCommand(commands.BuildHTTPCmd())
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
