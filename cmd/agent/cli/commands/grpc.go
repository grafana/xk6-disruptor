package commands

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/cmd/agent"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol/grpc"

	"github.com/spf13/cobra"
)

// BuildGrpcCmd returns a cobra command with the specification of the grpc command
func BuildGrpcCmd(agent *agent.Agent) *cobra.Command {
	disruption := grpc.Disruption{}
	var duration time.Duration
	var port uint
	var target uint
	var iface string
	upstreamHost := "localhost"
	transparent := true

	cmd := &cobra.Command{
		Use:   "grpc",
		Short: "grpc disruptor",
		Long: "Disrupts http request by introducing delays and errors." +
			" When running as a transparent proxy requires NET_ADMIM capabilities for setting" +
			" iptable rules.",
		RunE: func(cmd *cobra.Command, args []string) error {
			listenAddress := fmt.Sprintf(":%d", port)
			upstreamAddress := fmt.Sprintf("%s:%d", upstreamHost, target)
			
			proxyConfig := grpc.ProxyConfig{
				ListenAddress:   listenAddress,
				UpstreamAddress: upstreamAddress,
			}

			disruptorConfig := protocol.DisruptorConfig{
				TargetPort:   target,
				RedirectPort: port,
				Iface:        iface,
			}

			err := agent.GrpcDisruption(
				cmd.Context(),
				proxyConfig,
				disruption,
				disruptorConfig,
				transparent,
				duration,
			)
			return err
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
	cmd.Flags().BoolVar(&transparent, "transparent", true, "run as transparent proxy")

	return cmd
}
