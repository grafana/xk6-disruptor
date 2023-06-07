package commands

import (
	"fmt"
	"io"

	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/spf13/cobra"
)

// BuildRootCmd builds the root command for the agent with all the persistent flags.
// It also initializes/terminates the profiling if requested.
func BuildRootCmd(env runtime.Environment) *cobra.Command {
	profilerConfig := runtime.ProfilerConfig{}
	var profiler io.Closer

	rootCmd := &cobra.Command{
		Use:   "xk6-disruptor-agent",
		Short: "Inject disruptions in a system",
		Long: "A command for injecting disruptions in a target system.\n" +
			"It can run as stand-alone process or in a container",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := env.Process().Lock()
			if err != nil {
				return err
			}

			profiler, err = env.Profiler().Start(profilerConfig)
			if err != nil {
				return fmt.Errorf("could not create profiler %w", err)
			}

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				_ = env.Process().Unlock()
			}()

			err := profiler.Close()
			if err != nil {
				return fmt.Errorf("could not stop profiler %w", err)
			}

			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().BoolVar(&profilerConfig.CPUProfile, "cpu-profile", false, "profile agent execution")
	rootCmd.PersistentFlags().StringVar(&profilerConfig.CPUProfileFileName, "cpu-profile-file", "cpu.pprof",
		"cpu profiling output file")
	rootCmd.PersistentFlags().BoolVar(&profilerConfig.MemProfile, "mem-profile", false, "profile agent memory")
	rootCmd.PersistentFlags().StringVar(&profilerConfig.MemProfileFileName, "mem-profile-file", "mem.pprof",
		"memory profiling output file")
	rootCmd.PersistentFlags().IntVar(&profilerConfig.MemProfileRate, "mem-profile-rate", 1, "memory profiling rate")
	rootCmd.PersistentFlags().BoolVar(&profilerConfig.Trace, "trace", false, "trace agent execution")
	rootCmd.PersistentFlags().StringVar(&profilerConfig.TraceFileName, "trace-file", "trace.out", "tracing output file")

	return rootCmd
}
