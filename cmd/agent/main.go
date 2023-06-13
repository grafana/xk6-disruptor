// Package main implements the root level command for the disruptor agent CLI
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/grafana/xk6-disruptor/cmd/agent/commands"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/spf13/cobra"
)

func main() {
	env := runtime.DefaultEnvironment()
	ctx := context.Background()

	subcommands := []*cobra.Command{
		commands.BuildHTTPCmd(env),
		commands.BuildGrpcCmd(env),
	}

	rootCmd := commands.BuildRootCmd(env, subcommands)
	if err := rootCmd.Do(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
