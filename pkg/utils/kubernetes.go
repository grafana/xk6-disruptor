package utils

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// getTargetPort returns the ServicePort object that corresponds to the given port number
// if the given port is 0, it will return the first port or error if more than one port is defined
func getTargetPort(service corev1.Service, svcPort uint) (corev1.ServicePort, error) {
	ports := service.Spec.Ports
	if svcPort != 0 {
		for _, p := range ports {
			if uint(p.Port) == svcPort {
				return p, nil
			}
		}
		return corev1.ServicePort{}, fmt.Errorf("the service does not expose the given svcPort: %d", svcPort)
	}

	if len(ports) > 1 {
		return corev1.ServicePort{}, fmt.Errorf("service exposes multiple ports. Port option must be defined")
	}

	return ports[0], nil
}

// MapPort returns the port in the Pod that maps to the given service port
func MapPort(service corev1.Service, port uint, pod corev1.Pod) (uint, error) {
	svcPort, err := getTargetPort(service, port)
	if err != nil {
		return 0, err
	}

	switch svcPort.TargetPort.Type {
	case intstr.String:
		for _, container := range pod.Spec.Containers {
			for _, port := range container.Ports {
				if port.Name == svcPort.TargetPort.StrVal {
					return uint(port.ContainerPort), nil
				}
			}
		}

	case intstr.Int:
		for _, container := range pod.Spec.Containers {
			for _, port := range container.Ports {
				if port.ContainerPort == svcPort.TargetPort.IntVal {
					return uint(port.ContainerPort), nil
				}
			}
		}
	}

	return 0, fmt.Errorf("pod %q does match port %d for service %q", pod.Name, port, service.Name)
}

// HasPort verifies if a pods listen to the given port
func HasPort(pod corev1.Pod, port uint) bool {
	for _, container := range pod.Spec.Containers {
		for _, containerPort := range container.Ports {
			if uint(containerPort.ContainerPort) == port {
				return true
			}
		}
	}
	return false
}

// PodIP returns the pod IP for the supplied pod, or an error if it has no IP (yet).
func PodIP(pod corev1.Pod) (string, error) {
	// PodIP must be set if len(PodIPs > 0).
	if ip := pod.Status.PodIP; ip != "" {
		return ip, nil
	}

	return "", fmt.Errorf("pod %s/%s does not have an IP address", pod.Namespace, pod.Name)
}
