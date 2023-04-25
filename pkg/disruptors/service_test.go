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
		service     serviceDesc
		pods        []podDesc
		endpoints   []endpoint
		options     ServiceDisruptorOptions
		expectError bool
	}{
		{
			title:     "one endpoint",
			name:      "test-svc",
			namespace: "test-ns",
			service: serviceDesc{
				name:      "test-svc",
				namespace: "test-ns",
				ports: []corev1.ServicePort{
					{
						Name:       "http",
						Port:       80,
						TargetPort: intstr.FromInt(80),
					},
				},
				selector: map[string]string{
					"app": "test",
				},
			},
			pods: []podDesc{
				{
					name:      "pod-1",
					namespace: "test-ns",
					labels: map[string]string{
						"app": "test",
					},
				},
			},
			endpoints: []endpoint{
				{
					ports: []corev1.EndpointPort{
						{
							Name: "http",
							Port: 80,
						},
					},
					pods: []string{"pod-1"},
				},
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
			service: serviceDesc{
				name:      "test-svc",
				namespace: "test-ns",
				ports: []corev1.ServicePort{
					{
						Port:       80,
						TargetPort: intstr.FromInt(80),
					},
				},
				selector: map[string]string{
					"app": "test",
				},
			},
			pods:        []podDesc{},
			endpoints:   []endpoint{},
			options:     ServiceDisruptorOptions{},
			expectError: false,
		},
		{
			title:     "service does not exist",
			name:      "other-svc",
			namespace: "test-ns",
			service: serviceDesc{
				name:      "test-svc",
				namespace: "test-ns",
				ports: []corev1.ServicePort{
					{
						Port:       80,
						TargetPort: intstr.FromInt(80),
					},
				},
				selector: map[string]string{
					"app": "test",
				},
			},
			pods:        []podDesc{},
			endpoints:   []endpoint{},
			options:     ServiceDisruptorOptions{},
			expectError: true,
		},
		{
			title:     "empty service name",
			name:      "",
			namespace: "test-ns",
			service: serviceDesc{
				name:      "test-svc",
				namespace: "test-ns",
				ports: []corev1.ServicePort{
					{
						Port:       80,
						TargetPort: intstr.FromInt(80),
					},
				},
				selector: map[string]string{
					"app": "test",
				},
			},
			options:     ServiceDisruptorOptions{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			objs := []runtime.Object{}
			svc := builders.NewServiceBuilder(tc.service.name).
				WithNamespace(tc.service.namespace).
				WithSelector(tc.service.selector).
				WithPorts(tc.service.ports).
				Build()
			objs = append(objs, svc)

			for _, p := range tc.pods {
				pod := builders.NewPodBuilder(p.name).
					WithNamespace(p.namespace).
					WithLabels(p.labels).
					Build()
				objs = append(objs, pod)
			}

			epb := builders.NewEndPointsBuilder(tc.service.name).
				WithNamespace(tc.service.namespace)
			for _, ep := range tc.endpoints {
				epb = epb.WithSubset(ep.ports, ep.pods)
			}
			ep := epb.Build()
			objs = append(objs, ep)

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
