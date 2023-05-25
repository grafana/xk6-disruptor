package commands

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol/http"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func buildHTTPDisruptionFlags(disruption *http.Disruption) *pflag.FlagSet {
	flags := pflag.NewFlagSet("HTTP Disruption", pflag.ContinueOnError)

	flags.DurationVarP(&disruption.AverageDelay, "average-delay", "a", 0, "average request delay")
	flags.DurationVarP(&disruption.DelayVariation, "delay-variation", "v", 0, "variation in request delay")
	flags.UintVarP(&disruption.ErrorCode, "error", "e", 0, "error code")
	flags.Float32VarP(&disruption.ErrorRate, "rate", "r", 0, "error rate")
	flags.StringVarP(&disruption.ErrorBody, "body", "b", "", "body for injected faults")
	flags.StringSliceVarP(&disruption.Excluded, "exclude", "x", []string{}, "comma-separated list of path(s)"+
		" to be excluded from disruption")

	return flags
}

// BuildHTTPCmd returns a cobra command with the specification of the http command
func BuildHTTPCmd() *cobra.Command {
	disruption := http.Disruption{}
	config := protocol.DisruptorConfig{}
	upstreamHost := "127.0.0.1"
	var duration time.Duration
	cmd := &cobra.Command{
		Use:   "http",
		Short: "http disruptor",
		Long: "Installs a transparent proxy that disrupts http request by introducing delays and errors." +
			" Requires NET_ADMIM capabilities for setting iptable rules.",
		RunE: func(cmd *cobra.Command, args []string) error {
			upstreamAddress := fmt.Sprintf("http://%s:%d", upstreamHost, config.TargetPort)
			listenAddress := fmt.Sprintf(":%d", config.RedirectPort)
			proxy, err := http.NewProxy(
				http.ProxyConfig{
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
	cmd.Flags().AddFlagSet(buildHTTPDisruptionFlags(&disruption))

	// add flags for traffic redirection
	cmd.Flags().StringVarP(&config.Iface, "interface", "i", "eth0", "interface to disrupt")
	cmd.Flags().UintVarP(&config.TargetPort, "port", "p", 8080, "port the proxy will listen to")
	cmd.Flags().UintVarP(&config.TargetPort, "target", "t", 80, "port the proxy will redirect request to")

	return cmd
}

// BuildHTTPProxyCmd returns a cobra command with the specification of the http-proxy command
func BuildHTTPProxyCmd() *cobra.Command {
	disruption := http.Disruption{}
	var port uint
	upstreamHost := ""
	cmd := &cobra.Command{
		Use:   "http-proxy",
		Short: "http disruptor proxy ",
		Long:  "Proxy that disrupts http request by introducing delays and errors.",
		RunE: func(cmd *cobra.Command, args []string) error {
			listenAddress := fmt.Sprintf(":%d", port)
			proxy, err := http.NewProxy(
				http.ProxyConfig{
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

	cmd.Flags().AddFlagSet(buildHTTPDisruptionFlags(&disruption))
	cmd.Flags().UintVarP(&port, "port", "p", 8080, "port the proxy will listen to")
	cmd.Flags().StringVarP(&upstreamHost, "upstream", "u", "127.0.0.1:80", "upstream host to redirect to")

	return cmd
}
