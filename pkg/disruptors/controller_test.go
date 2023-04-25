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

func Test_InjectAgent(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title   string
		targets []string
		// Set timeout to -1 to prevent waiting the ephemeral container to be ready,
		// as the fake client will not update its status
		timeout     time.Duration
		expectError bool
	}{
		{
			title:       "Inject ephemeral container",
			targets:     []string{"pod1", "pod2"},
			timeout:     -1,
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
			helper := k8s.PodHelper(testNamespace)
			controller := NewAgentController(context.TODO(), helper, testNamespace, tc.targets, tc.timeout)

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
			helper := k8s.PodHelper(testNamespace)
			controller := NewAgentController(context.TODO(), helper, testNamespace, tc.targets, tc.timeout)

			err := controller.ExecCommand(tc.command)
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
