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

	config := &AgentConfig{
		profiler: &runtime.ProfilerConfig{},
	}
	rootCmd := BuildRootCmd(config)
	for _, s := range subcommands {
		rootCmd.AddCommand(s)
	}

	rootArgs := env.Args()[1:]
	rootCmd.SetArgs(rootArgs)

	// parse root command arguments and identify target command
	cmd, _, err := rootCmd.Traverse(rootArgs)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}

	agent := BuildAgent(env, config)

	if err := agent.Do(ctx, cmd); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// BuildRootCmd builds the root command for the agent that parses the configuration arguments
func BuildRootCmd(c *AgentConfig) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "xk6-disruptor-agent",
		Short: "Inject disruptions in a system",
		Long: "A command for injecting disruptions in a target system.\n" +
			"It can run as stand-alone process or in a container",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().BoolVar(&c.profiler.CPUProfile, "cpu-profile", false, "profile agent execution")
	rootCmd.PersistentFlags().StringVar(&c.profiler.CPUProfileFileName, "cpu-profile-file", "cpu.pprof",
		"cpu profiling output file")
	rootCmd.PersistentFlags().BoolVar(&c.profiler.MemProfile, "mem-profile", false, "profile agent memory")
	rootCmd.PersistentFlags().StringVar(&c.profiler.MemProfileFileName, "mem-profile-file", "mem.pprof",
		"memory profiling output file")
	rootCmd.PersistentFlags().IntVar(&c.profiler.MemProfileRate, "mem-profile-rate", 1, "memory profiling rate")
	rootCmd.PersistentFlags().BoolVar(&c.profiler.Trace, "trace", false, "trace agent execution")
	rootCmd.PersistentFlags().StringVar(&c.profiler.TraceFileName, "trace-file", "trace.out", "tracing output file")

	return rootCmd
}
