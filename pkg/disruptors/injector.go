package disruptors

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

// PodHTTPFaultInjector implements the Visitor interface for injecting HttpFaults in a Pod
type PodHTTPFaultInjector struct {
	fault    HTTPFault
	duration time.Duration
	options  HTTPDisruptionOptions
}

// Visit return the VisitCommands for injecting a HttpFault in a Pod
func (i PodHTTPFaultInjector) Visit(pod corev1.Pod) (VisitCommands, error) {
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

// PodGrpcFaultInjector implements the Visitor interface for injecting GrpcFaults in a Pod
type PodGrpcFaultInjector struct {
	fault    GrpcFault
	duration time.Duration
	options  GrpcDisruptionOptions
}

// Visit return the VisitCommands for injecting a GrpcFault in a Pod
func (i PodGrpcFaultInjector) Visit(pod corev1.Pod) (VisitCommands, error) {
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

// ServiceHTTPFaultInjector implements the Visitor interface for injecting HttpFaults in a Pod
type ServiceHTTPFaultInjector struct {
	service  corev1.Service
	fault    HTTPFault
	duration time.Duration
	options  HTTPDisruptionOptions
}

// Visit return the VisitCommands for injecting a HttpFault in a Service
func (i ServiceHTTPFaultInjector) Visit(pod corev1.Pod) (VisitCommands, error) {
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

// ServiceGrpcFaultInjector implements the Visitor interface for injecting a GrpcFault in a Service
type ServiceGrpcFaultInjector struct {
	service  corev1.Service
	fault    GrpcFault
	duration time.Duration
	options  GrpcDisruptionOptions
}

// Visit return the VisitCommands for injecting a GrpcFault in a Pod
func (i ServiceGrpcFaultInjector) Visit(pod corev1.Pod) (VisitCommands, error) {
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
