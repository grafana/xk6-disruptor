package main

import (
	"github.com/grafana/xk6-disruptor/cmd/agent/commands"
	"github.com/spf13/cobra"
)

func main() {
	var rootCmd = &cobra.Command{
		Use:   "xk6-disruptor-agent",
		Short: "Inject disruptions in a system",
		Long: "A command for injecting disruptions in a target system.\n" +
			"It can run as stand-alone process or in a container",
	}

	rootCmd.AddCommand(commands.BuildHttpCmd())
	rootCmd.Execute()
}
