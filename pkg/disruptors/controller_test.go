package disruptors

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
)

func Test_InjectAgent(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title     string
		namespace string
		pods      []*corev1.Pod
		// Set timeout to -1 to prevent waiting the ephemeral container to be ready,
		// as the fake client will not update its status
		timeout     time.Duration
		expectError bool
	}{
		{
			title:     "Inject ephemeral container",
			namespace: "test-ns",
			pods: []*corev1.Pod{
				builders.NewPodBuilder("pod1").
					WithNamespace("test-ns").
					Build(),
				builders.NewPodBuilder("pod2").
					WithNamespace("test-ns").
					Build(),
			},
			timeout:     -1,
			expectError: false,
		},
		{
			title:     "ephemeral container not ready",
			namespace: "test-ns",
			pods: []*corev1.Pod{
				builders.NewPodBuilder("pod1").
					WithNamespace("test-ns").
					Build(),
				builders.NewPodBuilder("pod2").
					WithNamespace("test-ns").
					Build(),
			},
			timeout:     1,
			expectError: true, // should fail because fake client will not update status
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			objs := []runtime.Object{}

			targets := []string{}
			for _, pod := range tc.pods {
				objs = append(objs, pod)
				targets = append(targets, pod.Name)
			}

			client := fake.NewSimpleClientset(objs...)
			executor := helpers.NewFakePodCommandExecutor()
			helper := helpers.NewFakePodHelper(client, tc.namespace, executor)
			controller := NewAgentController(
				context.TODO(),
				helper,
				tc.namespace,
				targets,
				tc.timeout,
			)

			err := controller.InjectDisruptorAgent(context.TODO())
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

			for _, podName := range targets {
				pod, err := client.CoreV1().
					Pods(tc.namespace).
					Get(context.TODO(), podName, metav1.GetOptions{})
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
		namespace   string
		pods        []*corev1.Pod
		command     []string
		err         error
		stdout      []byte
		stderr      []byte
		timeout     time.Duration
		expectError bool
	}{
		{
			title:     "successful execution",
			namespace: "test-ns",
			pods: []*corev1.Pod{
				builders.NewPodBuilder("pod1").
					WithNamespace("test-ns").
					Build(),
				builders.NewPodBuilder("pod2").
					WithNamespace("test-ns").
					Build(),
			},
			command:     []string{"echo", "-n", "hello", "world"},
			err:         nil,
			expectError: false,
		},
		{
			title:     "failed execution",
			namespace: "test-ns",
			pods: []*corev1.Pod{
				builders.NewPodBuilder("pod1").
					WithNamespace("test-ns").
					Build(),
				builders.NewPodBuilder("pod2").
					WithNamespace("test-ns").
					Build(),
			},
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

			targets := []string{}
			for _, pod := range tc.pods {
				objs = append(objs, pod)
				targets = append(targets, pod.Name)
			}
			client := fake.NewSimpleClientset(objs...)
			executor := helpers.NewFakePodCommandExecutor()
			helper := helpers.NewFakePodHelper(client, tc.namespace, executor)
			controller := NewAgentController(
				context.TODO(),
				helper,
				tc.namespace,
				targets,
				tc.timeout,
			)

			executor.SetResult(tc.stdout, tc.stderr, tc.err)
			err := controller.ExecCommand(context.TODO(), tc.command)
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

			pods := map[string]bool{}
			for _, p := range targets {
				pods[p] = true
			}

			history := executor.GetHistory()
			if len(history) != len(targets) {
				t.Errorf("invalid number of exec invocations. Expected %d got %d", len(targets), len(history))
			}
			for _, c := range history {
				if _, found := pods[c.Pod]; !found {
					t.Errorf("invalid pod name. Expected to be in %s got %s", targets, c.Pod)
					return
				}
				// TODO: don't use hard-coded agent name
				if c.Container != "xk6-agent" {
					t.Errorf("invalid container name. Expected %s got %s", "xk6-agent", c.Container)
					return
				}
				ec := strings.Join(tc.command, " ")
				ac := strings.Join(c.Command, " ")
				if ac != ec {
					t.Errorf("invalid command executed. Expected %s got %s", ec, ac)
					return
				}
			}
		})
	}
}
