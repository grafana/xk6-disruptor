package disruptors

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
)

func Test_NewServiceDisruptor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		name        string
		namespace   string
		service     *corev1.Service
		pods        []*corev1.Pod
		endpoints   []*corev1.Endpoints
		options     ServiceDisruptorOptions
		expectError bool
	}{
		{
			title:     "one endpoint",
			name:      "test-svc",
			namespace: "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelector(map[string]string{
					"app": "test",
				}).
				WithPorts([]corev1.ServicePort{
					{
						Name:       "http",
						Port:       80,
						TargetPort: intstr.FromInt(80),
					},
				}).
				Build(),
			pods: []*corev1.Pod{
				builders.NewPodBuilder("pod-1").
					WithNamespace("test-ns").
					WithLabels(map[string]string{
						"app": "test",
					}).Build(),
			},
			endpoints: []*corev1.Endpoints{
				builders.NewEndPointsBuilder("test-svc").
					WithNamespace("test-ns").
					WithSubset(
						[]corev1.EndpointPort{
							{
								Name: "http",
								Port: 80,
							},
						},
						[]string{"pod-1"}).
					Build(),
			},
			options: ServiceDisruptorOptions{
				InjectTimeout: -1,
			},
			expectError: false,
		},
		{
			title:     "no endpoints",
			name:      "test-svc",
			namespace: "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelector(map[string]string{
					"app": "test",
				}).
				WithPorts([]corev1.ServicePort{
					{
						Name:       "http",
						Port:       80,
						TargetPort: intstr.FromInt(80),
					},
				}).
				Build(),
			pods:        []*corev1.Pod{},
			endpoints:   []*corev1.Endpoints{},
			options:     ServiceDisruptorOptions{},
			expectError: false,
		},
		{
			title:     "service does not exist",
			name:      "test-svc",
			namespace: "test-ns",
			service: builders.NewServiceBuilder("other-svc").
				WithNamespace("test-ns").
				WithSelector(map[string]string{
					"app": "test",
				}).
				WithPorts([]corev1.ServicePort{
					{
						Name:       "http",
						Port:       80,
						TargetPort: intstr.FromInt(80),
					},
				}).
				Build(),
			pods:        []*corev1.Pod{},
			endpoints:   []*corev1.Endpoints{},
			options:     ServiceDisruptorOptions{},
			expectError: true,
		},
		{
			title:     "empty service name",
			name:      "",
			namespace: "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelector(map[string]string{
					"app": "test",
				}).
				WithPorts([]corev1.ServicePort{
					{
						Name:       "http",
						Port:       80,
						TargetPort: intstr.FromInt(80),
					},
				}).
				Build(),
			pods:        []*corev1.Pod{},
			endpoints:   []*corev1.Endpoints{},
			options:     ServiceDisruptorOptions{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			objs := []runtime.Object{}
			objs = append(objs, tc.service)
			for _, p := range tc.pods {
				objs = append(objs, p)
			}
			for _, e := range tc.endpoints {
				objs = append(objs, e)
			}

			client := fake.NewSimpleClientset(objs...)
			k, _ := kubernetes.NewFakeKubernetes(client)

			_, err := NewServiceDisruptor(
				context.TODO(),
				k,
				tc.name,
				tc.namespace,
				tc.options,
			)

			if !tc.expectError && err != nil {
				t.Errorf(" unexpected error creating service disruptor: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed creating service disruptor")
				return
			}

			if tc.expectError && err != nil {
				return
			}
		})
	}
}
