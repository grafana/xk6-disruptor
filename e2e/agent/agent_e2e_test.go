//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/checks"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/cluster"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/deploy"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fixtures"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/kubectl"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/kubernetes/namespace"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var (
	injectHTTP418 = []string{
		"xk6-disruptor-agent",
		"http",
		"--duration",
		"300s",
		"--rate",
		"1.0",
		"--error",
		"418",
		"--port",
		"8080",
		"--target",
		"80",
		"--upstream-host",
		// POD_IP is injected in the container and take its value from status.podIP
		"$(POD_IP)",
	}

	injectNothing = []string{
		"xk6-disruptor-agent",
		"http",
		"--duration",
		"300s",
		"--port",
		"8080",
		"--target",
		"80",
		"--upstream-host",
		// POD_IP is injected in the container and take its value from status.podIP
		"$(POD_IP)",
	}

	injectGrpcInternal = []string{
		"xk6-disruptor-agent",
		"grpc",
		"--duration",
		"300s",
		"--rate",
		"1.0",
		"--status",
		"14",
		"--message",
		"Internal error",
		"--port",
		"4000",
		"--target",
		"9000",
		"--upstream-host",
		// POD_IP is injected in the container and take its value from status.podIP
		"$(POD_IP)",
		"-x",
		// exclude reflection service otherwise the dynamic client will not work
		"grpc.reflection.v1alpha.ServerReflection,grpc.reflection.v1.ServerReflection",
	}

	injectUpstreamHTTP500 = []string{
		"xk6-disruptor-agent",
		"http",
		"--duration",
		"300s",
		"--rate",
		"1.0",
		"--error",
		"500",
		"--port",
		"80",
		"--target",
		"80",
		"--transparent=false",
		"--upstream-host=httpbin.default.svc.cluster.local",
	}
)

// deploy pod with [httpbin] and the xk6-disruptor as sidekick container
func buildHttpbinPodWithDisruptorAgent(cmd []string) corev1.Pod {
	httpbin := builders.NewContainerBuilder("httpbin").
		WithImage("kennethreitz/httpbin").
		WithPort("http", 80).
		Build()

	agent := builders.NewContainerBuilder("xk6-disruptor-agent").
		WithImage("ghcr.io/grafana/xk6-disruptor-agent").
		WithEnvVarFromField("POD_IP", "status.podIP").
		WithCommand(cmd...).
		WithCapabilities("NET_ADMIN").
		Build()

	return builders.NewPodBuilder("httpbin").
		WithLabel("app", "httpbin").
		WithContainer(httpbin).
		WithContainer(agent).
		Build()
}

// deploy pod with grpcbin and the xk6-disruptor as sidekick container
func buildGrpcbinPodWithDisruptorAgent(cmd []string) corev1.Pod {
	grpcbin := builders.NewContainerBuilder("grpcbin").
		WithImage("moul/grpcbin").
		WithPort("grpc", 9000).
		Build()

	agent := builders.NewContainerBuilder("xk6-disruptor-agent").
		WithImage("ghcr.io/grafana/xk6-disruptor-agent").
		WithEnvVarFromField("POD_IP", "status.podIP").
		WithCommand(cmd...).
		WithCapabilities("NET_ADMIN").
		Build()

	return builders.NewPodBuilder("grpcbin").
		WithLabel("app", "grpcbin").
		WithContainer(grpcbin).
		WithContainer(agent).
		Build()
}

// deploy pod with the xk6-disruptor
func buildDisruptorAgentPod(cmd []string) corev1.Pod {
	agent := builders.NewContainerBuilder("xk6-disruptor-agent").
		WithImage("ghcr.io/grafana/xk6-disruptor-agent").
		WithPort("http", 80).
		WithCommand(cmd...).
		WithCapabilities("NET_ADMIN").
		Build()

	return builders.NewPodBuilder("xk6-disruptor").
		WithLabel("app", "xk6-disruptor").
		WithContainer(agent).
		Build()
}

// builDisruptorService returns a Service definition that exposes httpbin pods
func builDisruptorService() corev1.Service {
	return builders.NewServiceBuilder("xk6-disruptor").
		WithSelectorLabel("app", "xk6-disruptor").
		WithPort("http", 80, intstr.FromString("http")).
		Build()
}

