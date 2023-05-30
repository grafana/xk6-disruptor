// Package main implements the root level command for the disruptor agent CLI
package main

import (
	"fmt"
	"os"

	"github.com/grafana/xk6-disruptor/cmd/agent/commands"
)

func main() {
	rootCmd := commands.BuildRootCmd()

	rootCmd.AddCommand(commands.BuildHTTPCmd())
	rootCmd.AddCommand(commands.BuildGrpcCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
