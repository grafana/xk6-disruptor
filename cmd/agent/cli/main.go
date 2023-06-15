// Package vli implements the root level command for the disruptor agent CLI
package main

import (
	"fmt"
	"os"

	"github.com/grafana/xk6-disruptor/cmd/agent"
	"github.com/grafana/xk6-disruptor/cmd/agent/cli/commands"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/spf13/cobra"
)

func main() {
	env := runtime.DefaultEnvironment()
	config := &agent.AgentConfig{
		Profiler: &runtime.ProfilerConfig{},
	}
	agent := agent.BuildAgent(env, config)

	rootCmd := BuildRootCmd(config)
	rootCmd.AddCommand(commands.BuildHTTPCmd(agent))
	rootCmd.AddCommand(commands.BuildGrpcCmd(agent))

	rootArgs := env.Args()[1:]
	rootCmd.SetArgs(rootArgs)

	err := rootCmd.Execute()
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}

// BuildRootCmd builds the root command for the agent that parses the configuration arguments
func BuildRootCmd(c *agent.AgentConfig) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "xk6-disruptor-agent",
		Short: "Inject disruptions in a system",
		Long: "A command for injecting disruptions in a target system.\n" +
			"It can run as stand-alone process or in a container",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().BoolVar(&c.Profiler.CPUProfile, "cpu-profile", false, "profile agent execution")
	rootCmd.PersistentFlags().StringVar(&c.Profiler.CPUProfileFileName, "cpu-profile-file", "cpu.pprof",
		"cpu profiling output file")
	rootCmd.PersistentFlags().BoolVar(&c.Profiler.MemProfile, "mem-profile", false, "profile agent memory")
	rootCmd.PersistentFlags().StringVar(&c.Profiler.MemProfileFileName, "mem-profile-file", "mem.pprof",
		"memory profiling output file")
	rootCmd.PersistentFlags().IntVar(&c.Profiler.MemProfileRate, "mem-profile-rate", 1, "memory profiling rate")
	rootCmd.PersistentFlags().BoolVar(&c.Profiler.Trace, "trace", false, "trace agent execution")
	rootCmd.PersistentFlags().StringVar(&c.Profiler.TraceFileName, "trace-file", "trace.out", "tracing output file")

	return rootCmd
}
