package disruptors

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/grafana/xk6-disruptor/pkg/testutils/command"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
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

func (f *fakeAgentController) Visit(_ context.Context, visitor func(corev1.Pod) (VisitCommands, error)) error {
	for _, t := range f.targets {
		visitCommands, err := visitor(t)
		if err != nil {
			return err
		}

		cmd := visitCommands.Exec
		_, err = f.executor.Exec(cmd[0], cmd[1:]...)
		if err != nil && visitCommands.Cleanup != nil {
			cleanup := visitCommands.Cleanup
			_, _ = f.executor.Exec(cleanup[0], cleanup[1:]...)
			return err
		}
	}
	return nil
}

func buildPodWithPort(name string, portName string, port int32) corev1.Pod {
	container := builders.NewContainerBuilder(name).
		WithPort(portName, port).
		Build()

	pod := builders.NewPodBuilder(name).
		WithNamespace("test-ns").
		WithContainer(container).
		WithIP("192.0.2.6").
		Build()

	return pod
}

func Test_PodHTTPFaultInjection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title        string
		target       corev1.Pod
		expectedCmds []string
		expectError  bool
		cmdError     error
		fault        HTTPFault
		opts         HTTPDisruptionOptions
		duration     time.Duration
	}{
		{
			title:  "Test error 500",
			target: buildPodWithPort("my-app-pod", "http", 80),
			fault: HTTPFault{
				ErrorRate: 0.1,
				ErrorCode: 500,
				Port:      80,
			},
			opts:         HTTPDisruptionOptions{},
			duration:     60 * time.Second,
			expectedCmds: []string{"xk6-disruptor-agent http -d 60s -t 80 -r 0.1 -e 500 --upstream-host 192.0.2.6"},
			expectError:  false,
			cmdError:     nil,
		},
		{
			title:  "Test error 500 with error body",
			target: buildPodWithPort("my-app-pod", "http", 80),
			// TODO: Make expectedCmd better represent the actual result ([]string), as it currently looks like we
			// are asserting a broken behavior (e.g. lack of quotes in -b) which is not the case.
			//nolint:lll
			expectedCmds: []string{"xk6-disruptor-agent http -d 60s -t 80 -r 0.1 -e 500 -b {\"error\": 500} --upstream-host 192.0.2.6"},
			expectError:  false,
			cmdError:     nil,
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
			title:        "Test Average delay",
			target:       buildPodWithPort("my-app-pod", "http", 80),
			expectedCmds: []string{"xk6-disruptor-agent http -d 60s -t 80 -a 100ms -v 0ms --upstream-host 192.0.2.6"},
			expectError:  false,
			cmdError:     nil,
			fault: HTTPFault{
				AverageDelay: 100 * time.Millisecond,
				Port:         80,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60 * time.Second,
		},
		{
			title:        "Test exclude list",
			target:       buildPodWithPort("my-app-pod", "http", 80),
			expectedCmds: []string{"xk6-disruptor-agent http -d 60s -t 80 -x /path1,/path2 --upstream-host 192.0.2.6"},
			expectError:  false,
			cmdError:     nil,
			fault: HTTPFault{
				Exclude: "/path1,/path2",
				Port:    80,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60 * time.Second,
		},
		{
			title:  "Test command execution fault",
			target: buildPodWithPort("my-app-pod", "http", 80),
			expectedCmds: []string{
				"xk6-disruptor-agent http -d 60s -t 80 --upstream-host 192.0.2.6",
				"xk6-disruptor-agent cleanup",
			},
			expectError: true,
			cmdError:    fmt.Errorf("error executing command"),
			fault: HTTPFault{
				Port: 80,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60 * time.Second,
		},
		{
			title:        "Container port not found",
			target:       buildPodWithPort("my-app-pod", "http", 80),
			expectedCmds: []string{},
			expectError:  true,
			fault: HTTPFault{
				Port: 8080,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
		{
			title: "Pod without PodIP",
			target: builders.NewPodBuilder("noip").
				WithNamespace("test-ns").
				WithLabel("app", "myapp").
				WithContainer(
					builders.NewContainerBuilder("noip").
						WithPort("http", 80).
						Build(),
				).
				Build(),
			expectedCmds: []string{},
			expectError:  true,
			fault: HTTPFault{
				Port: 80,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
		{
			title: "Pod with hostNetwork",
			target: builders.NewPodBuilder("hostnet").
				WithNamespace("test-ns").
				WithLabel("app", "myapp").
				WithHostNetwork(true).
				WithIP("192.0.2.6").
				WithContainer(
					builders.NewContainerBuilder("myapp").
						WithPort("http", 80).
						Build(),
				).
				Build(),
			expectedCmds: []string{},
			expectError:  true,
			fault: HTTPFault{
				Port: 80,
			},
			opts:     HTTPDisruptionOptions{},
			duration: 60,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			executor := runtime.NewFakeExecutor([]byte{}, tc.cmdError)

			controller := &fakeAgentController{
				namespace: "test-ns",
				targets:   []corev1.Pod{tc.target},
				executor:  executor,
			}

			d := podDisruptor{
				controller: controller,
			}

			err := d.InjectHTTPFaults(context.TODO(), tc.fault, tc.duration, tc.opts)

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error : %v", err)
				return
			}

			history := executor.CmdHistory()
			if len(tc.expectedCmds) != len(history) {
				t.Errorf("expected command: %s got: %s", tc.expectedCmds, history)
				return
			}

			for i, cmd := range tc.expectedCmds {
				if !command.AssertCmdEquals(cmd, history[i]) {
					t.Errorf("expected command: %s got: %s", cmd, history[i])
				}
			}
		})
	}
}

func Test_PodGrpcPFaultInjection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		target      corev1.Pod
		fault       GrpcFault
		opts        GrpcDisruptionOptions
		duration    time.Duration
		expectedCmd string
		expectError bool
		cmdError    error
	}{
		{
			title:  "Test error",
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
			cmdError:    nil,
		},
		{
			title:  "Test error with status message",
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
			cmdError:    nil,
		},
		{
			title:  "Test Average delay",
			target: buildPodWithPort("my-app-pod", "grpc", 3000),
			fault: GrpcFault{
				AverageDelay: 100 * time.Millisecond,
				Port:         3000,
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -t 3000 -a 100ms -v 0ms --upstream-host 192.0.2.6",
			expectError: false,
			cmdError:    nil,
		},
		{
			title:  "Test exclude list",
			target: buildPodWithPort("my-app-pod", "grpc", 3000),
			fault: GrpcFault{
				Exclude: "service1,service2",
				Port:    3000,
			},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -t 3000 -x service1,service2 --upstream-host 192.0.2.6",
			expectError: false,
			cmdError:    nil,
		},
		{
			title:       "Test command execution fault",
			target:      buildPodWithPort("my-app-pod", "grpc", 3000),
			fault:       GrpcFault{},
			opts:        GrpcDisruptionOptions{},
			duration:    60 * time.Second,
			expectedCmd: "xk6-disruptor-agent grpc -d 60s -t 3000 --upstream-host 192.0.2.6",
			expectError: true,
			cmdError:    fmt.Errorf("error executing command"),
		},
		{
			title:       "Container port not found",
			target:      buildPodWithPort("my-app-pod", "grpc", 3000),
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
				namespace: "test-ns",
				targets:   []corev1.Pod{tc.target},
				executor:  executor,
			}

			d := podDisruptor{controller: controller}

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

func Test_NewPodDisruptor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		name        string
		namespace   string
		pods        []corev1.Pod
		selector    PodSelector
		expectError bool
		expected    []string
	}{
		{
			title:     "matching pods",
			name:      "test-svc",
			namespace: "test-ns",
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-1").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					WithIP("192.0.2.6").
					Build(),
			},
			selector: PodSelector{
				Namespace: "test-ns",
				Select: PodAttributes{Labels: map[string]string{
					"app": "test",
				}},
			},
			expectError: false,
			expected:    []string{"pod-1"},
		},
		{
			title:     "no matching pods",
			name:      "test-svc",
			namespace: "test-ns",
			pods:      []corev1.Pod{},
			selector: PodSelector{
				Namespace: "test-ns",
				Select: PodAttributes{Labels: map[string]string{
					"app": "test",
				}},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			var objs []k8sruntime.Object
			for p := range tc.pods {
				objs = append(objs, &tc.pods[p])
			}

			client := fake.NewSimpleClientset(objs...)
			k, _ := kubernetes.NewFakeKubernetes(client)

			d, err := NewPodDisruptor(
				context.TODO(),
				k,
				tc.selector,
				PodDisruptorOptions{InjectTimeout: -1}, // Disable waiting for injected container to become Running.
			)

			if tc.expectError && err != nil {
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error creating pod disruptor: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed creating service disruptor")
				return
			}

			targets, _ := d.Targets(context.TODO())
			sort.Strings(targets)
			if diff := cmp.Diff(targets, tc.expected); diff != "" {
				t.Errorf("expected targets dot not match returned\n%s", diff)
				return
			}
		})
	}
}

func Test_PodSelectorString(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		selector PodSelector
		expected string
	}{
		{
			name:     "Empty selector",
			expected: `all pods in ns "default"`,
		},
		{
			name: "Only inclusions",
			selector: PodSelector{
				Namespace: "testns",
				Select:    PodAttributes{map[string]string{"foo": "bar"}},
			},
			expected: `pods including(foo=bar) in ns "testns"`,
		},
		{
			name: "Only exclusions",
			selector: PodSelector{
				Namespace: "testns",
				Exclude:   PodAttributes{map[string]string{"foo": "bar"}},
			},
			expected: `pods excluding(foo=bar) in ns "testns"`,
		},
		{
			name: "Both inclusions and exclusions",
			selector: PodSelector{
				Namespace: "testns",
				Select:    PodAttributes{map[string]string{"foo": "bar"}},
				Exclude:   PodAttributes{map[string]string{"boo": "baa"}},
			},
			expected: `pods including(foo=bar), excluding(boo=baa) in ns "testns"`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			output := tc.selector.String()
			if tc.expected != output {
				t.Errorf("expected string does not match output string:\n%s\n%s", tc.expected, output)
			}
		})
	}
}
