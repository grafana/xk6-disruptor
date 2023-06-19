package commands

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol/http"
	"github.com/grafana/xk6-disruptor/pkg/iptables"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/spf13/cobra"
)

// BuildHTTPCmd returns a cobra command with the specification of the http command
func BuildHTTPCmd(env runtime.Environment, config *agent.Config) *cobra.Command {
	disruption := http.Disruption{}
	var duration time.Duration
	var port uint
	var target uint
	var iface string
	upstreamHost := "localhost"
	transparent := true

	//nolint: dupl
	cmd := &cobra.Command{
		Use:   "http",
		Short: "http disruptor",
		Long: "Disrupts http request by introducing delays and errors." +
			" When running as a transparent proxy requires NET_ADMIM capabilities for setting" +
			" iptable rules.",
		RunE: func(cmd *cobra.Command, args []string) error {
			listenAddress := fmt.Sprintf(":%d", port)
			upstreamAddress := fmt.Sprintf("http://%s:%d", upstreamHost, target)

			proxyConfig := http.ProxyConfig{
				ListenAddress:   listenAddress,
				UpstreamAddress: upstreamAddress,
			}

			proxy, err := http.NewProxy(proxyConfig, disruption)
			if err != nil {
				return err
			}

			// Redirect traffic to the proxy
			var redirector protocol.TrafficRedirector
			if transparent {
				tr := &iptables.TrafficRedirectionSpec{
					Iface:           iface,
					DestinationPort: target,
					RedirectPort:    port,
				}

				redirector, err = iptables.NewTrafficRedirector(tr, env.Executor())
				if err != nil {
					return err
				}
			} else {
				redirector = protocol.NoopTrafficRedirector()
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
	cmd.Flags().UintVarP(&disruption.ErrorCode, "error", "e", 0, "error code")
	cmd.Flags().Float32VarP(&disruption.ErrorRate, "rate", "r", 0, "error rate")
	cmd.Flags().StringVarP(&disruption.ErrorBody, "body", "b", "", "body for injected faults")
	cmd.Flags().StringSliceVarP(&disruption.Excluded, "exclude", "x", []string{}, "comma-separated list of path(s)"+
		" to be excluded from disruption")
	cmd.Flags().BoolVar(&transparent, "transparent", true, "run as transparent proxy")
	cmd.Flags().StringVarP(&iface, "interface", "i", "eth0", "interface to disrupt")
	cmd.Flags().UintVarP(&port, "port", "p", 8080, "port the proxy will listen to")
	cmd.Flags().UintVarP(&target, "target", "t", 80, "port the proxy will redirect request to")

	return cmd
}
