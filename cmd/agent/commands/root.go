package commands

import (
	"github.com/grafana/xk6-disruptor/pkg/agent"
	"github.com/spf13/cobra"
)

// BuildRootCmd builds the root command for the agent that parses the configuration arguments
func BuildRootCmd(c *agent.Config) *cobra.Command {
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
