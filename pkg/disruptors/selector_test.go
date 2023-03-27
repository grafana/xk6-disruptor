package disruptors

import (
	"context"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
)

func Test_PodSelector(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title        string
		pods         []podDesc
		selector     PodSelector
		expectError  bool
		expectedPods []string
	}{
		{
			title: "No matching pod",
			pods: []podDesc{
				{
					name:      "pod-without-labels",
					namespace: testNamespace,
					labels:    map[string]string{},
				},
			},
			selector: PodSelector{
				Namespace: "test-ns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "test",
					},
				},
			},
			expectError:  false,
			expectedPods: []string{},
		},
		{
			title: "No matching namespace",
			pods: []podDesc{
				{
					name:      "pod-with-app-label-in-another-ns",
					namespace: "anotherNamespace",
					labels: map[string]string{
						"app": "test",
					},
				},
			},
			selector: PodSelector{
				Namespace: "test-ns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "test",
					},
				},
			},
			expectError:  false,
			expectedPods: []string{},
		},
		{
			title: "one matching pod",
			pods: []podDesc{
				{
					name:      "pod-with-app-label",
					namespace: "test-ns",
					labels: map[string]string{
						"app": "test",
					},
				},
			},
			selector: PodSelector{
				Namespace: "test-ns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "test",
					},
				},
			},
			expectError: false,
			expectedPods: []string{
				"pod-with-app-label",
			},
		},
		{
			title: "multiple matching pods",
			pods: []podDesc{
				{
					name:      "pod-with-app-label",
					namespace: "test-ns",
					labels: map[string]string{
						"app": "test",
					},
				},
				{
					name:      "another-pod-with-app-label",
					namespace: "test-ns",
					labels: map[string]string{
						"app": "test",
					},
				},
			},
			selector: PodSelector{
				Namespace: "test-ns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "test",
					},
				},
			},
			expectError: false,
			expectedPods: []string{
				"pod-with-app-label",
				"another-pod-with-app-label",
			},
		},
		{
			title: "multiple selector labels",
			pods: []podDesc{
				{
					name:      "pod-with-app-label",
					namespace: "test-ns",
					labels: map[string]string{
						"app": "test",
					},
				},
				{
					name:      "pod-with-dev-label",
					namespace: "test-ns",
					labels: map[string]string{
						"app": "test",
						"env": "dev",
					},
				},
				{
					name:      "pod-with-prod-label",
					namespace: "test-ns",
					labels: map[string]string{
						"app": "test",
						"env": "prod",
					},
				},
			},
			selector: PodSelector{
				Namespace: "test-ns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "test",
						"env": "dev",
					},
				},
			},
			expectError: false,
			expectedPods: []string{
				"pod-with-dev-label",
			},
		},
		{
			title: "exclude labels",
			pods: []podDesc{
				{
					name:      "pod-with-dev-label",
					namespace: "test-ns",
					labels: map[string]string{
						"app": "test",
						"env": "dev",
					},
				},
				{
					name:      "pod-with-prod-label",
					namespace: "test-ns",
					labels: map[string]string{
						"app": "test",
						"env": "prod",
					},
				},
			},
			selector: PodSelector{
				Namespace: "test-ns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "test",
					},
				},
				Exclude: PodAttributes{
					Labels: map[string]string{
						"env": "prod",
					},
				},
			},
			expectError: false,
			expectedPods: []string{
				"pod-with-dev-label",
			},
		},
		{
			title: "Namespace selector",
			pods: []podDesc{
				{
					name:      "pod-in-test-ns",
					namespace: "test-ns",
					labels:    map[string]string{},
				},
				{
					name:      "another-pod-in-test-ns",
					namespace: "test-ns",
					labels:    map[string]string{},
				},
				{
					name:      "pod-in-another-namespace",
					namespace: "other-ns",
					labels:    map[string]string{},
				},
			},
			selector: PodSelector{
				Namespace: "test-ns",
			},
			expectError: false,
			expectedPods: []string{
				"pod-in-test-ns",
				"another-pod-in-test-ns",
			},
		},
		{
			title:        "Empty selector",
			pods:         []podDesc{},
			selector:     PodSelector{},
			expectError:  true,
			expectedPods: []string{},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			pods := []runtime.Object{}
			for _, p := range tc.pods {
				pod := builders.NewPodBuilder(p.name).
					WithNamespace(p.namespace).
					WithLabels(p.labels).
					Build()
				pods = append(pods, pod)
			}
			client := fake.NewSimpleClientset(pods...)
			k, _ := kubernetes.NewFakeKubernetes(client)
			targets, err := tc.selector.GetTargets(context.TODO(), k)
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
