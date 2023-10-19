package utils

import (
	"fmt"

	"github.com/grafana/xk6-disruptor/pkg/types/intstr"
	corev1 "k8s.io/api/core/v1"
)

// GetTargetPort returns the target port for the fiven service port
func GetTargetPort(service corev1.Service, svcPort intstr.IntOrString) (intstr.IntOrString, error) {
	// Handle default port mapping
	// TODO: make port required
	if svcPort == intstr.NullValue || (svcPort.IsInt() && svcPort.Int32() == 0) {
		if len(service.Spec.Ports) > 1 {
			return intstr.NullValue, fmt.Errorf("no port selected and service exposes more than one service")
		}
		return intstr.IntOrString(service.Spec.Ports[0].TargetPort.String()), nil
	}

	for _, p := range service.Spec.Ports {
		if p.Name == svcPort.Str() || (svcPort.IsInt() && p.Port == svcPort.Int32()) {
			return intstr.IntOrString(p.TargetPort.String()), nil
		}
	}

	return intstr.NullValue, fmt.Errorf("the service does not expose the given svcPort: %s", svcPort)
}

// FindPort returns the port in the Pod that maps to the given port by port number or name
func FindPort(port intstr.IntOrString, pod corev1.Pod) (intstr.IntOrString, error) {
	switch port.Type() {
	case intstr.StringValue:
		for _, container := range pod.Spec.Containers {
			for _, p := range container.Ports {
				if p.Name == port.Str() {
					return intstr.FromInt32(p.ContainerPort), nil
				}
			}
		}

	case intstr.IntValue:
		for _, container := range pod.Spec.Containers {
			for _, p := range container.Ports {
				if p.ContainerPort == port.Int32() {
					return intstr.FromInt32(p.ContainerPort), nil
				}
			}
		}
	}

	return intstr.NullValue, fmt.Errorf("pod %q does exports port %q", pod.Name, port.Str())
}

// HasHostNetwork returns whether a pod has HostNetwork enabled, i.e. it shares the host's network namespace.
func HasHostNetwork(pod corev1.Pod) bool {
	return pod.Spec.HostNetwork
}

// PodIP returns the pod IP for the supplied pod, or an error if it has no IP (yet).
func PodIP(pod corev1.Pod) (string, error) {
	// PodIP must be set if len(PodIPs > 0).
	if ip := pod.Status.PodIP; ip != "" {
		return ip, nil
	}

	return "", fmt.Errorf("pod %s/%s does not have an IP address", pod.Namespace, pod.Name)
}
