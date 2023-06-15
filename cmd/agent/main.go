// Package vli implements the root level command for the disruptor agent CLI
package main

import (
	"fmt"
	"os"

	"github.com/grafana/xk6-disruptor/cmd/agent/commands"
	"github.com/grafana/xk6-disruptor/pkg/agent"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

func main() {
	env := runtime.DefaultEnvironment()
	config := &agent.Config{
		Profiler: &runtime.ProfilerConfig{},
	}
	agent := agent.BuildAgent(env, config)

	rootCmd := commands.BuildRootCmd(config)
	rootCmd.AddCommand(commands.BuildHTTPCmd(agent))
	rootCmd.AddCommand(commands.BuildGrpcCmd(agent))

	rootArgs := env.Args()[1:]
	rootCmd.SetArgs(rootArgs)

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
