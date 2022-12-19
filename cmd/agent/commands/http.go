package commands

import (
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent/disruptors/http"
	"github.com/spf13/cobra"
)

// BuildHTTPCmd returns a cobra command with the specification of the http command
func BuildHTTPCmd() *cobra.Command {
	target := http.DisruptionTarget{}
	disruption := http.Disruption{}
	config := http.DisruptorConfig{}
	var duration time.Duration
	cmd := &cobra.Command{
		Use:   "http",
		Short: "http disruptor",
		Long: "Disrupts http request by introducing delays and errors." +
			" Requires NET_ADMIM capabilities for setting iptable rules.",
		RunE: func(cmd *cobra.Command, args []string) error {
			disruptor, err := http.NewDisruptor(
				target,
				disruption,
				config,
			)
			if err != nil {
				return err
			}
			return disruptor.Apply(duration)
		},
	}
	cmd.Flags().DurationVarP(&duration, "duration", "d", 0*time.Second, "duration of the disruptions")
	cmd.Flags().UintVarP(&disruption.AverageDelay, "average-delay", "a", 0, "average request delay (milliseconds)")
	cmd.Flags().UintVarP(&disruption.DelayVariation, "delay-variation", "v", 0, "variation in request delay (milliseconds")
	cmd.Flags().UintVarP(&disruption.ErrorCode, "error", "e", 0, "error code")
	cmd.Flags().Float32VarP(&disruption.ErrorRate, "rate", "r", 0, "error rate")
	cmd.Flags().StringVarP(&disruption.ErrorBody, "body", "b", "", "body for injected faults")
	cmd.Flags().StringVarP(&target.Iface, "interface", "i", "eth0", "interface to disrupt")
	cmd.Flags().UintVarP(&config.ProxyConfig.ListeningPort, "port", "p", 8080, "port the proxy will listen to")
	cmd.Flags().UintVarP(&target.TargetPort, "target", "t", 80, "port the proxy will redirect request to")
	cmd.Flags().StringSliceVarP(&disruption.Excluded, "exclude", "x", []string{}, "comma-separated list of path(s)"+
		" to be excluded from disruption")

	return cmd
}