func Test_Agent(t *testing.T) {
	t.Parallel()

	cluster, err := cluster.BuildE2eCluster(
		cluster.DefaultE2eClusterConfig(),
	)
	if err != nil {
		t.Errorf("failed to create e2e cluster: %v", err)
		return
	}
	t.Cleanup(func() {
		_ = cluster.Cleanup()
	})

	k8s, err := kubernetes.NewFromKubeconfig(cluster.Kubeconfig())
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	t.Run("Test Fault Injection", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			title string
			pod   corev1.Pod
			svc   corev1.Service
			port  int
			check checks.Check
		}{
			{
				title: "HTTP request is passed through",
				pod:   buildHttpbinPodWithDisruptorAgent(injectNothing),
				svc:   fixtures.BuildHttpbinService(),
				port:  80,
				check: checks.HTTPCheck{
					Service:      "httpbin",
					Port:         80,
					Path:         "/status/200",
					ExpectedCode: 200,
				},
			},
			{
				title: "Inject HTTP 500",
				pod:   buildHttpbinPodWithDisruptorAgent(injectHTTP418),
				svc:   fixtures.BuildHttpbinService(),
				port:  80,
				check: checks.HTTPCheck{
					Service:      "httpbin",
					Port:         80,
					Path:         "/status/200",
					ExpectedCode: 418,
				},
			},
			{
				title: "Inject Grpc Internal error",
				pod:   buildGrpcbinPodWithDisruptorAgent(injectGrpcInternal),
				svc:   fixtures.BuildGrpcbinService(),
				port:  9000,
				check: checks.GrpcCheck{
					Service:        "grpcbin",
					GrpcService:    "grpcbin.GRPCBin",
					Method:         "Empty",
					Request:        []byte("{}"),
					ExpectedStatus: 14, // grpc status Internal
				},
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.title, func(t *testing.T) {
				t.Parallel()

				namespace, err := namespace.CreateTestNamespace(context.TODO(), t, k8s.Client())
				if err != nil {
					t.Errorf("failed to create test namespace: %v", err)
					return
				}

				err = deploy.ExposeApp(
					k8s,
					namespace,
					tc.pod,
					tc.svc,
					intstr.FromInt(tc.port),
					30*time.Second,
				)
				if err != nil {
					t.Errorf("failed to deploy service: %v", err)
					return
				}

				t.Run("via ingress", func(t *testing.T) {
					t.Parallel()

					err := tc.check.Verify(k8s, cluster.Ingress(), namespace)
					if err != nil {
						t.Errorf("failed : %v", err)
						return
					}
				})

				t.Run("via port-forward", func(t *testing.T) {
					t.Parallel()

					ctx, cancel := context.WithCancel(context.Background())
					t.Cleanup(func() {
						cancel()
					})

					kc, err := kubectl.NewFromKubeconfig(ctx, cluster.Kubeconfig())
					if err != nil {
						t.Fatalf("creating kubectl client from kubeconfig: %v", err)
					}

					port, err := kc.ForwardPodPort(ctx, namespace, tc.pod.Name, uint(tc.port))
					if err != nil {
						t.Fatalf("forwarding port from %s/%s: %v", namespace, tc.pod.Name, err)
					}

					err = tc.check.Verify(k8s, net.JoinHostPort("localhost", fmt.Sprint(port)), namespace)
					if err != nil {
						t.Errorf("failed to access service: %v", err)
						return
					}
				})
			})
		}
	})

	t.Run("Prevent execution of multiple commands", func(t *testing.T) {
		t.Parallel()

		namespace, err := namespace.CreateTestNamespace(context.TODO(), t, k8s.Client())
		if err != nil {
			t.Errorf("failed to create test namespace: %v", err)
			return
		}

		err = deploy.RunPod(
			k8s,
			namespace,
			buildHttpbinPodWithDisruptorAgent(injectHTTP418),
			30*time.Second,
		)
		if err != nil {
			t.Errorf("failed to create pod: %v", err)
			return
		}
		_, stderr, err := k8s.PodHelper(namespace).Exec(
			context.TODO(),
			"httpbin",
			"xk6-disruptor-agent",
			injectHTTP418,
			[]byte{},
		)
		if err == nil {
			t.Errorf("command should had failed")
		}

		if !strings.Contains(string(stderr), "is already running") {
			t.Errorf("unexpected error: %s: ", string(stderr))
		}
	})

	t.Run("Non-transparent proxy to upstream service", func(t *testing.T) {
		t.Parallel()

		namespace, err := namespace.CreateTestNamespace(context.TODO(), t, k8s.Client())
		if err != nil {
			t.Errorf("failed to create test namespace: %v", err)
			return
		}

		err = deploy.ExposeApp(
			k8s,
			namespace,
			buildDisruptorAgentPod(injectUpstreamHTTP500),
			builDisruptorService(),
			intstr.FromInt(80),
			30*time.Second,
		)
		if err != nil {
			t.Errorf("failed to deploy service: %v", err)
			return
		}

		check := checks.HTTPCheck{
			Service:      "xk6-disruptor",
			Port:         80,
			Path:         "/status/200",
			ExpectedCode: 500,
		}

		err = check.Verify(k8s, cluster.Ingress(), namespace)
		if err != nil {
			t.Errorf("failed : %v", err)
			return
		}
	})
}
