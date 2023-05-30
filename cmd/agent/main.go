// Package main implements the root level command for the disruptor agent CLI
package main

import (
	"fmt"
	"os"
	"path"

	"github.com/grafana/xk6-disruptor/cmd/agent/commands"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/spf13/cobra"
)

const lockFile = "xk6-disruptor"

// returns the path to the lock file
func getLockPath() string {
	// get runtime directory for user
	lockDir := os.Getenv("XDG_RUNTIME_DIR")
	if lockDir == "" {
		lockDir = os.TempDir()
	}

	return path.Join(lockDir, lockFile)
}

// ensure only one instance of the agent runs
func lockExecution() error {
	acquired, err := runtime.Lock(getLockPath())
	if err != nil {
		return fmt.Errorf("failed to create lock file %q: %w", getLockPath(), err)
	}
	if !acquired {
		return fmt.Errorf("another disruptor command is already in execution")
	}

	return nil
}

func main() {
	profilerConfig := runtime.ProfilerConfig{}
	var profiler runtime.Profiler

	rootCmd := &cobra.Command{
		Use:   "xk6-disruptor-agent",
		Short: "Inject disruptions in a system",
		Long: "A command for injecting disruptions in a target system.\n" +
			"It can run as stand-alone process or in a container",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			err := lockExecution()
			if err != nil {
				return err
			}

			profiler, err = runtime.NewProfiler(profilerConfig)
			if err != nil {
				return fmt.Errorf("could not create profiler %w", err)
			}

			err = profiler.Start()
			if err != nil {
				return fmt.Errorf("could not start profiler %w", err)
			}

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			_ = runtime.Unlock(getLockPath())

			err := profiler.Stop()
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

	rootCmd.AddCommand(commands.BuildHTTPCmd())
	rootCmd.AddCommand(commands.BuildGrpcCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
