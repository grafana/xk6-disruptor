package disruptors

import (
	"context"
	"fmt"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
)

type fakeCommand struct {
	err     error
	exec    []string
	cleanup []string
}

func (f fakeCommand) Commands(_ corev1.Pod) ([]string, []string, error) {
	return f.exec, f.cleanup, f.err
}

func visitCommands() PodVisitCommand {
	return fakeCommand{
		exec:    []string{"command"},
		cleanup: []string{"cleanup"},
	}
}

func Test_VisitPod(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		namespace   string
		pod         corev1.Pod
		visitCmds   PodVisitCommand
		err         error
		stdout      []byte
		stderr      []byte
		options     PodAgentVisitorOptions
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
			visitCmds: visitCommands(),
			err:       nil,
			options: PodAgentVisitorOptions{
				Timeout: -1,
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
			visitCmds: visitCommands(),
			err:       fmt.Errorf("fake error"),
			stderr:    []byte("error output"),
			options: PodAgentVisitorOptions{
				Timeout: -1,
			},
			expectError: true,
			expected: []helpers.Command{
				{Pod: "pod1", Container: "xk6-agent", Namespace: "test-ns", Command: []string{"command"}, Stdin: []byte{}},
				{Pod: "pod1", Container: "xk6-agent", Namespace: "test-ns", Command: []string{"cleanup"}, Stdin: []byte{}},
			},
		},
		{
			title:     "ephemeral container not ready",
			namespace: "test-ns",
			pod: builders.NewPodBuilder("pod1").
				WithNamespace("test-ns").
				WithIP("192.0.2.6").
				Build(),
			visitCmds: visitCommands(),
			err:       nil,
			options: PodAgentVisitorOptions{
				Timeout: 1,
			},
			expectError: true,
			expected:    nil,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			client := fake.NewSimpleClientset(&tc.pod)
			executor := helpers.NewFakePodCommandExecutor()
			helper := helpers.NewPodHelper(client, executor, tc.namespace)
			visitor := NewPodAgentVisitor(
				helper,
				tc.namespace,
				tc.options,
			)

			executor.SetResult(tc.stdout, tc.stderr, tc.err)
			err := visitor.Visit(context.TODO(), tc.pod, tc.visitCmds)
			if tc.expectError && err == nil {
				t.Fatalf("should had failed")
			}

			if tc.expectError && err != nil {
				// error expected
				return
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
