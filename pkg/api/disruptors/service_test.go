package disruptors

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
)

func Test_ServiceSelector(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title        string
		service      string
		namespace    string
		pods         []podDesc
		selector     map[string]string
		expectError  bool
		expectedPods []string
	}{
		{
			title:     "one matching pod",
			service:   "test-svc",
			namespace: testNamespace,
			pods: []podDesc{
				{
					name:      "pod-1",
					namespace: testNamespace,
					labels: map[string]string{
						"app": "test",
					},
				},
			},
			selector: map[string]string{
				"app": "test",
			},
			expectError:  false,
			expectedPods: []string{"pod-1"},
		},
		{
			title:     "no matching pod",
			service:   "test-svc",
			namespace: testNamespace,
			pods: []podDesc{
				{
					name:      "pod-1",
					namespace: testNamespace,
					labels: map[string]string{
						"app": "other-app",
					},
				},
			},
			selector: map[string]string{
				"app": "test",
			},
			expectError:  false,
			expectedPods: []string{},
		},
		{
			title:     "no pods",
			service:   "test-svc",
			namespace: testNamespace,
			pods:      []podDesc{},
			selector: map[string]string{
				"app": "test",
			},
			expectError:  false,
			expectedPods: []string{},
		},
		{
			title:     "pods in another namespace",
			service:   "test-svc",
			namespace: testNamespace,
			pods: []podDesc{
				{
					name:      "pod-1",
					namespace: "another-ns",
					labels: map[string]string{
						"app": "test",
					},
				},
			},
			selector: map[string]string{
				"app": "test",
			},
			expectError:  false,
			expectedPods: []string{},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			objs := []runtime.Object{}
			svc := builders.NewServiceBuilder(tc.service).
				WithNamespace(tc.namespace).
				WithSelector(tc.selector).
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

			d, err := NewServiceDisruptor(k, tc.service, tc.namespace, ServiceDisruptorOptions{})

			if !tc.expectError && err != nil {
				t.Errorf(" unexpected error creating service disruptor: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed creating service disruptor")
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
