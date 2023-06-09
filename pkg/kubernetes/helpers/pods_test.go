package helpers

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stest "k8s.io/client-go/testing"

	"github.com/grafana/xk6-disruptor/pkg/testutils/assertions"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
)

const (
	testNamespace = "ns-test"
)

func TestPods_Wait(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		test           string
		name           string
		status         corev1.PodPhase
		delay          time.Duration
		expectError    bool
		expectedResult bool
		timeout        time.Duration
	}

	testCases := []TestCase{
		{
			test:           "wait pod running",
			name:           "pod-running",
			delay:          1 * time.Second,
			status:         corev1.PodRunning,
			expectError:    false,
			expectedResult: true,
			timeout:        5 * time.Second,
		},
		{
			test:           "timeout waiting pod running",
			name:           "pod-running",
			status:         corev1.PodRunning,
			delay:          10 * time.Second,
			expectError:    false,
			expectedResult: false,
			timeout:        5 * time.Second,
		},
		{
			test:           "wait failed pod",
			name:           "pod-running",
			status:         corev1.PodFailed,
			delay:          1 * time.Second,
			expectError:    true,
			expectedResult: false,
			timeout:        5 * time.Second,
		},
	}
	for _, tc := range testCases {
		tc := tc

		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			client := fake.NewSimpleClientset()
			watcher := watch.NewRaceFreeFake()
			client.PrependWatchReactor("pods", k8stest.DefaultWatchReactor(watcher, nil))
			go func(tc TestCase) {
				pod := builders.NewPodBuilder(tc.name).
					WithNamespace(testNamespace).
					WithStatus(tc.status).
					Build()
				time.Sleep(tc.delay)
				watcher.Modify(pod)
			}(tc)

			h := NewPodHelper(client, nil, testNamespace)
			result, err := h.WaitPodRunning(
				context.TODO(),
				tc.name,
				tc.timeout,
			)

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tc.expectError && err == nil {
				t.Error("expected an error but none returned")
				return
			}
			if result != tc.expectedResult {
				t.Errorf("expected result %t but %t returned", tc.expectedResult, result)
				return
			}
		})
	}
}

func TestPods_AddEphemeralContainer(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		test        string
		podName     string
		delay       time.Duration
		expectError bool
		container   string
		state       corev1.ContainerState
		options     AttachOptions
	}

	// TODO: check injecting agent when it is already present in the pod
	testCases := []TestCase{
		{
			test:        "Create ephemeral container not waiting",
			podName:     "test-pod",
			delay:       1 * time.Second,
			expectError: false,
			container:   "ephemeral",
			state: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{},
			},
			options: AttachOptions{
				Timeout:        0,
				IgnoreIfExists: true,
			},
		},
		{
			test:        "Create ephemeral container waiting",
			podName:     "test-pod",
			delay:       3 * time.Second,
			expectError: false,
			container:   "ephemeral",
			state: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{},
			},
			options: AttachOptions{
				Timeout:        5 * time.Second,
				IgnoreIfExists: true,
			},
		},
		{
			test:        "Fail waiting for container",
			podName:     "test-pod",
			delay:       3 * time.Second,
			expectError: true,
			container:   "ephemeral",
			state: corev1.ContainerState{
				Waiting: &corev1.ContainerStateWaiting{},
			},
			options: AttachOptions{
				Timeout:        5 * time.Second,
				IgnoreIfExists: true,
			},
		},
	}
	for _, tc := range testCases {
		tc := tc

		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			pod := builders.NewPodBuilder(tc.podName).
				WithNamespace(testNamespace).
				Build()
			client := fake.NewSimpleClientset(pod)
			watcher := watch.NewRaceFreeFake()
			client.PrependWatchReactor("pods", k8stest.DefaultWatchReactor(watcher, nil))

			// add watcher to update ephemeral container's status
			go func(tc TestCase) {
				time.Sleep(tc.delay)
				pod.Status.EphemeralContainerStatuses = []corev1.ContainerStatus{
					{
						Name:  tc.container,
						State: tc.state,
					},
				}
				watcher.Modify(pod)
			}(tc)

			h := NewPodHelper(client, nil, testNamespace)
			err := h.AttachEphemeralContainer(
				context.TODO(),
				tc.podName,
				corev1.EphemeralContainer{},
				tc.options,
			)
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
		})
	}
}

