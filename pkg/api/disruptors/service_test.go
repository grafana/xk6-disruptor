package disruptors

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
)

type serviceDesc struct {
	name      string
	namespace string
	ports     []corev1.ServicePort
	selector  map[string]string
}

func Test_NewServiceDisruptor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title        string
		service      serviceDesc
		options      ServiceDisruptorOptions
		pods         []podDesc
		expectError  bool
		expectedPods []string
	}{
		{
			title: "one matching pod",
			service: serviceDesc{
				name:      "test-svc",
				namespace: testNamespace,
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
			options: ServiceDisruptorOptions{},
			pods: []podDesc{
				{
					name:      "pod-1",
					namespace: testNamespace,
					labels: map[string]string{
						"app": "test",
					},
				},
			},
			expectError:  false,
			expectedPods: []string{"pod-1"},
		},
		{
			title: "no matching pod",
			service: serviceDesc{
				name:      "test-svc",
				namespace: testNamespace,
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
			options: ServiceDisruptorOptions{},
			pods: []podDesc{
				{
					name:      "pod-1",
					namespace: testNamespace,
					labels: map[string]string{
						"app": "other-app",
					},
				},
			},
			expectError:  false,
			expectedPods: []string{},
		},
		{
			title: "no pods",
			service: serviceDesc{
				name:      "test-svc",
				namespace: testNamespace,
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
			options:      ServiceDisruptorOptions{},
			pods:         []podDesc{},
			expectError:  false,
			expectedPods: []string{},
		},
		{
			title: "pods in another namespace",
			service: serviceDesc{
				name:      "test-svc",
				namespace: testNamespace,
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
			options: ServiceDisruptorOptions{},
			pods: []podDesc{
				{
					name:      "pod-1",
					namespace: "another-ns",
					labels: map[string]string{
						"app": "test",
					},
				},
			},
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
			client := fake.NewSimpleClientset(objs...)
			k, _ := kubernetes.NewFakeKubernetes(client)

			// Force no wait for agent injection as the mock client will not update its status
			tc.options.InjectTimeout = -1
			d, err := NewServiceDisruptor(
				k,
				tc.service.name,
				tc.service.namespace,
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
			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
				return
			}

			if tc.expectError && err != nil {
				return
			}

			if !compareStringArrays(tc.expectedPods, targets) {
				t.Errorf("result does not match expected value. Expected: %s\nActual: %s\n", tc.expectedPods, targets)
				return
			}
		})
	}
}

// TODO: check the commands sent to the pods
func Test_HTTPFaultInjection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		service     serviceDesc
		options     ServiceDisruptorOptions
		fault       HTTPFault
		duration    uint
		httpOptions HTTPDisruptionOptions
		pods        []podDesc
		expectError bool
	}{
		{
			title: "invalid Port option",
			service: serviceDesc{
				name:      "test-svc",
				namespace: testNamespace,
				ports: []corev1.ServicePort{
					{
						Port:       8080,
						TargetPort: intstr.FromInt(8080),
					},
				},
				selector: map[string]string{
					"app": "test",
				},
			},
			options: ServiceDisruptorOptions{
				InjectTimeout: -1,
			},
			fault: HTTPFault{
				Port: 80,
			},
			duration:    1,
			httpOptions: HTTPDisruptionOptions{},
			pods: []podDesc{
				{
					name:      "pod-1",
					namespace: testNamespace,
					labels: map[string]string{
						"app": "test",
					},
				},
			},
			expectError: true,
		},
		{
			title: "Port option specified",
			service: serviceDesc{
				name:      "test-svc",
				namespace: testNamespace,
				ports: []corev1.ServicePort{
					{
						Port:       8080,
						TargetPort: intstr.FromInt(8080),
					},
				},
				selector: map[string]string{
					"app": "test",
				},
			},
			options: ServiceDisruptorOptions{
				InjectTimeout: -1,
			},
			fault: HTTPFault{
				Port: 8080,
			},
			duration:    1,
			httpOptions: HTTPDisruptionOptions{},
			pods: []podDesc{
				{
					name:      "pod-1",
					namespace: testNamespace,
					labels: map[string]string{
						"app": "test",
					},
				},
			},
			expectError: false,
		},
		{
			title: "default option specified",
			service: serviceDesc{
				name:      "test-svc",
				namespace: testNamespace,
				ports: []corev1.ServicePort{
					{
						Port:       8080,
						TargetPort: intstr.FromInt(8080),
					},
				},
				selector: map[string]string{
					"app": "test",
				},
			},
			options: ServiceDisruptorOptions{
				InjectTimeout: -1,
			},
			fault: HTTPFault{
				Port: 0,
			},
			duration:    1,
			httpOptions: HTTPDisruptionOptions{},
			pods: []podDesc{
				{
					name:      "pod-1",
					namespace: testNamespace,
					labels: map[string]string{
						"app": "test",
					},
				},
			},
			expectError: false,
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
			client := fake.NewSimpleClientset(objs...)
			k, _ := kubernetes.NewFakeKubernetes(client)

			// Force no wait for agent injection as the mock client will not update its status
			tc.options.InjectTimeout = -1
			d, err := NewServiceDisruptor(
				k,
				tc.service.name,
				tc.service.namespace,
				tc.options,
			)

			if !tc.expectError && err != nil {
				t.Errorf(" unexpected error creating service disruptor: %v", err)
				return
			}

			err = d.InjectHTTPFaults(tc.fault, tc.duration, tc.httpOptions)
			if !tc.expectError && err != nil {
				t.Errorf(" unexpected error creating service disruptor: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if tc.expectError && err != nil {
				return
			}
		})
	}
}
