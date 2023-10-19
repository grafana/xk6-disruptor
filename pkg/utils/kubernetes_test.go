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

func Test_FindPort(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		pod         corev1.Pod
		port        intstr.IntOrString
		expectError bool
		expected    intstr.IntOrString
	}{
		{
			title:       "Numeric port",
			pod:         buildPodWithPort("pod-1", "http", 80),
			port:        intstr.FromInt(80),
			expectError: false,
			expected:    intstr.FromInt(80),
		},
		{
			title:       "Numeric port not exposed",
			pod:         buildPodWithPort("pod-1", "http", 80),
			port:        intstr.FromInt(8080),
			expectError: true,
			expected:    intstr.NullValue,
		},
		{
			title:       "Named port",
			pod:         buildPodWithPort("pod-1", "http", 80),
			port:        intstr.FromString("http"),
			expectError: false,
			expected:    intstr.FromInt(80),
		},
		{
			title:       "Named port not exposed port",
			pod:         buildPodWithPort("pod-1", "http", 80),
			port:        intstr.FromString("http2"),
			expectError: true,
			expected:    intstr.NullValue,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			port, err := FindPort(tc.port, tc.pod)
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

func Test_GetTargetPort(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title string

		service     corev1.Service
		endpoints   *corev1.Endpoints
		port        intstr.IntOrString
		expectError bool
		expected    intstr.IntOrString
	}{
		{
			title:       "Numeric service port specified",
			service:     buildServicWithPort("test-svc", "http", 8080, k8sintstr.FromInt(80)),
			port:        intstr.FromInt(8080),
			expectError: false,
			expected:    intstr.FromInt(80),
		},
		{
			title:       "Named service port",
			service:     buildServicWithPort("test-svc", "http", 8080, k8sintstr.FromInt(80)),
			port:        intstr.FromString("http"),
			expectError: false,
			expected:    intstr.FromInt(80),
		},
		{
			title:       "Named target port",
			service:     buildServicWithPort("test-svc", "http", 8080, k8sintstr.FromString("http")),
			port:        intstr.FromInt(8080),
			expectError: false,
			expected:    intstr.FromString("http"),
		},
		{
			title:       "Default port mapping",
			service:     buildServicWithPort("test-svc", "http", 8080, k8sintstr.FromInt(80)),
			port:        intstr.FromInt(0),
			expectError: false,
			expected:    intstr.FromInt(80),
		},
		{
			title:       "Numeric port not exposed",
			service:     buildServicWithPort("test-svc", "http", 80, k8sintstr.FromInt(80)),
			port:        intstr.FromInt(8080),
			expectError: true,
		},
		{
			title:       "Named port not exposed",
			service:     buildServicWithPort("test-svc", "http", 80, k8sintstr.FromString("http")),
			port:        intstr.FromString("http2"),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			port, err := GetTargetPort(tc.service, tc.port)
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
