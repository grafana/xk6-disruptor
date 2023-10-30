package commands

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent"
	"github.com/grafana/xk6-disruptor/pkg/agent/tcpconn"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/spf13/cobra"
)

// BuildTCPDropCmd returns a cobra command with the specification of the tcp-drop command.
func BuildTCPDropCmd(env runtime.Environment, config *agent.Config) *cobra.Command {
	disruption := tcpconn.Disruption{}
	var duration time.Duration

	cmd := &cobra.Command{
		Use:   "tcp-drop",
		Short: "tcp coneciton drop",
		Long: "Disrupts TCP connections by terminating a certain percentage of them." +
			" When running as a transparent proxy requires NET_ADMIM capabilities for setting" +
			" iptable rules.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if disruption.Port == 0 {
				return fmt.Errorf("target port for fault injection is required")
			}

			agent, err := agent.Start(env, config)
			if err != nil {
				return fmt.Errorf("initializing agent: %w", err)
			}

			defer agent.Stop()

			nfqConfig := tcpconn.RandomNFQConfig()

			queue := tcpconn.NFQueue{
				NFQConfig:  nfqConfig,
				Disruption: disruption,
				Executor:   env.Executor(),
			}

			disruptor := tcpconn.Disruptor{
				NFQConfig:  nfqConfig,
				Disruption: disruption,
				Queue:      queue,
			}

			return agent.ApplyDisruption(cmd.Context(), disruptor, duration)
		},
	}

	cmd.Flags().DurationVarP(&duration, "duration", "d", 0, "duration of the disruptions")
	cmd.Flags().UintVarP(&disruption.Port, "port", "p", 8000, "target port of the connections to be disrupted")
	cmd.Flags().Float64VarP(&disruption.DropRate, "rate", "r", 0, "error rate")

	return cmd
}
