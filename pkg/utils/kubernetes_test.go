package utils

import (
	"testing"

	corev1 "k8s.io/api/core/v1"

	k8sintstr "k8s.io/apimachinery/pkg/util/intstr"

	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
	"github.com/grafana/xk6-disruptor/pkg/types/intstr"
)

func buildPodWithPort(name string, portName string, port int32) corev1.Pod {
	container := builders.NewContainerBuilder(name).
		WithPort(portName, port).
		Build()

	pod := builders.NewPodBuilder(name).
		WithContainer(container).
		Build()

	return pod
}

func buildServicWithPort(name string, portName string, port int32, target k8sintstr.IntOrString) corev1.Service {
	return builders.NewServiceBuilder(name).
		WithNamespace("test-ns").
		WithSelectorLabel("app", "test").
		WithPort(portName, port, target).
		Build()
}

func Test_ServicePortMapping(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		serviceName string
		namespace   string
		service     corev1.Service
		pod         corev1.Pod
		endpoints   *corev1.Endpoints
		port        intstr.IntOrString
		expectError bool
		expected    intstr.IntOrString
	}{
		{
			title:       "invalid Port option",
			serviceName: "test-svc",
			namespace:   "test-ns",
			service:     buildServicWithPort("test-svc", "http", 8080, k8sintstr.FromInt(8080)),
			pod:         buildPodWithPort("pod-1", "http", 80),
			port:        intstr.FromInt(80),
			expectError: true,
			expected:    intstr.FromInt(0),
		},
		{
			title:       "Numeric target port specified",
			serviceName: "test-svc",
			namespace:   "test-ns",
			service:     buildServicWithPort("test-svc", "http", 8080, k8sintstr.FromInt(80)),
			pod:         buildPodWithPort("pod-1", "http", 80),
			port:        intstr.FromInt(8080),
			expectError: false,
			expected:    intstr.FromInt(80),
		},
		{
			title:       "Named target port",
			serviceName: "test-svc",
			namespace:   "test-ns",
			service:     buildServicWithPort("test-svc", "http", 8080, k8sintstr.FromString("http")),
			pod:         buildPodWithPort("pod-1", "http", 80),
			port:        intstr.FromInt32(8080),
			expectError: false,
			expected:    intstr.FromInt(80),
		},
		{
			title:       "Default port mapping",
			serviceName: "test-svc",
			namespace:   "test-ns",
			service:     buildServicWithPort("test-svc", "http", 8080, k8sintstr.FromInt(80)),
			pod:         buildPodWithPort("pod-1", "http", 80),
			port:        intstr.FromInt(0),
			expectError: false,
			expected:    intstr.FromInt(80),
		},
		{
			title:       "No target for mapping",
			serviceName: "test-svc",
			namespace:   "test-ns",
			service:     buildServicWithPort("test-svc", "http", 8080, k8sintstr.FromInt(80)),
			pod:         buildPodWithPort("pod-1", "http", 8080),
			port:        intstr.FromInt(8080),
			expectError: true,
			expected:    intstr.FromInt(0),
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			port, err := MapPort(tc.service, tc.port, tc.pod)
			if !tc.expectError && err != nil {
				t.Errorf(" failed: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if tc.expectError && err != nil {
				return
			}

			if tc.expected != port {
				t.Errorf("expected %q got %q", tc.expected.Str(), port.Str())
				return
			}
		})
	}
}

func Test_ValidatePort(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title      string
		namespace  string
		pod        corev1.Pod
		targetPort intstr.IntOrString
		expect     bool
	}{
		{
			title:     "Pods listen to the specified port",
			namespace: "testns",
			pod: builders.NewPodBuilder("test-pod-1").
				WithContainer(corev1.Container{Ports: []corev1.ContainerPort{{ContainerPort: 8080}}}).
				WithNamespace("testns").
				Build(),
			targetPort: intstr.FromInt(8080),
			expect:     true,
		},
		{
			title:     "Pod doesn't listen to the specified port",
			namespace: "testns",
			pod: builders.NewPodBuilder("test-pod-2").
				WithContainer(corev1.Container{Ports: []corev1.ContainerPort{{ContainerPort: 9090}}}).
				WithNamespace("testns").
				Build(),
			targetPort: intstr.FromInt(8080),
			expect:     false,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			validation := HasPort(tc.pod, tc.targetPort)
			if validation != tc.expect {
				t.Errorf("expected %t got %t", tc.expect, validation)
			}
		})
	}
}
