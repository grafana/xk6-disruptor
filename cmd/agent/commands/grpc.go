package commands

import (
	"fmt"
	"net"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol/grpc"
	"github.com/grafana/xk6-disruptor/pkg/iptables"
	"github.com/grafana/xk6-disruptor/pkg/runtime"

	"github.com/spf13/cobra"
)

// BuildGrpcCmd returns a cobra command with the specification of the grpc command
//
//nolint:funlen
func BuildGrpcCmd(env runtime.Environment, config *agent.Config) *cobra.Command {
	proxyConfig := grpc.ProxyConfig{}
	disruption := grpc.Disruption{}
	var duration time.Duration
	transparent := true
	var transparentInterface string
	var transparentAddress string

	//nolint: dupl
	cmd := &cobra.Command{
		Use:   "grpc",
		Short: "grpc disruptor",
		Long: "Disrupts http request by introducing delays and errors." +
			" When running as a transparent proxy requires NET_ADMIM capabilities for setting" +
			" iptable rules.",
		RunE: func(cmd *cobra.Command, args []string) error {
			// Redirect traffic to the proxy
			var redirector protocol.TrafficRedirector
			if transparent {
				_, lPort, err := net.SplitHostPort(proxyConfig.ListenAddress)
				if err != nil {
					return fmt.Errorf("parsing listen address %q: %w", proxyConfig.ListenAddress, err)
				}

				_, uPort, err := net.SplitHostPort(proxyConfig.UpstreamAddress)
				if err != nil {
					return fmt.Errorf("parsing upstream address %q: %w", proxyConfig.UpstreamAddress, err)
				}

				tr := &iptables.TrafficRedirectionSpec{
					Interface:    transparentInterface,
					LocalAddress: transparentAddress,
					ProxyPort:    lPort,
					TargetPort:   uPort,
				}

				redirector, err = iptables.NewTrafficRedirector(tr, env.Executor())
				if err != nil {
					return err
				}
			} else {
				redirector = protocol.NoopTrafficRedirector()
			}

			proxy, err := grpc.NewProxy(proxyConfig, disruption)
			if err != nil {
				return err
			}

			disruptor, err := protocol.NewDisruptor(
				env.Executor(),
				proxy,
				redirector,
			)
			if err != nil {
				return err
			}

			agent := agent.BuildAgent(env, config)

			return agent.ApplyDisruption(cmd.Context(), disruptor, duration)
		},
	}

	cmd.Flags().DurationVarP(&duration, "duration", "d", 0, "duration of the disruptions")
	cmd.Flags().DurationVarP(&disruption.AverageDelay, "average-delay", "a", 0, "average request delay")
	cmd.Flags().DurationVarP(&disruption.DelayVariation, "delay-variation", "v", 0, "variation in request delay")
	cmd.Flags().Int32VarP(&disruption.StatusCode, "status", "s", 0, "status code")
	cmd.Flags().Float32VarP(&disruption.ErrorRate, "rate", "r", 0, "error rate")
	cmd.Flags().StringVarP(&disruption.StatusMessage, "message", "m", "", "error message for injected faults")
	cmd.Flags().StringSliceVarP(&disruption.Excluded, "exclude", "x", []string{}, "comma-separated list of grpc services"+
		" to be excluded from disruption")
	cmd.Flags().BoolVar(&transparent, "transparent", true,
		"Run as transparent proxy. This mode requires root or NET_ADMIN privileges")
	// 120, 107, and 54 are ASCII for xk6, respectively.
	cmd.Flags().StringVar(&transparentAddress, "transparent-address", "127.120.107.54",
		"When running in transparent mode, this address will be added to the interface specified in "+
			"--transparent-interface. "+
			"Proxy will use this address to send requests upstream. "+
			"This flag is ignored if the agent does not run in transparent mode.")
	cmd.Flags().StringVar(&transparentInterface, "transparent-interface", "lo",
		"When running in transparent mode, the agent will add the address specified in --transparent-address to "+
			"this interface. "+
			"This flag does not affect in which interface the proxy listens in, which is only determined by --listen. "+
			"This flag is ignored if the agent runs in non-transparent mode.")
	cmd.Flags().StringVarP(&proxyConfig.ListenAddress, "listen", "l", ":8080",
		"Address where the proxy will listen at")
	cmd.Flags().StringVarP(&proxyConfig.UpstreamAddress, "target", "t", "localhost:80",
		"Address where the proxy will redirect requests to.")

	return cmd
}
