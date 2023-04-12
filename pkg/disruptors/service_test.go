package disruptors

import (
	"context"
	"reflect"
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

func Test_ServicePortMapping(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		service     serviceDesc
		endpoints   []endpoint
		port        uint
		expectError bool
		targets     map[string]uint
	}{
		{
			title: "invalid Port option",
			service: serviceDesc{
				name:      "test-svc",
				namespace: testNamespace,
				ports: []corev1.ServicePort{
					{
						Name:       "http",
						Port:       8080,
						TargetPort: intstr.FromInt(8080),
					},
				},
				selector: map[string]string{
					"app": "test",
				},
			},
			port: 80,
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
			targets:     map[string]uint{},
			expectError: true,
		},
		{
			title: "Numeric target port specified",
			service: serviceDesc{
				name:      "test-svc",
				namespace: testNamespace,
				ports: []corev1.ServicePort{
					{
						Name:       "http",
						Port:       8080,
						TargetPort: intstr.FromInt(80),
					},
				},
				selector: map[string]string{
					"app": "test",
				},
			},
			port: 8080,
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
			expectError: false,
			targets: map[string]uint{
				"pod-1": 80,
			},
		},
		{
			title: "named target port",
			service: serviceDesc{
				name:      "test-svc",
				namespace: testNamespace,
				ports: []corev1.ServicePort{
					{
						Name:       "http",
						Port:       8080,
						TargetPort: intstr.FromString("http"),
					},
				},
				selector: map[string]string{
					"app": "test",
				},
			},
			port: 8080,
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
			expectError: false,
			targets: map[string]uint{
				"pod-1": 80,
			},
		},
		{
			title: "Multiple target ports",
			service: serviceDesc{
				name:      "test-svc",
				namespace: testNamespace,
				ports: []corev1.ServicePort{
					{
						Name:       "http",
						Port:       80,
						TargetPort: intstr.FromString("http"),
					},
				},
				selector: map[string]string{
					"app": "test",
				},
			},
			port: 80,
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
				{
					ports: []corev1.EndpointPort{
						{
							Name: "http",
							Port: 8080,
						},
					},
					pods: []string{"pod-2"},
				},
			},
			expectError: false,
			targets: map[string]uint{
				"pod-1": 80,
				"pod-2": 8080,
			},
		},
		{
			title: "Default port mapping",
			service: serviceDesc{
				name:      "test-svc",
				namespace: testNamespace,
				ports: []corev1.ServicePort{
					{
						Name:       "http",
						Port:       8080,
						TargetPort: intstr.FromInt(80),
					},
				},
				selector: map[string]string{
					"app": "test",
				},
			},
			port: 0,
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
			targets: map[string]uint{
				"pod-1": 80,
			},
			expectError: false,
		},
		{
			title: "No target for mapping",
			service: serviceDesc{
				name:      "test-svc",
				namespace: testNamespace,
				ports: []corev1.ServicePort{
					{
						Port:       8080,
						TargetPort: intstr.FromInt(80),
					},
				},
				selector: map[string]string{
					"app": "test",
				},
			},
			port: 8080,
			endpoints: []endpoint{
				{
					ports: []corev1.EndpointPort{
						{
							Name: "http",
							Port: 8080,
						},
					},
					pods: []string{"pod-1"},
				},
			},
			expectError: false,
			targets:     map[string]uint{},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			svc := builders.NewServiceBuilder(tc.service.name).
				WithNamespace(tc.service.namespace).
				WithSelector(tc.service.selector).
				WithPorts(tc.service.ports).
				Build()

			epb := builders.NewEndPointsBuilder(tc.service.name).
				WithNamespace(tc.service.namespace)
			for _, ep := range tc.endpoints {
				epb = epb.WithSubset(ep.ports, ep.pods)
			}
			ep := epb.Build()

			m := NewPortMapper(
				context.TODO(),
				svc,
				ep,
			)

			targets, err := m.Map(tc.port)
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

			if !reflect.DeepEqual(tc.targets, targets) {
				t.Errorf("expected port %v in fault injection got %v", tc.targets, targets)
				return
			}
		})
	}
}

func Test_Targets(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title        string
		serviceName  string
		namespace    string
		service      serviceDesc
		pods         []podDesc
		options      ServiceDisruptorOptions
		endpoints    []endpoint
		expectError  bool
		expectedPods []string
	}{
		{
			title:       "one endpoint",
			serviceName: "test-svc",
			namespace:   "test-ns",
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
				InjectTimeout: -1, // do not wait for agent to be injected
			},
			expectError:  false,
			expectedPods: []string{"pod-1"},
		},
		{
			title:       "no endpoints",
			serviceName: "test-svc",
			namespace:   "test-ns",
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
			endpoints:    []endpoint{},
			options:      ServiceDisruptorOptions{},
			expectError:  false,
			expectedPods: []string{},
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

			epb := builders.NewEndPointsBuilder(tc.serviceName).
				WithNamespace(tc.namespace)
			for _, ep := range tc.endpoints {
				epb = epb.WithSubset(ep.ports, ep.pods)
			}

			endpoints := epb.Build()
			objs = append(objs, endpoints)

			client := fake.NewSimpleClientset(objs...)
			k, _ := kubernetes.NewFakeKubernetes(client)

			d, err := NewServiceDisruptor(
				context.TODO(),
				k,
				tc.serviceName,
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

			targets, err := d.Targets()
			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
				return
			}

			if !compareStringArrays(tc.expectedPods, targets) {
				t.Errorf("result does not match expected value. Expected: %s\nActual: %s\n", tc.expectedPods, targets)
				return
			}
		})
	}
}
