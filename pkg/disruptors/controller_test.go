package disruptors

import (
	"context"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
)

func Test_InjectAgent(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		namespace   string
		pod         corev1.Pod
		options     AgentControllerOptions
		expectError bool
	}{
		{
			title:     "Inject ephemeral container",
			namespace: "test-ns",
			pod: builders.NewPodBuilder("pod1").
				WithNamespace("test-ns").
				WithIP("192.0.2.6").
				Build(),
			options: AgentControllerOptions{
				// Set timeout to -1 to prevent waiting the ephemeral container to be ready,
				// as the fake client will not update its status
				Timeout: -1,
			},
			expectError: false,
		},
		{
			title:     "ephemeral container not ready",
			namespace: "test-ns",
			pod: builders.NewPodBuilder("pod1").
				WithNamespace("test-ns").
				WithIP("192.0.2.6").
				Build(),
			options: AgentControllerOptions{
				Timeout: 1,
			},
			expectError: true, // should fail because fake client will not update status
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()
			client := fake.NewSimpleClientset(&tc.pod)
			executor := helpers.NewFakePodCommandExecutor()
			helper := helpers.NewPodHelper(client, executor, tc.namespace)
			controller := NewAgentController(
				helper,
				tc.namespace,
				tc.options,
			)

			err := controller.InjectDisruptorAgent(context.TODO(), tc.pod)
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

			pod, err := client.CoreV1().
				Pods(tc.namespace).
				Get(context.TODO(), tc.pod.Name, metav1.GetOptions{})
			if err != nil {
				t.Errorf("failed: %v", err)
				return
			}

			if len(pod.Spec.EphemeralContainers) == 0 {
				t.Errorf("agent container is not attached")
				return
			}
		})
	}
}

type fakeVisitor struct {
	cmds VisitCommands
	err  error
}

func (v fakeVisitor) Visit(_ corev1.Pod) (VisitCommands, error) {
	return v.cmds, v.err
}

func Test_VisitPod(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		namespace   string
		pod         corev1.Pod
		visitCmds   VisitCommands
		err         error
		stdout      []byte
		stderr      []byte
		options     AgentControllerOptions
		expectError bool
		expected    []helpers.Command
	}{
		{
			title:     "successful execution",
			namespace: "test-ns",
			pod: builders.NewPodBuilder("pod1").
				WithNamespace("test-ns").
				WithIP("192.0.2.6").
				Build(),
			visitCmds: VisitCommands{
				Exec:    []string{"command"},
				Cleanup: []string{"cleanup"},
			},
			err: nil,
			options: AgentControllerOptions{
				Timeout: 1,
			},
			expectError: false,
			expected: []helpers.Command{
				{Pod: "pod1", Container: "xk6-agent", Namespace: "test-ns", Command: []string{"command"}, Stdin: []byte{}},
			},
		},
		{
			title:     "failed execution",
			namespace: "test-ns",
			pod: builders.NewPodBuilder("pod1").
				WithNamespace("test-ns").
				WithIP("192.0.2.6").
				Build(),
			visitCmds: VisitCommands{
				Exec:    []string{"command"},
				Cleanup: []string{"cleanup"},
			},
			err:    fmt.Errorf("fake error"),
			stderr: []byte("error output"),
			options: AgentControllerOptions{
				Timeout: 1,
			},
			expectError: true,
			expected: []helpers.Command{
				{Pod: "pod1", Container: "xk6-agent", Namespace: "test-ns", Command: []string{"command"}, Stdin: []byte{}},
				{Pod: "pod1", Container: "xk6-agent", Namespace: "test-ns", Command: []string{"cleanup"}, Stdin: []byte{}},
			},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			client := fake.NewSimpleClientset(&tc.pod)
			executor := helpers.NewFakePodCommandExecutor()
			helper := helpers.NewPodHelper(client, executor, tc.namespace)
			controller := NewAgentController(
				helper,
				tc.namespace,
				tc.options,
			)

			executor.SetResult(tc.stdout, tc.stderr, tc.err)
			visitor := fakeVisitor{
				cmds: tc.visitCmds,
			}
			err := controller.Visit(context.TODO(), tc.pod, visitor)
			if tc.expectError && err == nil {
				t.Fatalf("should had failed")
			}

			if !tc.expectError && err != nil {
				t.Fatalf("failed unexpectedly: %v", err)
			}

			if tc.expectError && err != nil {
				if !strings.Contains(err.Error(), string(tc.stderr)) {
					t.Fatalf("returned error message should contain stderr (%q)", string(tc.stderr))
				}
			}

			if diff := cmp.Diff(tc.expected, executor.GetHistory()); diff != "" {
				t.Errorf("Expected command did not match returned:\n%s", diff)
			}
		})
	}
}
