// Package main implements the root level command for the disruptor agent CLI
package main

import (
	"fmt"
	"os"

	"github.com/grafana/xk6-disruptor/cmd/agent/commands"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

func main() {
	env := runtime.DefaultEnvironment()
	rootCmd := commands.BuildRootCmd(env)

	rootCmd.AddCommand(commands.BuildHTTPCmd(env))
	rootCmd.AddCommand(commands.BuildGrpcCmd(env))

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
