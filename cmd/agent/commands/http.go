// BuildHttpCmd returns a cobra command with the specification of the http command
package commands

import (
	"time"

	"github.com/grafana/xk6-disruptor/pkg/disruptors/http"
	"github.com/spf13/cobra"
)

func BuildHttpCmd() *cobra.Command {
	d := &http.HttpDisruption{}
	c := &cobra.Command{
		Use:   "http",
		Short: "http disruptor",
		Long: "Disrupts http request by introducing delays." +
			" Requires NET_ADMIM and NET_RAW capabilities for setting iptable rules.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return d.Run()
		},
	}
	c.Flags().DurationVarP(&d.Duration, "duration", "d", 60*time.Second, "duration of the disruptions")
	c.Flags().UintVarP(&d.AverageDelay, "average-delay", "a", 100, "average request delay (milliseconds)")
	c.Flags().UintVarP(&d.DelayVariation, "delay-variation", "v", 0, "variation in request delay (milliseconds")
	c.Flags().UintVarP(&d.ErrorCode, "error", "e", 0, "error code")
	c.Flags().Float32VarP(&d.ErrorRate, "rate", "r", 0, "error rate")
	c.Flags().StringVarP(&d.Iface, "interface", "i", "eth0", "interface to disrupt")
	c.Flags().UintVarP(&d.Port, "port", "p", 8080, "port the proxy will listen to")
	c.Flags().UintVarP(&d.Target, "target", "t", 80, "port the proxy will redirect request to")
	c.Flags().StringArrayVarP(&d.Excluded, "exclude", "x", []string{}, "path(s) to be excluded from disruption")

	return c
}
