package disruptors

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/grafana/xk6-disruptor/pkg/testutils/command"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"

	corev1 "k8s.io/api/core/v1"
	kruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

type fakeAgentController struct {
	namespace string
	targets   []string
	executor  *runtime.FakeExecutor
}

func (f *fakeAgentController) Targets(ctx context.Context) ([]string, error) {
	return f.targets, nil
}

func (f *fakeAgentController) InjectDisruptorAgent(ctx context.Context) error {
	return nil
}

func (f *fakeAgentController) ExecCommand(ctx context.Context, cmd []string) error {
	_, err := f.executor.Exec(cmd[0], cmd[1:]...)
	return err
}

func (f *fakeAgentController) Visit(ctx context.Context, visitor func(string) []string) error {
	for _, t := range f.targets {
		cmd := visitor(t)
		_, err := f.executor.Exec(cmd[0], cmd[1:]...)
		if err != nil {
			return err
		}
	}
	return nil
}

func newPodDisruptorForTesting(controller AgentController, podHelper helpers.PodHelper, podSelector PodSelector) PodDisruptor {
	return &podDisruptor{
		controller: controller,
		podFilter: helpers.PodFilter{
			Select:  podSelector.Select.Labels,
			Exclude: podSelector.Exclude.Labels,
		},
		podHelper: podHelper,
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
		duration    time.Duration
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
			duration: 60 * time.Second,
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
			duration: 60 * time.Second,
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
			expectedCmd: "xk6-disruptor-agent http -d 60s -a 100ms -v 0ms",
			expectError: false,
			cmdError:    nil,
			fault: HTTPFault{
				AverageDelay: 100 * time.Millisecond,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60 * time.Second,
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
			duration: 60 * time.Second,
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
			expectedCmd: "xk6-disruptor-agent http -d 60s",
			expectError: true,
			cmdError:    fmt.Errorf("error executing command"),
			fault:       HTTPFault{},
			opts:        HTTPDisruptionOptions{},
			duration:    60 * time.Second,
		},
		{
			title: "Container port not found",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets:     []string{"my-app-pod"},
			expectError: true,
			fault:       HTTPFault{Port: 8080},
			opts:        HTTPDisruptionOptions{},
			duration:    60,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			executor := runtime.NewFakeExecutor([]byte{}, tc.cmdError)

			controller := &fakeAgentController{
				namespace: tc.selector.Namespace,
				targets:   tc.targets,
				executor:  executor,
			}

			objs := []kruntime.Object{}

			for _, target := range tc.targets {
				obj := builders.NewPodBuilder(target).
					WithLabels(tc.selector.Select.Labels).
					WithNamespace(tc.selector.Namespace).
					WithContainer(defaultContainer()).
					Build()
				objs = append(objs, obj)
			}

			client := fake.NewSimpleClientset(objs...)
			k, _ := kubernetes.NewFakeKubernetes(client)

			d := newPodDisruptorForTesting(controller, k.PodHelper(tc.selector.Namespace), tc.selector)

			err := d.InjectHTTPFaults(context.TODO(), tc.fault, tc.duration, tc.opts)

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

func Test_PodGrpcPFaultInjection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		selector    PodSelector
		targets     []string
		fault       GrpcFault
		opts        GrpcDisruptionOptions
		duration    time.Duration
		expectedCmd string
		expectError bool
		cmdError    error
	}{
		{
			title: "Test error",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets: []string{"my-app-pod"},

			fault: GrpcFault{
				ErrorRate:  0.1,
				StatusCode: 14,
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -r 0.1 -s 14",
			expectError: false,
			cmdError:    nil,
		},
		{
			title: "Test error with status message",
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			targets: []string{"my-app-pod"},
			fault: GrpcFault{
				ErrorRate:     0.1,
				StatusCode:    14,
				StatusMessage: "internal error",
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -r 0.1 -s 14 -m internal error",
			expectError: false,
			cmdError:    nil,
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
			targets: []string{"my-app-pod"},
			fault: GrpcFault{
				AverageDelay: 100 * time.Millisecond,
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -a 100ms -v 0ms",
			expectError: false,
			cmdError:    nil,
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
			targets: []string{"my-app-pod"},
			fault: GrpcFault{
				Exclude: "service1,service2",
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -x service1,service2",
			expectError: false,
			cmdError:    nil,
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
			fault:       GrpcFault{},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s",
			expectError: true,
			cmdError:    fmt.Errorf("error executing command"),
		},
		{
			title:   "Container port not found",
			targets: []string{"my-app-pod"},
			selector: PodSelector{
				Namespace: "testns",
				Select: PodAttributes{
					Labels: map[string]string{
						"app": "myapp",
					},
				},
			},
			expectError: true,
			fault:       GrpcFault{Port: 8080},
			opts:        GrpcDisruptionOptions{},
			duration:    60,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			executor := runtime.NewFakeExecutor([]byte{}, tc.cmdError)

			controller := &fakeAgentController{
				namespace: tc.selector.Namespace,
				targets:   tc.targets,
				executor:  executor,
			}

			objs := []kruntime.Object{}

			for _, target := range tc.targets {
				obj := builders.NewPodBuilder(target).
					WithLabels(tc.selector.Select.Labels).
					WithNamespace(tc.selector.Namespace).
					WithContainer(defaultContainer()).
					Build()
				objs = append(objs, obj)
			}

			client := fake.NewSimpleClientset(objs...)
			k, _ := kubernetes.NewFakeKubernetes(client)

			d := newPodDisruptorForTesting(controller, k.PodHelper(tc.selector.Namespace), tc.selector)

			err := d.InjectGrpcFaults(context.TODO(), tc.fault, tc.duration, tc.opts)

			if tc.expectError && err != nil {
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			cmd := executor.Cmd()
			if !command.AssertCmdEquals(tc.expectedCmd, cmd) {
				t.Errorf("expected command: %s got: %s", tc.expectedCmd, cmd)
			}
		})
	}
}

func defaultContainer() corev1.Container {
	return corev1.Container{
		Name:    "busybox",
		Image:   "busybox",
		Command: []string{"sh", "-c", "sleep 300"},
		Ports: []corev1.ContainerPort{
			{
				ContainerPort: 80,
			},
		},
	}
}
