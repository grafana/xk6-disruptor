package disruptors

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/utils"
)

//nolint:dupl
func buildGrpcFaultCmd(fault GrpcFault, duration time.Duration, options GrpcDisruptionOptions) []string {
	cmd := []string{
		"xk6-disruptor-agent",
		"grpc",
		"-d", utils.DurationSeconds(duration),
	}

	if fault.AverageDelay > 0 {
		cmd = append(cmd, "-a", fmt.Sprint(fault.AverageDelay), "-v", fmt.Sprint(fault.DelayVariation))
	}

	if fault.ErrorRate > 0 {
		cmd = append(
			cmd,
			"-s",
			fmt.Sprint(fault.StatusCode),
			"-r",
			fmt.Sprint(fault.ErrorRate),
		)
		if fault.StatusMessage != "" {
			cmd = append(cmd, "-m", fault.StatusMessage)
		}
	}

	if fault.Port != 0 {
		cmd = append(cmd, "-t", fmt.Sprint(fault.Port))
	}

	if len(fault.Exclude) > 0 {
		cmd = append(cmd, "-x", fault.Exclude)
	}

	if options.ProxyPort != 0 {
		cmd = append(cmd, "-p", fmt.Sprint(options.ProxyPort))
	}

	if options.Iface != "" {
		cmd = append(cmd, "-i", options.Iface)
	}

	return cmd
}

//nolint:dupl
func buildHTTPFaultCmd(fault HTTPFault, duration time.Duration, options HTTPDisruptionOptions) []string {
	cmd := []string{
		"xk6-disruptor-agent",
		"http",
		"-d", utils.DurationSeconds(duration),
	}

	if fault.AverageDelay > 0 {
		cmd = append(cmd, "-a", fmt.Sprint(fault.AverageDelay), "-v", fmt.Sprint(fault.DelayVariation))
	}

	if fault.ErrorRate > 0 {
		cmd = append(
			cmd,
			"-e",
			fmt.Sprint(fault.ErrorCode),
			"-r",
			fmt.Sprint(fault.ErrorRate),
		)
		if fault.ErrorBody != "" {
			cmd = append(cmd, "-b", fault.ErrorBody)
		}
	}

	if fault.Port != 0 {
		cmd = append(cmd, "-t", fmt.Sprint(fault.Port))
	}

	if len(fault.Exclude) > 0 {
		cmd = append(cmd, "-x", fault.Exclude)
	}

	if options.ProxyPort != 0 {
		cmd = append(cmd, "-p", fmt.Sprint(options.ProxyPort))
	}

	if options.Iface != "" {
		cmd = append(cmd, "-i", options.Iface)
	}

	return cmd
}
