package disruptors

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/utils"
	corev1 "k8s.io/api/core/v1"
)

// PodHTTPFaultVisitor implements the Visitor interface for injecting HttpFaults in a Pod
type PodHTTPFaultVisitor struct {
	fault    HTTPFault
	duration time.Duration
	options  HTTPDisruptionOptions
}

// Visit return the VisitCommands for injecting a HttpFault in a Pod
func (i PodHTTPFaultVisitor) Visit(pod corev1.Pod) (VisitCommands, error) {
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

// PodGrpcFaultVisitor implements the Visitor interface for injecting GrpcFaults in a Pod
type PodGrpcFaultVisitor struct {
	fault    GrpcFault
	duration time.Duration
	options  GrpcDisruptionOptions
}

// Visit return the VisitCommands for injecting a GrpcFault in a Pod
func (i PodGrpcFaultVisitor) Visit(pod corev1.Pod) (VisitCommands, error) {
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

// ServiceHTTPFaultVisitor implements the Visitor interface for injecting HttpFaults in a Pod
type ServiceHTTPFaultVisitor struct {
	service  corev1.Service
	fault    HTTPFault
	duration time.Duration
	options  HTTPDisruptionOptions
}

// Visit return the VisitCommands for injecting a HttpFault in a Service
func (i ServiceHTTPFaultVisitor) Visit(pod corev1.Pod) (VisitCommands, error) {
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

// ServiceGrpcFaultVisitor implements the Visitor interface for injecting a GrpcFault in a Service
type ServiceGrpcFaultVisitor struct {
	service  corev1.Service
	fault    GrpcFault
	duration time.Duration
	options  GrpcDisruptionOptions
}

// Visit return the VisitCommands for injecting a GrpcFault in a Pod
func (i ServiceGrpcFaultVisitor) Visit(pod corev1.Pod) (VisitCommands, error) {
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
