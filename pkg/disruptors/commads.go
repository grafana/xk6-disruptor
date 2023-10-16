package disruptors

import (
	"fmt"
	"time"

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

// PodHTTPFaultCommandGenerator implements the AgentCommandGenerator interface for injecting
// HttpFaults in a Pod
type PodHTTPFaultCommandGenerator struct {
	fault    HTTPFault
	duration time.Duration
	options  HTTPDisruptionOptions
}

// GetCommands return the VisitCommands for injecting a HttpFault in a Pod
func (i PodHTTPFaultCommandGenerator) GetCommands(pod corev1.Pod) (VisitCommands, error) {
	if !utils.HasPort(pod, i.fault.Port) {
		return VisitCommands{}, fmt.Errorf("pod %q does not expose port %d", pod.Name, i.fault.Port)
	}

	if utils.HasHostNetwork(pod) {
		return VisitCommands{}, fmt.Errorf("pod %q cannot be safely injected as it has hostNetwork set to true", pod.Name)
	}

	targetAddress, err := utils.PodIP(pod)
	if err != nil {
		return VisitCommands{}, err
	}

	visitCommands := VisitCommands{
		Exec:    buildHTTPFaultCmd(targetAddress, i.fault, i.duration, i.options),
		Cleanup: buildCleanupCmd(),
	}

	return visitCommands, nil
}

// PodGrpcFaultCommandGenerator implements the AgentCommandGenerator interface for injecting GrpcFaults in a Pod
type PodGrpcFaultCommandGenerator struct {
	fault    GrpcFault
	duration time.Duration
	options  GrpcDisruptionOptions
}

// GetCommands return the VisitCommands for injecting a GrpcFault in a Pod
func (i PodGrpcFaultCommandGenerator) GetCommands(pod corev1.Pod) (VisitCommands, error) {
	if !utils.HasPort(pod, i.fault.Port) {
		return VisitCommands{}, fmt.Errorf("pod %q does not expose port %d", pod.Name, i.fault.Port)
	}

	targetAddress, err := utils.PodIP(pod)
	if err != nil {
		return VisitCommands{}, err
	}

	visitCommands := VisitCommands{
		Exec:    buildGrpcFaultCmd(targetAddress, i.fault, i.duration, i.options),
		Cleanup: buildCleanupCmd(),
	}

	return visitCommands, nil
}

// ServiceHTTPFaultCommandGenerator implements the AgentCommandGenerator interface for injecting HttpFaults in a Pod
type ServiceHTTPFaultCommandGenerator struct {
	service  corev1.Service
	fault    HTTPFault
	duration time.Duration
	options  HTTPDisruptionOptions
}

// GetCommands return the VisitCommands for injecting a HttpFault in a Service
func (i ServiceHTTPFaultCommandGenerator) GetCommands(pod corev1.Pod) (VisitCommands, error) {
	port, err := utils.MapPort(i.service, i.fault.Port, pod)
	if err != nil {
		return VisitCommands{}, err
	}

	if utils.HasHostNetwork(pod) {
		return VisitCommands{}, fmt.Errorf("pod %q cannot be safely injected as it has hostNetwork set to true", pod.Name)
	}

	// copy fault to change target port for the pod
	podFault := i.fault
	podFault.Port = port

	targetAddress, err := utils.PodIP(pod)
	if err != nil {
		return VisitCommands{}, err
	}

	visitCommands := VisitCommands{
		Exec:    buildHTTPFaultCmd(targetAddress, podFault, i.duration, i.options),
		Cleanup: buildCleanupCmd(),
	}

	return visitCommands, nil
}

// ServiceGrpcFaultCommandGenerator implements the AgentCommandGenerator interface for injecting a
// GrpcFault in a Service
type ServiceGrpcFaultCommandGenerator struct {
	service  corev1.Service
	fault    GrpcFault
	duration time.Duration
	options  GrpcDisruptionOptions
}

// GetCommands return the VisitCommands for injecting a GrpcFault in a Pod
func (i ServiceGrpcFaultCommandGenerator) GetCommands(pod corev1.Pod) (VisitCommands, error) {
	port, err := utils.MapPort(i.service, i.fault.Port, pod)
	if err != nil {
		return VisitCommands{}, err
	}

	podFault := i.fault
	podFault.Port = port

	targetAddress, err := utils.PodIP(pod)
	if err != nil {
		return VisitCommands{}, err
	}

	visitCommands := VisitCommands{
		Exec:    buildGrpcFaultCmd(targetAddress, podFault, i.duration, i.options),
		Cleanup: buildCleanupCmd(),
	}

	return visitCommands, nil
}
