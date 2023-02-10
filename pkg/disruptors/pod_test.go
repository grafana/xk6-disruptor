package disruptors

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func Test_InjectAgent(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title   string
		targets []string
		// Set timeout to 0 to prevent waiting the ephemeral container to be ready,
		// as the fake client will not update its status
		timeout     time.Duration
		expectError bool
	}{
		{
			title:       "Inject ephemeral container",
			targets:     []string{"pod1", "pod2"},
			timeout:     0,
			expectError: false,
		},
		{
			title:       "ephemeral container not ready",
			targets:     []string{"pod1", "pod2"},
			timeout:     1,
			expectError: true, // should fail because fake client will not update status
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			objs := []runtime.Object{}
			for _, podName := range tc.targets {
				pod := builders.NewPodBuilder(podName).WithNamespace(testNamespace).Build()
				objs = append(objs, pod)
			}
			client := fake.NewSimpleClientset(objs...)
			k8s, _ := kubernetes.NewFakeKubernetes(client)
			controller := AgentController{
				k8s:       k8s,
				namespace: testNamespace,
				targets:   tc.targets,
				timeout:   tc.timeout,
			}

			err := controller.InjectDisruptorAgent()
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

			for _, podName := range tc.targets {
				pod, err := client.CoreV1().Pods(testNamespace).Get(context.TODO(), podName, metav1.GetOptions{})
				if err != nil {
					t.Errorf("failed: %v", err)
					return
				}

				if len(pod.Spec.EphemeralContainers) == 0 {
					t.Errorf("agent container is not attached")
					return
				}
			}
		})
	}
}

func Test_ExecCommand(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		targets     []string
		command     []string
		err         error
		stdout      []byte
		stderr      []byte
		timeout     time.Duration
		expectError bool
	}{
		{
			title:       "successful execution",
			targets:     []string{"pod1", "pod2"},
			command:     []string{"echo", "-n", "hello", "world"},
			err:         nil,
			expectError: false,
		},
		{
			title:       "failed execution",
			targets:     []string{"pod1", "pod2"},
			command:     []string{"echo", "-n", "hello", "world"},
			err:         fmt.Errorf("fake error"),
			stderr:      []byte("error output"),
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			objs := []runtime.Object{}
			for _, podName := range tc.targets {
				pod := builders.NewPodBuilder(podName).WithNamespace(testNamespace).Build()
				objs = append(objs, pod)
			}
			client := fake.NewSimpleClientset(objs...)
			k8s, _ := kubernetes.NewFakeKubernetes(client)
			executor := k8s.GetFakeProcessExecutor()
			executor.SetResult(tc.stdout, tc.stderr, tc.err)
			controller := AgentController{
				k8s:       k8s,
				namespace: testNamespace,
				targets:   tc.targets,
				timeout:   tc.timeout,
			}
			err := controller.ExecCommand()
			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
				return
			}

			if tc.expectError && err != nil {
				if !strings.Contains(err.Error(), string(tc.stderr)) {
					t.Errorf("invalid error message. Expected to contain %s", string(tc.stderr))
				}
				return
			}
		})
	}
}
