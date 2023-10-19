package utils

import (
	"fmt"

	"github.com/grafana/xk6-disruptor/pkg/types/intstr"
	corev1 "k8s.io/api/core/v1"
	k8sintstr "k8s.io/apimachinery/pkg/util/intstr"
)

// getTargetPort returns the ServicePort object that corresponds to the given port searching for a match
// for the port number or port name
func getTargetPort(service corev1.Service, svcPort intstr.IntOrString) (corev1.ServicePort, error) {
	// Handle default port mapping
	// TODO: make port required
	if svcPort == intstr.NullValue || (svcPort.IsInt() && svcPort.Int32() == 0) {
		if len(service.Spec.Ports) > 1 {
			return corev1.ServicePort{}, fmt.Errorf("no port selected and service exposes more than one service")
		}
		return service.Spec.Ports[0], nil
	}

	for _, p := range service.Spec.Ports {
		if p.Port == svcPort.Int32() || p.Name == svcPort.Str() {
			return p, nil
		}
	}
	return corev1.ServicePort{}, fmt.Errorf("the service does not expose the given svcPort: %s", svcPort)
}

// MapPort returns the port in the Pod that maps to the given service port
func MapPort(service corev1.Service, port intstr.IntOrString, pod corev1.Pod) (intstr.IntOrString, error) {
	svcPort, err := getTargetPort(service, port)
	if err != nil {
		return intstr.NullValue, err
	}

	switch svcPort.TargetPort.Type {
	case k8sintstr.String:
		for _, container := range pod.Spec.Containers {
			for _, port := range container.Ports {
				if port.Name == svcPort.TargetPort.StrVal {
					return intstr.FromInt32(port.ContainerPort), nil
				}
			}
		}

	case k8sintstr.Int:
		for _, container := range pod.Spec.Containers {
			for _, port := range container.Ports {
				if port.ContainerPort == svcPort.TargetPort.IntVal {
					return intstr.FromInt32(port.ContainerPort), nil
				}
			}
		}
	}

	return intstr.NullValue, fmt.Errorf("pod %q does match port %q for service %q", pod.Name, port.Str(), service.Name)
}

// HasPort verifies if a pods listen to the given port
func HasPort(pod corev1.Pod, port intstr.IntOrString) bool {
	for _, container := range pod.Spec.Containers {
		for _, containerPort := range container.Ports {
			if containerPort.ContainerPort == port.Int32() {
				return true
			}
		}
	}
	return false
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
