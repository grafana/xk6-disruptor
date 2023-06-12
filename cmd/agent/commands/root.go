package commands

import (
	"context"
	"fmt"

	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/spf13/cobra"
)

// RootCommand maintains the state required for executing an agent command
type RootCommand struct {
	env            runtime.Environment
	cmd            *cobra.Command
	profilerConfig runtime.ProfilerConfig
}

// BuildRootCmd builds the root command for the agent with all the persistent flags.
// It also initializes/terminates the profiling if requested.
func BuildRootCmd(env runtime.Environment) *RootCommand {
	rootCmd := &cobra.Command{
		Use:   "xk6-disruptor-agent",
		Short: "Inject disruptions in a system",
		Long: "A command for injecting disruptions in a target system.\n" +
			"It can run as stand-alone process or in a container",
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	profilerConfig := runtime.ProfilerConfig{}

	rootCmd.PersistentFlags().BoolVar(&profilerConfig.CPUProfile, "cpu-profile", false, "profile agent execution")
	rootCmd.PersistentFlags().StringVar(&profilerConfig.CPUProfileFileName, "cpu-profile-file", "cpu.pprof",
		"cpu profiling output file")
	rootCmd.PersistentFlags().BoolVar(&profilerConfig.MemProfile, "mem-profile", false, "profile agent memory")
	rootCmd.PersistentFlags().StringVar(&profilerConfig.MemProfileFileName, "mem-profile-file", "mem.pprof",
		"memory profiling output file")
	rootCmd.PersistentFlags().IntVar(&profilerConfig.MemProfileRate, "mem-profile-rate", 1, "memory profiling rate")
	rootCmd.PersistentFlags().BoolVar(&profilerConfig.Trace, "trace", false, "trace agent execution")
	rootCmd.PersistentFlags().StringVar(&profilerConfig.TraceFileName, "trace-file", "trace.out", "tracing output file")

	// Add subcommands
	rootCmd.AddCommand(BuildHTTPCmd(env))
	rootCmd.AddCommand(BuildGrpcCmd(env))

	return &RootCommand{
		env:            env,
		cmd:            rootCmd,
		profilerConfig: profilerConfig,
	}
}

// Do executes the RootCommand
func (r *RootCommand) Do(ctx context.Context) error {
	if err := r.env.Process().Lock(); err != nil {
		return fmt.Errorf("could not acquire process lock %w", err)
	}
	defer func() {
		_ = r.env.Process().Unlock()
	}()

	profiler, err := r.env.Profiler().Start(r.profilerConfig)
	if err != nil {
		return fmt.Errorf("could not create profiler %w", err)
	}

	defer func() {
		_ = profiler.Close()
	}()

	// set context for command
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// pass context to subcommands
	r.cmd.SetContext(ctx)

	return r.cmd.Execute()
}
