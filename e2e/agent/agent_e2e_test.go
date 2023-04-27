//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/cluster"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/checks"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fixtures"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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
func buildHttpbinPodWithDisruptorAgent(namespace string, cmd []string) *corev1.Pod {
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
		WithNamespace(namespace).
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
func buildGrpcbinPodWithDisruptorAgent(namespace string, cmd []string) *corev1.Pod {
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
		WithNamespace(namespace).
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
	// we need to access the grpc service using a nodeport because
	// we cannot use a service proxy as with http services
	grpcPort := cluster.NodePort{
		NodePort: 30000,
		HostPort: 30000,
	}
	cluster, err := fixtures.BuildCluster("e2e-xk6-agent", grpcPort)
	if err != nil {
		t.Errorf("failed to create cluster config: %v", err)
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

	t.Run("Test HTTP Fault Injection", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			// description of the test
			title string
			// command to pass to disruptor agent running in the target pod
			cmd []string
			// Function that checks the test conditions
			check func(k8s kubernetes.Kubernetes, ns string) error
		}{
			{
				title: "Inject HTTP 500",
				cmd:   injectHTTP500,
				check: func(k8s kubernetes.Kubernetes, ns string) error {
					err = fixtures.ExposeService(k8s, ns, fixtures.BuildHttpbinService(ns), 20*time.Second)
					if err != nil {
						return fmt.Errorf("failed to create service: %v", err)
					}
					return checks.CheckService(
						k8s,
						checks.ServiceCheck{
							Namespace:    ns,
							Service:      "httpbin",
							Port:         80,
							Path:         "/status/200",
							ExpectedCode: 500,
						},
					)
				},
			},
			{
				title: "Prevent execution of multiple commands",
				cmd:   injectHTTP500,
				check: func(k8s kubernetes.Kubernetes, ns string) error {
					_, stderr, err := k8s.PodHelper(ns).Exec(
						"httpbin",
						"xk6-disruptor-agent",
						injectHTTP500,
						[]byte{},
					)
					if err == nil {
						return fmt.Errorf("command should had failed")
					}

					if !strings.Contains(string(stderr), "command is already in execution") {
						return fmt.Errorf("unexpected error: %s: ", string(stderr))
					}
					return nil
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

				err = fixtures.RunPod(
					k8s,
					namespace,
					buildHttpbinPodWithDisruptorAgent(namespace, tc.cmd),
					30*time.Second,
				)
				if err != nil {
					t.Errorf("failed to create pod: %v", err)
					return
				}

				err = tc.check(k8s, namespace)
				if err != nil {
					t.Errorf("failed : %v", err)
					return
				}
			})
		}
	})

	t.Run("Test GRPC fault injection", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			// description of the test
			title string
			// command to pass to disruptor agent running in the target pod
			cmd []string
			// Function that checks the test conditions
			check func(k8s kubernetes.Kubernetes, ns string) error
		}{
			{
				title: "Inject Grpc Internal error",
				cmd:   injectGrpcInternal,
				check: func(k8s kubernetes.Kubernetes, ns string) error {
					err = fixtures.ExposeService(k8s,
						ns,
						fixtures.BuildGrpcbinService(ns, uint(grpcPort.NodePort)),
						20*time.Second,
					)
					if err != nil {
						return fmt.Errorf("failed to create service: %v", err)
					}
					return checks.CheckGrpcService(
						k8s,
						checks.GrpcServiceCheck{
							Host:           "localhost",
							Port:           int(grpcPort.HostPort),
							Service:        "grpcbin.GRPCBin",
							Method:         "Empty",
							Request:        []byte("{}"),
							ExpectedStatus: 14, // grpc status Internal
						},
					)
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

				err = fixtures.RunPod(
					k8s,
					namespace,
					buildGrpcbinPodWithDisruptorAgent(namespace, tc.cmd),
					30*time.Second,
				)
				if err != nil {
					t.Errorf("failed to create pod: %v", err)
					return
				}

				err = tc.check(k8s, namespace)
				if err != nil {
					t.Errorf("failed : %v", err)
					return
				}
			})
		}
	})
}
