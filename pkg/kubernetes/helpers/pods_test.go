package helpers

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes/fake"
	k8stest "k8s.io/client-go/testing"

	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
)

const (
	testName      = "pod-test"
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
		t.Run(tc.test, func(t *testing.T) {
			client := fake.NewSimpleClientset()
			watcher := watch.NewRaceFreeFake()
			client.PrependWatchReactor("pods", k8stest.DefaultWatchReactor(watcher, nil))
			h := NewHelper(client, nil, context.TODO(), testNamespace)
			go func(tc TestCase) {
				pod := builders.NewPodBuilder(tc.name).
					WithNamespace(testNamespace).
					WithStatus(tc.status).
					Build()
				time.Sleep(tc.delay)
				watcher.Modify(pod)
			}(tc)

			result, err := h.WaitPodRunning(
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

var containerRunning = corev1.ContainerState{
	Running: &corev1.ContainerStateRunning{},
}

var containerWaiting = corev1.ContainerState{
	Waiting: &corev1.ContainerStateWaiting{},
}

func TestPods_AddEphemeralContainer(t *testing.T) {
	t.Parallel()
	type TestCase struct {
		test        string
		podName     string
		delay       time.Duration
		expectError bool
		container   string
		image       string
		state       corev1.ContainerState
		timeout     time.Duration
	}

	testCases := []TestCase{
		{
			test:        "Create ephemeral container not waiting",
			podName:     "test-pod",
			delay:       1 * time.Second,
			expectError: false,
			container:   "ephemeral",
			image:       "busybox",
			state:       containerWaiting,
			timeout:     0,
		},
		{
			test:        "Create ephemeral container waiting",
			podName:     "test-pod",
			delay:       3 * time.Second,
			expectError: false,
			container:   "ephemeral",
			image:       "busybox",
			state:       containerRunning,
			timeout:     5 * time.Second,
		},
		{
			test:        "Fail waiting for container",
			podName:     "test-pod",
			delay:       3 * time.Second,
			expectError: true,
			container:   "ephemeral",
			image:       "busybox",
			state:       containerWaiting,
			timeout:     5 * time.Second,
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
			h := NewHelper(client, nil, context.TODO(), testNamespace)

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

			err := h.AttachEphemeralContainer(
				tc.podName,
				corev1.EphemeralContainer{},
				tc.timeout,
			)
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
		})
	}
}
