package disruptors

import (
	"context"
	"fmt"
	"testing"

	"github.com/grafana/xk6-disruptor/pkg/testutils/command"
	"github.com/grafana/xk6-disruptor/pkg/utils/process"
)

type fakeAgentController struct {
	namespace string
	targets   []string
	executor  *process.FakeExecutor
}

func (f *fakeAgentController) Targets() ([]string, error) {
	return f.targets, nil
}

func (f *fakeAgentController) InjectDisruptorAgent() error {
	return nil
}

func (f *fakeAgentController) ExecCommand(cmd ...string) error {
	_, err := f.executor.Exec(cmd[0], cmd[1:]...)
	return err
}

func newPodDisruptorForTesting(ctx context.Context, selector PodSelector, controller AgentController) PodDisruptor {
	return &podDisruptor{
		ctx:        ctx,
		selector:   selector,
		controller: controller,
	}
}

func Test_PodHTTPFaultInjection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		selector    PodSelector
		targets     []string
		expectedCmd string
		expectError bool
		cmdError    error
		fault       HTTPFault
		opts        HTTPDisruptionOptions
		duration    uint
	}{
		{
			title: "Test error 500",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:     []string{"my-app-pod"},
			expectedCmd: "xk6-disruptor-agent http -d 60s -r 0.1 -e 500",
			expectError: false,
			cmdError:    nil,
			fault: HTTPFault{
				ErrorRate: 0.1,
				ErrorCode: 500,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
		{
			title: "Test error 500 with error body",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:     []string{"my-app-pod"},
			expectedCmd: "xk6-disruptor-agent http -d 60s -r 0.1 -e 500 -b {\"error\": 500}",
			expectError: false,
			cmdError:    nil,
			fault: HTTPFault{
				ErrorRate: 0.1,
				ErrorCode: 500,
				ErrorBody: "{\"error\": 500}",
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
		{
			title: "Test Average delay",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:     []string{"my-app-pod"},
			expectedCmd: "xk6-disruptor-agent http -d 60s -a 100 -v 0",
			expectError: false,
			cmdError:    nil,
			fault: HTTPFault{
				AverageDelay: 100,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
		{
			title: "Test exclude list",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:     []string{"my-app-pod"},
			expectedCmd: "xk6-disruptor-agent http -d 60s -x /path1,/path2",
			expectError: false,
			cmdError:    nil,
			fault: HTTPFault{
				Exclude: "/path1,/path2",
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
		{
			title: "Test command execution fault",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:     []string{"my-app-pod"},
			expectedCmd: "xk6-disruptor-agent http -d 60s -x /path1,/path2",
			expectError: true,
			cmdError:    fmt.Errorf("error executing command"),
			fault:       HTTPFault{},
			opts:        HTTPDisruptionOptions{},
			duration:    60,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			executor := process.NewFakeExecutor([]byte{}, tc.cmdError)

			controller := &fakeAgentController{
				namespace: tc.selector.Namespace,
				targets:   tc.targets,
				executor:  executor,
			}

			d := newPodDisruptorForTesting(context.TODO(), tc.selector, controller)

			err := d.InjectHTTPFaults(tc.fault, tc.duration, tc.opts)

			if tc.expectError && err != nil {
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error : %v", err)
				return
			}

			cmd := executor.Cmd()
			if !command.AssertCmdEquals(tc.expectedCmd, cmd) {
				t.Errorf("expected command: %s got: %s", tc.expectedCmd, cmd)
			}
		})
	}
}
