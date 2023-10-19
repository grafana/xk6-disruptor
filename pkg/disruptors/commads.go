package disruptors

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/types/intstr"
	"github.com/grafana/xk6-disruptor/pkg/utils"

	corev1 "k8s.io/api/core/v1"
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
	if fault.Port != intstr.NullValue {
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
	if fault.Port != intstr.NullValue {
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

// PodHTTPFaultCommand implements the PodVisitCommands interface for injecting
// HttpFaults in a Pod
type PodHTTPFaultCommand struct {
	fault    HTTPFault
	duration time.Duration
	options  HTTPDisruptionOptions
}

// Commands return the command for injecting a HttpFault in a Pod
func (c PodHTTPFaultCommand) Commands(pod corev1.Pod) (VisitCommands, error) {
	if !utils.HasPort(pod, c.fault.Port) {
		return VisitCommands{}, fmt.Errorf("pod %q does not expose port %q", pod.Name, c.fault.Port.Str())
	}

	if utils.HasHostNetwork(pod) {
		return VisitCommands{}, fmt.Errorf("pod %q cannot be safely injected as it has hostNetwork set to true", pod.Name)
	}

	targetAddress, err := utils.PodIP(pod)
	if err != nil {
		return VisitCommands{}, err
	}

	return VisitCommands{
		Exec:    buildHTTPFaultCmd(targetAddress, c.fault, c.duration, c.options),
		Cleanup: buildCleanupCmd(),
	}, nil
}

// PodGrpcFaultCommand implements the PodVisitCommands interface for injecting GrpcFaults in a Pod
type PodGrpcFaultCommand struct {
	fault    GrpcFault
	duration time.Duration
	options  GrpcDisruptionOptions
}

// Commands return the command for injecting a GrpcFault in a Pod
func (c PodGrpcFaultCommand) Commands(pod corev1.Pod) (VisitCommands, error) {
	if !utils.HasPort(pod, c.fault.Port) {
		return VisitCommands{}, fmt.Errorf("pod %q does not expose port %q", pod.Name, c.fault.Port.Str())
	}

	targetAddress, err := utils.PodIP(pod)
	if err != nil {
		return VisitCommands{}, err
	}

	return VisitCommands{
		Exec:    buildGrpcFaultCmd(targetAddress, c.fault, c.duration, c.options),
		Cleanup: buildCleanupCmd(),
	}, nil
}

// ServiceHTTPFaultCommand implements the PodVisitCommands interface for injecting HttpFaults in a Pod
type ServiceHTTPFaultCommand struct {
	service  corev1.Service
	fault    HTTPFault
	duration time.Duration
	options  HTTPDisruptionOptions
}

// Commands return the command for injecting a HttpFault in a Service
func (c ServiceHTTPFaultCommand) Commands(pod corev1.Pod) (VisitCommands, error) {
	port, err := utils.MapPort(c.service, c.fault.Port, pod)
	if err != nil {
		return VisitCommands{}, err
	}

	if utils.HasHostNetwork(pod) {
		return VisitCommands{}, fmt.Errorf("pod %q cannot be safely injected as it has hostNetwork set to true", pod.Name)
	}

	// copy fault to change target port for the pod
	podFault := c.fault
	podFault.Port = port

	targetAddress, err := utils.PodIP(pod)
	if err != nil {
		return VisitCommands{}, err
	}

	return VisitCommands{
		Exec:    buildHTTPFaultCmd(targetAddress, podFault, c.duration, c.options),
		Cleanup: buildCleanupCmd(),
	}, nil
}

// Cleanup defines the command to execute for cleaning up if command execution fails
func (c ServiceHTTPFaultCommand) Cleanup(_ corev1.Pod) []string {
	return buildCleanupCmd()
}

// ServiceGrpcFaultCommand implements the PodVisitCommands interface for injecting a
// GrpcFault in a Service
type ServiceGrpcFaultCommand struct {
	service  corev1.Service
	fault    GrpcFault
	duration time.Duration
	options  GrpcDisruptionOptions
}

// Commands return the VisitCommands for injecting a GrpcFault in a Service
func (c ServiceGrpcFaultCommand) Commands(pod corev1.Pod) (VisitCommands, error) {
	port, err := utils.MapPort(c.service, c.fault.Port, pod)
	if err != nil {
		return VisitCommands{}, err
	}

	podFault := c.fault
	podFault.Port = port

	targetAddress, err := utils.PodIP(pod)
	if err != nil {
		return VisitCommands{}, err
	}

	return VisitCommands{
		Exec:    buildGrpcFaultCmd(targetAddress, podFault, c.duration, c.options),
		Cleanup: buildCleanupCmd(),
	}, nil
}
