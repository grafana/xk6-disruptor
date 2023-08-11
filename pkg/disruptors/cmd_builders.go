package disruptors

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/utils"
)

func buildGrpcFaultCmd(
	targetAddress string,
	fault GrpcFault,
	duration time.Duration,
	options GrpcDisruptionOptions,
) []string {
	cmd := []string{
		"xk6-disruptor-agent",
		"grpc",
		"-d", utils.DurationSeconds(duration),
		"-t", fmt.Sprint(fault.Port),
	}

	// TODO: make port mandatory
	if fault.Port != 0 {
		cmd = append(cmd, "-t", fmt.Sprint(fault.Port))
	}

	if fault.AverageDelay > 0 {
		cmd = append(
			cmd,
			"-a",
			utils.DurationMillSeconds(fault.AverageDelay),
			"-v",
			utils.DurationMillSeconds(fault.DelayVariation),
		)
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

	if len(fault.Exclude) > 0 {
		cmd = append(cmd, "-x", fault.Exclude)
	}

	if options.ProxyPort != 0 {
		cmd = append(cmd, "-p", fmt.Sprint(options.ProxyPort))
	}

	cmd = append(cmd, "--upstream-host", targetAddress)

	return cmd
}

func buildHTTPFaultCmd(
	targetAddress string,
	fault HTTPFault,
	duration time.Duration,
	options HTTPDisruptionOptions,
) []string {
	cmd := []string{
		"xk6-disruptor-agent",
		"http",
		"-d", utils.DurationSeconds(duration),
	}

	// TODO: make port mandatory
	if fault.Port != 0 {
		cmd = append(cmd, "-t", fmt.Sprint(fault.Port))
	}

	if fault.AverageDelay > 0 {
		cmd = append(
			cmd,
			"-a",
			utils.DurationMillSeconds(fault.AverageDelay),
			"-v",
			utils.DurationMillSeconds(fault.DelayVariation),
		)
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

	if len(fault.Exclude) > 0 {
		cmd = append(cmd, "-x", fault.Exclude)
	}

	if options.ProxyPort != 0 {
		cmd = append(cmd, "-p", fmt.Sprint(options.ProxyPort))
	}

	cmd = append(cmd, "--upstream-host", targetAddress)

	return cmd
}

func buildCleanupCmd() []string {
	return []string{"xk6-disruptor-agent", "cleanup"}
}
