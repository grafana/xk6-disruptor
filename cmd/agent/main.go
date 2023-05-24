// Package main implements the root level command for the disruptor agent CLI
package main

import (
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	runtimetrace "runtime/trace"

	"github.com/grafana/xk6-disruptor/cmd/agent/commands"
	"github.com/grafana/xk6-disruptor/pkg/utils/process"
	"github.com/spf13/cobra"
)

const lockFile = "/var/run/xk6-disruptor"

//nolint:funlen,gocognit
func main() {
	cpuProfile := false
	var cpuProfileFileName string
	memProfile := false
	var memProfileFileName string
	var memProfileFile *os.File
	trace := false
	var traceFileName string
	var traceFile *os.File

	rootCmd := &cobra.Command{
		Use:   "xk6-disruptor-agent",
		Short: "Inject disruptions in a system",
		Long: "A command for injecting disruptions in a target system.\n" +
			"It can run as stand-alone process or in a container",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			acquired, err := process.Lock(lockFile)
			if err != nil {
				return fmt.Errorf("error creating lock file: %w", err)
			}
			if !acquired {
				return fmt.Errorf("another disruptor command is already in execution")
			}

			// cpu profiling
			if cpuProfile {
				var profileFile *os.File
				profileFile, err = os.Create(cpuProfileFileName)
				if err != nil {
					return fmt.Errorf("error creating CPU profiling file %q: %w", cpuProfileFileName, err)
				}

				err = pprof.StartCPUProfile(profileFile)
				if err != nil {
					return fmt.Errorf("failed to start CPU profiling: %w", err)
				}
			}

			// memory profiling
			if memProfile {
				memProfileFile, err = os.Create(memProfileFileName)
				if err != nil {
					return fmt.Errorf("error creating memory profiling file %q: %w", memProfileFileName, err)
				}

				runtime.MemProfileRate = 1.
			}

			// trace program execution
			if trace {
				traceFile, err = os.Create(traceFileName)
				if err != nil {
					return fmt.Errorf("failed to create trace output file %q: %w", traceFileName, err)
				}

				if err := runtimetrace.Start(traceFile); err != nil {
					return fmt.Errorf("failed to start trace: %w", err)
				}
			}

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			_ = process.Unlock(lockFile)
			if cpuProfile {
				pprof.StopCPUProfile()
			}
			if memProfile {
				err := pprof.Lookup("heap").WriteTo(memProfileFile, 0)
				if err != nil {
					return fmt.Errorf("failed to write memory profile to file %q: %w", memProfileFileName, err)
				}
			}
			if trace {
				runtimetrace.Stop()
				_ = traceFile.Close()
			}

			return nil
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().BoolVar(&cpuProfile, "cpu-profile", false, "profile agent execution")
	rootCmd.PersistentFlags().StringVar(&cpuProfileFileName, "cpu-profile-file", "cpu.pprof",
		"cpu profiling output file")
	rootCmd.PersistentFlags().BoolVar(&memProfile, "mem-profile", false, "profile agent memory")
	rootCmd.PersistentFlags().StringVar(&memProfileFileName, "mem-profile-file", "mem.pprof",
		"memory profiling output file")
	rootCmd.PersistentFlags().BoolVar(&trace, "trace", false, "trace agent execution")
	rootCmd.PersistentFlags().StringVar(&traceFileName, "trace-file", "trace.out", "tracing output file")

	rootCmd.AddCommand(commands.BuildHTTPCmd())
	rootCmd.AddCommand(commands.BuildGrpcCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
