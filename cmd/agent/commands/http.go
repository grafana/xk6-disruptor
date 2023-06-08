package commands

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol/http"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/spf13/cobra"
)

// BuildHTTPCmd returns a cobra command with the specification of the http command
func BuildHTTPCmd(env runtime.Environment) *cobra.Command {
	disruption := http.Disruption{}
	var duration time.Duration
	var port uint
	var target uint
	var iface string
	upstreamHost := "localhost"
	transparent := true

	cmd := &cobra.Command{
		Use:   "http",
		Short: "http disruptor",
		Long: "Disrupts http request by introducing delays and errors." +
			" When running as a transparent proxy requires NET_ADMIM capabilities for setting" +
			" iptable rules.",
		RunE: func(cmd *cobra.Command, args []string) error {
			listenAddress := fmt.Sprintf(":%d", port)
			upstreamAddress := fmt.Sprintf("http://%s:%d", upstreamHost, target)
			proxy, err := http.NewProxy(
				http.ProxyConfig{
					ListenAddress:   listenAddress,
					UpstreamAddress: upstreamAddress,
				}, disruption)
			if err != nil {
				return err
			}

			// run as a regular proxy
			if !transparent {
				// TODO: pass a context with a timeout using the duration argument
				return proxy.Start()
			}

			disruptor, err := protocol.NewDisruptor(
				env.Executor(),
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