func Test_ListPods(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title        string
		pods         []*corev1.Pod
		namespace    string
		filter       PodFilter
		expectError  bool
		expectedPods []string
	}{
		{
			title: "No matching pod",
			pods: []*corev1.Pod{
				builders.NewPodBuilder("pod-without-labels").
					WithNamespace("test-ns").
					Build(),
			},
			namespace: "test-ns",
			filter: PodFilter{
				Select: map[string]string{
					"app": "test",
				},
			},
			expectError:  false,
			expectedPods: []string{},
		},
		{
			title:     "No matching namespace",
			namespace: "test-ns",
			pods: []*corev1.Pod{
				builders.NewPodBuilder("pod-with-app-label-in-another-ns").
					WithNamespace("anotherNamespace").
					WithLabels(
						map[string]string{
							"app": "test",
						},
					).
					Build(),
			},
			filter: PodFilter{
				Select: map[string]string{
					"app": "test",
				},
			},
			expectError:  false,
			expectedPods: []string{},
		},
		{
			title:     "one matching pod",
			namespace: "test-ns",
			pods: []*corev1.Pod{
				builders.NewPodBuilder("pod-with-app-label").
					WithNamespace("test-ns").
					WithLabels(
						map[string]string{
							"app": "test",
						},
					).
					Build(),
			},
			filter: PodFilter{
				Select: map[string]string{
					"app": "test",
				},
			},
			expectError: false,
			expectedPods: []string{
				"pod-with-app-label",
			},
		},
		{
			title:     "multiple matching pods",
			namespace: "test-ns",
			pods: []*corev1.Pod{
				builders.NewPodBuilder("pod-with-app-label").
					WithNamespace("test-ns").
					WithLabels(
						map[string]string{
							"app": "test",
						},
					).
					Build(),
				builders.NewPodBuilder("another-pod-with-app-label").
					WithNamespace("test-ns").
					WithLabels(
						map[string]string{
							"app": "test",
						},
					).
					Build(),
			},
			filter: PodFilter{
				Select: map[string]string{
					"app": "test",
				},
			},
			expectError: false,
			expectedPods: []string{
				"pod-with-app-label",
				"another-pod-with-app-label",
			},
		},
		{
			title:     "multiple selector labels",
			namespace: "test-ns",
			pods: []*corev1.Pod{
				builders.NewPodBuilder("pod-with-app-label").
					WithNamespace("test-ns").
					WithLabels(
						map[string]string{
							"app": "test",
						},
					).
					Build(),
				builders.NewPodBuilder("pod-with-dev-label").
					WithNamespace("test-ns").
					WithLabels(
						map[string]string{
							"app": "test",
							"env": "dev",
						},
					).
					Build(),
				builders.NewPodBuilder("pod-with-prod-label").
					WithNamespace("test-ns").
					WithLabels(
						map[string]string{
							"app": "test",
							"env": "prod",
						},
					).
					Build(),
			},
			filter: PodFilter{
				Select: map[string]string{
					"app": "test",
					"env": "dev",
				},
			},
			expectError: false,
			expectedPods: []string{
				"pod-with-dev-label",
			},
		},
		{
			title:     "exclude labels",
			namespace: "test-ns",
			pods: []*corev1.Pod{
				builders.NewPodBuilder("pod-with-dev-label").
					WithNamespace("test-ns").
					WithLabels(
						map[string]string{
							"app": "test",
							"env": "dev",
						},
					).
					Build(),
				builders.NewPodBuilder("pod-with-prod-label").
					WithNamespace("test-ns").
					WithLabels(
						map[string]string{
							"app": "test",
							"env": "prod",
						},
					).
					Build(),
			},
			filter: PodFilter{
				Select: map[string]string{
					"app": "test",
				},
				Exclude: map[string]string{
					"env": "prod",
				},
			},
			expectError: false,
			expectedPods: []string{
				"pod-with-dev-label",
			},
		},
		{
			title:     "Namespace selector",
			namespace: "test-ns",
			pods: []*corev1.Pod{
				builders.NewPodBuilder("pod-in-test-ns").
					WithNamespace("test-ns").
					Build(),
				builders.NewPodBuilder("another-pod-in-test-ns").
					WithNamespace("test-ns").
					Build(),
				builders.NewPodBuilder("pod-in-another-namespace").
					WithNamespace("other-ns").
					Build(),
			},
			filter:      PodFilter{},
			expectError: false,
			expectedPods: []string{
				"pod-in-test-ns",
				"another-pod-in-test-ns",
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			pods := []runtime.Object{}
			for _, p := range tc.pods {
				pods = append(pods, p)
			}
			client := fake.NewSimpleClientset(pods...)

			helper := NewPodHelper(client, nil, tc.namespace)
			podList, err := helper.List(context.TODO(), tc.filter)

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

			if !assertions.CompareStringArrays(podList, tc.expectedPods) {
				t.Errorf("result does not match expected value. Expected: %s\nActual: %s\n", tc.expectedPods, pods)
				return
			}
		})
	}
}

func Test_ValidatePort(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		namespace   string
		pods        []*corev1.Pod
		targetPort  uint
		expectError bool
	}{
		{
			title:     "Pods listen to the specified port",
			namespace: "testns",
			pods: []*corev1.Pod{
				builders.NewPodBuilder("test-pod-1").
					WithContainer(corev1.Container{Ports: []corev1.ContainerPort{{ContainerPort: 8080}}}).
					WithNamespace("testns").
					Build(),
				builders.NewPodBuilder("test-pod-2").
					WithContainer(corev1.Container{Ports: []corev1.ContainerPort{{ContainerPort: 8080}}}).
					WithNamespace("testns").
					Build(),
			},
			targetPort:  8080,
			expectError: false,
		},
		{
			title:     "One pod doesn't listen to the specified port",
			namespace: "testns",
			pods: []*corev1.Pod{
				builders.NewPodBuilder("test-pod-1").
					WithContainer(corev1.Container{Ports: []corev1.ContainerPort{{ContainerPort: 8080}}}).
					WithNamespace("testns").
					Build(),
				builders.NewPodBuilder("test-pod-2").
					WithContainer(corev1.Container{Ports: []corev1.ContainerPort{{ContainerPort: 9090}}}).
					WithNamespace("testns").
					Build(),
			},
			targetPort:  8080,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			pods := []runtime.Object{}
			for _, p := range tc.pods {
				pods = append(pods, p)
			}

			client := fake.NewSimpleClientset(pods...)

			helper := NewPodHelper(client, nil, tc.namespace)
			err := helper.ValidatePort(context.TODO(), PodFilter{}, tc.targetPort)

			if err != nil && !tc.expectError {
				t.Errorf("unexpected error: %v", err)
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}
		})
	}
}
