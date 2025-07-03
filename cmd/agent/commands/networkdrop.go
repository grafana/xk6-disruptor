package commands

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent"
	"github.com/grafana/xk6-disruptor/pkg/agent/network"
	"github.com/grafana/xk6-disruptor/pkg/iptables"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/spf13/cobra"
)

// BuildNetworkDropCmd builds the command for dropping network traffic on a given port.
func BuildNetworkDropCmd(env runtime.Environment, config *agent.Config) *cobra.Command {
	var duration time.Duration
	filter := network.Filter{}

	cmd := &cobra.Command{
		Use:   "network-drop",
		Short: "network connection drop",
		Long: "Drops network traffic on a given port. Requires either to be run as root, or the NET_ADMIN capability." +
			" Unlike tcp-drop, this command drops network traffic without sending any response to the client.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if filter.Port == 0 {
				return fmt.Errorf("target port for fault injection is required")
			}

			agent, err := agent.Start(env, config)
			if err != nil {
				return fmt.Errorf("initializing agent: %w", err)
			}

			defer agent.Stop()

			disruptor := network.Disruptor{
				Iptables: iptables.New(env.Executor()),
				Filter:   filter,
			}

			return agent.ApplyDisruption(cmd.Context(), disruptor, duration)
		},
	}
	cmd.Flags().DurationVarP(&duration, "duration", "d", 0, "duration of the disruptions")
	cmd.Flags().UintVarP(&filter.Port, "port", "p", 0, "target port of the connections to be disrupted")
	cmd.Flags().StringVarP(&filter.Protocol, "protocol", "P", "tcp", "target protocol of the connections to be disrupted")

	return cmd
}
