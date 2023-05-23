// Package main implements the root level command for the disruptor agent CLI
package main

import (
	"fmt"
	"os"
	"runtime/pprof"
	runtimetrace "runtime/trace"

	"github.com/grafana/xk6-disruptor/cmd/agent/commands"
	"github.com/grafana/xk6-disruptor/pkg/utils/process"
	"github.com/spf13/cobra"
)

const lockFile = "/var/run/xk6-disruptor"

func main() {
	profile := false
	var profileFileName string
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

			// start profiling
			if profile {
				var profileFile *os.File
				profileFile, err = os.Create(profileFileName)
				if err != nil {
					return fmt.Errorf("error creating profiling file %s: %w", profileFileName, err)
				}

				err = pprof.StartCPUProfile(profileFile)
				if err != nil {
					return fmt.Errorf("failed to start CPU profiling: %w", err)
				}
			}

			// trace program execution
			if trace {
				traceFile, err = os.Create(traceFileName)
				if err != nil {
					return fmt.Errorf("failed to create trace output file %s: %w", traceFileName, err)
				}

				if err := runtimetrace.Start(traceFile); err != nil {
					return fmt.Errorf("failed to start trace: %w", err)
				}
			}

			return nil
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			_ = process.Unlock(lockFile)
			if profile {
				pprof.StopCPUProfile()
			}
			if trace {
				runtimetrace.Stop()
				_ = traceFile.Close()
			}
		},
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	rootCmd.PersistentFlags().BoolVar(&profile, "profile", false, "profile agent execution")
	rootCmd.PersistentFlags().StringVar(&profileFileName, "profile-file", "profile.pb.gz", "profiling output file")
	rootCmd.PersistentFlags().BoolVar(&trace, "trace", false, "trace agent execution")
	rootCmd.PersistentFlags().StringVar(&traceFileName, "trace-file", "trace.out", "tracing output file")

	rootCmd.AddCommand(commands.BuildHTTPCmd())
	rootCmd.AddCommand(commands.BuildGrpcCmd())

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
}
