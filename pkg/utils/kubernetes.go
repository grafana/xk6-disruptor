package utils

import (
	"fmt"

	"github.com/grafana/xk6-disruptor/pkg/types/intstr"
	corev1 "k8s.io/api/core/v1"
)

// GetTargetPort returns the target port for the given service port
func GetTargetPort(service corev1.Service, svcPort intstr.IntOrString) (intstr.IntOrString, error) {
	// Handle default port mapping
	// TODO: make port required
	if svcPort.IsNull() || svcPort.IsZero() {
		if len(service.Spec.Ports) > 1 {
			return intstr.NullValue, fmt.Errorf("no port selected and service exposes more than one service")
		}
		return intstr.IntOrString(service.Spec.Ports[0].TargetPort.String()), nil
	}

	for _, p := range service.Spec.Ports {
		if svcPort.Str() == p.Name || (svcPort.IsInt() && svcPort.Int32() == p.Port) {
			return intstr.IntOrString(p.TargetPort.String()), nil
		}
	}

	return intstr.NullValue, fmt.Errorf("the service does not expose the given svcPort: %s", svcPort)
}

// FindPort returns the port in the Pod that maps to the given port by port number or name
func FindPort(port intstr.IntOrString, pod corev1.Pod) (intstr.IntOrString, error) {
	switch port.Type() {
	case intstr.ValueTypeString:
		for _, container := range pod.Spec.Containers {
			for _, p := range container.Ports {
				if p.Name == port.Str() {
					return intstr.FromInt32(p.ContainerPort), nil
				}
			}
		}

	case intstr.ValueTypeInt:
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

// PodNames return the name of the pods in a list
func PodNames(pods []corev1.Pod) []string {
	names := make([]string, 0, len(pods))
	for _, pod := range pods {
		names = append(names, pod.Name)
	}

	return names
}

// Sample a subset of the given list of Pods
func Sample(pods []corev1.Pod, count int) ([]corev1.Pod, error) {
	if count > len(pods) {
		return nil, fmt.Errorf("cannot sample %d pods out of a total of %d", count, len(pods))
	}

	return pods[:count], nil
}
