package disruptors

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/grafana/xk6-disruptor/pkg/testutils/command"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/fake"
)

type fakeAgentController struct {
	namespace string
	targets   []corev1.Pod
	executor  *runtime.FakeExecutor
}

func (f *fakeAgentController) Targets(_ context.Context) ([]string, error) {
	names := []string{}
	for _, p := range f.targets {
		names = append(names, p.Name)
	}
	return names, nil
}

func (f *fakeAgentController) InjectDisruptorAgent(_ context.Context) error {
	return nil
}

func (f *fakeAgentController) ExecCommand(_ context.Context, cmd []string) error {
	_, err := f.executor.Exec(cmd[0], cmd[1:]...)
	return err
}

func (f *fakeAgentController) Visit(_ context.Context, visitor func(corev1.Pod) ([]string, error)) error {
	for _, t := range f.targets {
		cmd, err := visitor(t)
		if err != nil {
			return err
		}
		_, err = f.executor.Exec(cmd[0], cmd[1:]...)
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

func buildPodWithPort(name string, portName string, port int32) *corev1.Pod {
	container := builders.NewContainerBuilder(name).
		WithPort(portName, port).
		Build()

	pod := builders.NewPodBuilder(name).
		WithNamespace("test-ns").
		WithContainer(*container).
		WithIP("192.0.2.6").
		Build()

	return pod
}

// podSelectorForPod returns a PodSelector that matches the given Pod
func podSelectorForPod(pod *corev1.Pod) PodSelector {
	return PodSelector{
		Namespace: pod.Namespace,
		Select: PodAttributes{
			Labels: pod.Labels,
		},
	}
}

type httpFaultTestCase struct {
	title       string
	target      *corev1.Pod
	expectedCmd string
	expectError bool
	fault       HTTPFault
	opts        HTTPDisruptionOptions
	duration    time.Duration
}

func httpFaultTestCases() []httpFaultTestCase {
	return []httpFaultTestCase{
		{
			title:  "Test error 500",
			target: buildPodWithPort("my-app-pod", "http", 80),
			fault: HTTPFault{
				ErrorRate: 0.1,
				ErrorCode: 500,
				Port:      80,
			},
			opts:        HTTPDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent http -d 60s -t 80 -r 0.1 -e 500 --upstream-host 192.0.2.6",
			expectError: false,
		},
		{
			title:  "Test error 500 with error body",
			target: buildPodWithPort("my-app-pod", "http", 80),
			// TODO: Make expectedCmd better represent the actual result ([]string), as it currently looks like we
			// are asserting a broken behavior (e.g. lack of quotes in -b) which is not the case.
			expectedCmd: "xk6-disruptor-agent http -d 60s -t 80 -r 0.1 -e 500 -b {\"error\": 500} --upstream-host 192.0.2.6",
			expectError: false,
			fault: HTTPFault{
				ErrorRate: 0.1,
				ErrorCode: 500,
				ErrorBody: "{\"error\": 500}",
				Port:      80,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60 * time.Second,
		},
		{
			title:       "Test Average delay",
			target:      buildPodWithPort("my-app-pod", "http", 80),
			expectedCmd: "xk6-disruptor-agent http -d 60s -t 80 -a 100ms -v 0ms --upstream-host 192.0.2.6",
			expectError: false,
			fault: HTTPFault{
				AverageDelay: 100 * time.Millisecond,
				Port:         80,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60 * time.Second,
		},
		{
			title:       "Test exclude list",
			target:      buildPodWithPort("my-app-pod", "http", 80),
			expectedCmd: "xk6-disruptor-agent http -d 60s -t 80 -x /path1,/path2 --upstream-host 192.0.2.6",
			expectError: false,
			fault: HTTPFault{
				Exclude: "/path1,/path2",
				Port:    80,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60 * time.Second,
		},
		{
			title:       "Container port not found",
			target:      buildPodWithPort("my-app-pod", "http", 80),
			expectError: true,
			fault: HTTPFault{
				Port: 8080,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
		{
			title: "Pod with hostNetwork",
			target: builders.NewPodBuilder("hostnet").
				WithNamespace("test-ns").
				WithLabels(map[string]string{
					"app": "myapp",
				}).
				WithHostNetwork(true).
				WithIP("192.0.2.6").
				WithContainer(
					*builders.NewContainerBuilder("myapp").
						WithPort("http", 80).
						Build(),
				).
				Build(),
			expectError: true,
			fault: HTTPFault{
				Port: 80,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
	}
}

type grpcFaultTestCase struct {
	title       string
	target      *corev1.Pod
	fault       GrpcFault
	opts        GrpcDisruptionOptions
	duration    time.Duration
	expectedCmd string
	expectError bool
}

func grpcFaultTestCases() []grpcFaultTestCase {
	return []grpcFaultTestCase{
		{
			title: "Test error",
			target: buildPodWithPort("my-app-pod", "grpc", 3000),
			fault: GrpcFault{
				ErrorRate:  0.1,
				StatusCode: 14,
				Port:       3000,
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -t 3000 -r 0.1 -s 14 --upstream-host 192.0.2.6",
			expectError: false,
		},
		{
			title: "Test error with status message",
			target: buildPodWithPort("my-app-pod", "grpc", 3000),
			fault: GrpcFault{
				ErrorRate:     0.1,
				StatusCode:    14,
				StatusMessage: "internal error",
				Port:          3000,
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -t 3000 -r 0.1 -s 14 -m internal error --upstream-host 192.0.2.6",
			expectError: false,
		},
		{
			title: "Test Average delay",
			target: buildPodWithPort("my-app-pod", "grpc", 3000),
			fault: GrpcFault{
				AverageDelay: 100 * time.Millisecond,
				Port:         3000,
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -t 3000 -a 100ms -v 0ms --upstream-host 192.0.2.6",
			expectError: false,
		},
		{
			title: "Test exclude list",
			target: buildPodWithPort("my-app-pod", "grpc", 3000),
			fault: GrpcFault{
				Exclude: "service1,service2",
				Port:    3000,
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -t 3000 -x service1,service2 --upstream-host 192.0.2.6",
			expectError: false,
		},
		{
			title:  "Container port not found",
			target: buildPodWithPort("my-app-pod", "grpc", 3000),
			expectError: true,
			fault:       GrpcFault{Port: 8080},
			opts:        GrpcDisruptionOptions{},
			duration:    60,
		},
	}
}

func Test_PodHTTPFaultInjection(t *testing.T) {
	t.Parallel()

	for _, tc := range httpFaultTestCases() {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			executor := runtime.NewFakeExecutor([]byte{}, nil)

			controller := &fakeAgentController{
				namespace: tc.target.Namespace,
				targets:   []corev1.Pod{*tc.target},
				executor:  executor,
			}

			client := fake.NewSimpleClientset(tc.target)
			k, _ := kubernetes.NewFakeKubernetes(client)

			selector := podSelectorForPod(tc.target)
			d := newPodDisruptorForTesting(
				controller,
				k.PodHelper(tc.target.Namespace),
				selector,
			)

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

	for _, tc := range grpcFaultTestCases() {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			executor := runtime.NewFakeExecutor([]byte{}, nil)

			controller := &fakeAgentController{
				namespace: tc.target.Namespace,
				targets:   []corev1.Pod{*tc.target},
				executor:  executor,
			}

			client := fake.NewSimpleClientset(tc.target)
			k, _ := kubernetes.NewFakeKubernetes(client)

			selector := podSelectorForPod(tc.target)
			d := newPodDisruptorForTesting(
				controller,
				k.PodHelper(tc.target.Namespace),
				selector,
			)

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
