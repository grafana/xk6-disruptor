package commands

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol/grpc"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func buildGrpcDisruptionFlags(disruption *grpc.Disruption) *pflag.FlagSet {
	flags := pflag.NewFlagSet("gRPC Disruption", pflag.ContinueOnError)

	flags.DurationVarP(&disruption.AverageDelay, "average-delay", "a", 0, "average request delay")
	flags.DurationVarP(&disruption.DelayVariation, "delay-variation", "v", 0, "variation in request delay")
	flags.Int32VarP(&disruption.StatusCode, "status", "s", 0, "status code")
	flags.Float32VarP(&disruption.ErrorRate, "rate", "r", 0, "error rate")
	flags.StringVarP(&disruption.StatusMessage, "message", "m", "", "error message for injected faults")
	flags.StringSliceVarP(&disruption.Excluded, "exclude", "x", []string{}, "comma-separated list of grpc services"+
		" to be excluded from disruption")

	return flags
}

// BuildGrpcCmd returns a cobra command with the specification of the grpc command
func BuildGrpcCmd() *cobra.Command {
	disruption := grpc.Disruption{}
	var duration time.Duration
	config := protocol.DisruptorConfig{}
	upstreamHost := "127.0.0.1"
	cmd := &cobra.Command{
		Use:   "grpc",
		Short: "grpc disruptor",
		Long: "Disrupts grpc request by introducing delays and errors." +
			" Requires NET_ADMIM capabilities for setting iptable rules.",
		RunE: func(cmd *cobra.Command, args []string) error {
			upstreamAddress := fmt.Sprintf("%s:%d", upstreamHost, config.TargetPort)
			listenAddress := fmt.Sprintf(":%d", config.RedirectPort)
			proxy, err := grpc.NewProxy(
				grpc.ProxyConfig{
					ListenAddress:   listenAddress,
					UpstreamAddress: upstreamAddress,
				}, disruption)
			if err != nil {
				return err
			}

			disruptor, err := protocol.NewDisruptor(
				config,
				proxy,
			)
			if err != nil {
				return err
			}

			return disruptor.Apply(duration)
		},
	}
	cmd.Flags().DurationVarP(&duration, "duration", "d", 0, "duration of the disruptions")

	// add flags for HTTP disruption
	cmd.Flags().AddFlagSet(buildGrpcDisruptionFlags(&disruption))

	// add flags for traffic redirection
	cmd.Flags().StringVarP(&config.Iface, "interface", "i", "eth0", "interface to disrupt")
	cmd.Flags().UintVarP(&config.RedirectPort, "port", "p", 8080, "port the proxy will listen to")
	cmd.Flags().UintVarP(&config.TargetPort, "target", "t", 80, "port the proxy will redirect request to")

	return cmd
}

// BuildGrpcProxyCmd returns a cobra command with the specification of the grpc-proxy command
func BuildGrpcProxyCmd() *cobra.Command {
	disruption := grpc.Disruption{}
	var port uint
	upstreamHost := ""
	cmd := &cobra.Command{
		Use:   "grpc-proxy",
		Short: "grpc disruptor proxy",
		Long:  "Proxy that disrupts gRPC request by introducing delays and errors.",
		RunE: func(cmd *cobra.Command, args []string) error {
			listenAddress := fmt.Sprintf(":%d", port)
			proxy, err := grpc.NewProxy(
				grpc.ProxyConfig{
					ListenAddress:   listenAddress,
					UpstreamAddress: upstreamHost,
				}, disruption)
			if err != nil {
				return err
			}

			err = proxy.Start()
			return err
		},
	}

	cmd.Flags().AddFlagSet(buildGrpcDisruptionFlags(&disruption))
	cmd.Flags().UintVarP(&port, "port", "p", 8080, "port the proxy will listen to")
	cmd.Flags().StringVarP(&upstreamHost, "upstream", "u", "", "upstream host to redirect to")

	return cmd
}
