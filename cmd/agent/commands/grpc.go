package commands

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol/grpc"
	"github.com/spf13/cobra"
)

// BuildGrpcCmd returns a cobra command with the specification of the grpc command
func BuildGrpcCmd() *cobra.Command {
	disruption := grpc.Disruption{}
	var duration time.Duration
	var port uint
	var target uint
	var iface string
	cmd := &cobra.Command{
		Use:   "grpc",
		Short: "grpc disruptor",
		Long: "Disrupts grpc request by introducing delays and errors." +
			" Requires NET_ADMIM capabilities for setting iptable rules.",
		RunE: func(cmd *cobra.Command, args []string) error {
			listenAddress := fmt.Sprintf(":%d", port)
			upstreamAddress := fmt.Sprintf(":%d", target)
			proxy, err := grpc.NewProxy(
				grpc.ProxyConfig{
					ListenAddress:   listenAddress,
					UpstreamAddress: upstreamAddress,
				}, disruption)
			if err != nil {
				return err
			}

			disruptor, err := protocol.NewDisruptor(
				protocol.DisruptorConfig{
					TargetPort:   target,
					RedirectPort: port,
					Iface:        iface,
				},
				proxy,
			)
			if err != nil {
				return err
			}

			return disruptor.Apply(duration)
		},
	}
	cmd.Flags().DurationVarP(&duration, "duration", "d", 0, "duration of the disruptions")
	cmd.Flags().DurationVarP(&disruption.AverageDelay, "average-delay", "a", 0, "average request delay")
	cmd.Flags().DurationVarP(&disruption.DelayVariation, "delay-variation", "v", 0, "variation in request delay")
	cmd.Flags().Int32VarP(&disruption.StatusCode, "status", "s", 0, "status code")
	cmd.Flags().Float32VarP(&disruption.ErrorRate, "rate", "r", 0, "error rate")
	cmd.Flags().StringVarP(&disruption.StatusMessage, "message", "m", "", "error message for injected faults")
	cmd.Flags().StringVarP(&iface, "interface", "i", "eth0", "interface to disrupt")
	cmd.Flags().UintVarP(&port, "port", "p", 8080, "port the proxy will listen to")
	cmd.Flags().UintVarP(&target, "target", "t", 80, "port the proxy will redirect request to")
	cmd.Flags().StringSliceVarP(&disruption.Excluded, "exclude", "x", []string{}, "comma-separated list of grpc services"+
		" to be excluded from disruption")

	return cmd
}
