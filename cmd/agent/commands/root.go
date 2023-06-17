package commands

import (
	"context"

	"github.com/grafana/xk6-disruptor/pkg/agent"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/grafana/xk6-disruptor/pkg/runtime/profiler"

	"github.com/spf13/cobra"
)

// RootCommand maintains the state for executing a command on the Agent
type RootCommand struct {
	cmd *cobra.Command
	env runtime.Environment
}

// NewRootCommand builds the for the agent that parses the configuration arguments
func NewRootCommand(env runtime.Environment) *RootCommand {
	config := &agent.Config{
		Profiler: &profiler.Config{},
	}

	rootCmd := buildRootCmd(config)
	rootCmd.AddCommand(BuildHTTPCmd(env, config))
	rootCmd.AddCommand(BuildGrpcCmd(env, config))

	return &RootCommand{
		cmd: rootCmd,
		env: env,
	}
}

// Execute executes the RootCommand
func (c *RootCommand) Execute(ctx context.Context) error {
	rootArgs := c.env.Args()[1:]
	c.cmd.SetArgs(rootArgs)
	c.cmd.SetContext(ctx)

	return c.cmd.Execute()
}

func buildRootCmd(c *agent.Config) *cobra.Command {
	rootCmd := &cobra.Command{
		Use:   "xk6-disruptor-agent",
		Short: "Inject disruptions in a system",
		Long: "A command for injecting disruptions in a target system.\n" +
			"It can run as stand-alone process or in a container",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().BoolVar(&c.Profiler.CPU.Enabled, "cpu-profile", false, "profile agent execution")
	rootCmd.PersistentFlags().StringVar(&c.Profiler.CPU.FileName, "cpu-profile-file", "cpu.pprof",
		"cpu profiling output file")
	rootCmd.PersistentFlags().BoolVar(&c.Profiler.Memory.Enabled, "mem-profile", false, "profile agent memory")
	rootCmd.PersistentFlags().StringVar(&c.Profiler.Memory.FileName, "mem-profile-file", "mem.pprof",
		"memory profiling output file")
	rootCmd.PersistentFlags().IntVar(&c.Profiler.Memory.Rate, "mem-profile-rate", 1, "memory profiling rate")
	rootCmd.PersistentFlags().BoolVar(&c.Profiler.Trace.Enabled, "trace", false, "trace agent execution")
	rootCmd.PersistentFlags().StringVar(&c.Profiler.Trace.FileName, "trace-file", "trace.out", "tracing output file")

	return rootCmd
}
