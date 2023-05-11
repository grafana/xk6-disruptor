//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/checks"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fixtures"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

var injectHTTP500 = []string{
	"xk6-disruptor-agent",
	"http",
	"--duration",
	"300s",
	"--rate",
	"1.0",
	"--error",
	"500",
	"--port",
	"8080",
	"--target",
	"80",
}

var injectGrpcInternal = []string{
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
	"-x",
	// exclude reflection service otherwise the dynamic client will not work
	"grpc.reflection.v1alpha.ServerReflection,grpc.reflection.v1.ServerReflection",
}

// deploy pod with [httpbin] and the xk6-disruptor as sidekick container
func buildHttpbinPodWithDisruptorAgent(cmd []string) *corev1.Pod {
	httpbin := builders.NewContainerBuilder("httpbin").
		WithImage("kennethreitz/httpbin").
		WithPort("http", 80).
		Build()

	agent := builders.NewContainerBuilder("xk6-disruptor-agent").
		WithImage("ghcr.io/grafana/xk6-disruptor-agent").
		WithCommand(cmd...).
		WithCapabilities("NET_ADMIN").
		Build()

	return builders.NewPodBuilder("httpbin").
		WithLabels(
			map[string]string{
				"app": "httpbin",
			},
		).
		WithContainer(*httpbin).
		WithContainer(*agent).
		Build()
}

// deploy pod with grpcbin and the xk6-disruptor as sidekick container
func buildGrpcbinPodWithDisruptorAgent(cmd []string) *corev1.Pod {
	grpcbin := builders.NewContainerBuilder("grpcbin").
		WithImage("moul/grpcbin").
		WithPort("grpc", 9000).
		Build()

	agent := builders.NewContainerBuilder("xk6-disruptor-agent").
		WithImage("ghcr.io/grafana/xk6-disruptor-agent").
		WithCommand(cmd...).
		WithCapabilities("NET_ADMIN").
		Build()

	return builders.NewPodBuilder("grpcbin").
		WithLabels(
			map[string]string{
				"app": "grpcbin",
			},
		).
		WithContainer(*grpcbin).
		WithContainer(*agent).
		Build()
}

func Test_Agent(t *testing.T) {
	cluster, err := fixtures.BuildE2eCluster(
		fixtures.DefaultE2eClusterConfig(),
		fixtures.WithName("e2e-xk6-agent"),
		fixtures.WithIngressPort(30080),
	)
	if err != nil {
		t.Errorf("failed to create e2e cluster: %v", err)
		return
	}

	t.Cleanup(func() {
		_ = cluster.Delete()
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
			pod   *corev1.Pod
			svc   *corev1.Service
			port  int
			check checks.Check
		}{
			{
				title: "Inject HTTP 500",
				pod:   buildHttpbinPodWithDisruptorAgent(injectHTTP500),
				svc:   fixtures.BuildHttpbinService(),
				port:  80,
				check: checks.HTTPCheck{
					Service:      "httpbin",
					Port:         80,
					Path:         "/status/200",
					ExpectedCode: 500,
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
				namespace, err := k8s.NamespaceHelper().CreateRandomNamespace(context.TODO(), "test-")
				if err != nil {
					t.Errorf("error creating test namespace: %v", err)
					return
				}
				defer k8s.Client().CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})

				err = fixtures.DeployApp(
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

				err = tc.check.Verify(k8s, cluster.Ingress(), namespace)
				if err != nil {
					t.Errorf("failed : %v", err)
					return
				}
			})
		}
	})

	t.Run("Prevent execution of multiple commands", func(t *testing.T) {
		t.Parallel()

		namespace, err := k8s.NamespaceHelper().CreateRandomNamespace(context.TODO(), "test-")
		if err != nil {
			t.Errorf("error creating test namespace: %v", err)
			return
		}
		defer k8s.Client().CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})

		err = fixtures.RunPod(
			k8s,
			namespace,
			buildHttpbinPodWithDisruptorAgent(injectHTTP500),
			30*time.Second,
		)
		if err != nil {
			t.Errorf("failed to create pod: %v", err)
			return
		}
		_, stderr, err := k8s.PodHelper(namespace).Exec(
			"httpbin",
			"xk6-disruptor-agent",
			injectHTTP500,
			[]byte{},
		)
		if err == nil {
			t.Errorf("command should had failed")
		}

		if !strings.Contains(string(stderr), "command is already in execution") {
			t.Errorf("unexpected error: %s: ", string(stderr))
		}
	})
}
