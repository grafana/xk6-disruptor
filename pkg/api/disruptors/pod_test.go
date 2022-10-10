package disruptors

import (
	"context"
	"fmt"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
)

func Test_PodSelectorWithLabels(t *testing.T) {
	testCases := []struct {
		title        string
		pods         []podDesc
		labels       map[string]string
		exclude      map[string]string
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
			labels: map[string]string{
				"app": "test",
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
			labels: map[string]string{
				"app": "test",
			},
			expectError:  false,
			expectedPods: []string{},
		},
		{
			title: "one matching pod",
			pods: []podDesc{
				{
					name:      "pod-with-app-label",
					namespace: testNamespace,
					labels: map[string]string{
						"app": "test",
					},
				},
			},
			labels: map[string]string{
				"app": "test",
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
					namespace: testNamespace,
					labels: map[string]string{
						"app": "test",
					},
				},
				{
					name:      "another-pod-with-app-label",
					namespace: testNamespace,
					labels: map[string]string{
						"app": "test",
					},
				},
			},
			labels: map[string]string{
				"app": "test",
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
					namespace: testNamespace,
					labels: map[string]string{
						"app": "test",
					},
				},
				{
					name:      "pod-with-dev-label",
					namespace: testNamespace,
					labels: map[string]string{
						"app": "test",
						"env": "dev",
					},
				},
				{
					name:      "pod-with-prod-label",
					namespace: testNamespace,
					labels: map[string]string{
						"app": "test",
						"env": "prod",
					},
				},
			},
			labels: map[string]string{
				"app": "test",
				"env": "dev",
			},
			expectError: false,
			expectedPods: []string{
				"pod-with-dev-label",
			},
		},
		{
			title: "exclude environment",
			pods: []podDesc{
				{
					name:      "pod-with-dev-label",
					namespace: testNamespace,
					labels: map[string]string{
						"app": "test",
						"env": "dev",
					},
				},
				{
					name:      "pod-with-prod-label",
					namespace: testNamespace,
					labels: map[string]string{
						"app": "test",
						"env": "prod",
					},
				},
			},
			labels: map[string]string{
				"app": "test",
			},
			exclude: map[string]string{
				"env": "prod",
			},
			expectError: false,
			expectedPods: []string{
				"pod-with-dev-label",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
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
			selector := PodSelector{
				Namespace: testNamespace,
				Select: PodAttributes{
					Labels: tc.labels,
				},
				Exclude: PodAttributes{
					Labels: tc.exclude,
				},
			}

			targets, err := selector.GetTargets(k)
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
	testCases := []struct {
		title       string
		targets     []string
		timeout     time.Duration
		expectError bool
	}{
		{
			title:       "Inject ephemeral container",
			targets:     []string{"pod1", "pod2"},
			expectError: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			objs := []runtime.Object{}
			for _, podName := range tc.targets {
				pod := builders.NewPodBuilder(podName).WithNamespace(testNamespace).Build()
				objs = append(objs, pod)

			}
			client := fake.NewSimpleClientset(objs...)
			k8s, _ := kubernetes.NewFakeKubernetes(client)
			// Set timeout to 0 to prevent waiting the ephemeral container to be ready, as the fake client will not update its status
			controller := AgentController{
				k8s:       k8s,
				namespace: testNamespace,
				targets:   tc.targets,
				timeout:   0,
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
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
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
				return
			}

		})
	}
}
