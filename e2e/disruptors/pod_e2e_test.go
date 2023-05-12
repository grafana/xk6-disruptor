//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/grafana/xk6-disruptor/pkg/disruptors"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/checks"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fixtures"
)

func Test_PodDisruptor(t *testing.T) {
	t.Parallel()

	cluster, err := fixtures.BuildE2eCluster(
		fixtures.DefaultE2eClusterConfig(),
		fixtures.WithName("e2e-pod-disruptor"),
		fixtures.WithIngressPort(30082),
	)
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
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

	t.Run("Test Http fault injection", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			title   string
			fault   disruptors.HTTPFault
			options disruptors.HTTPDisruptionOptions
			check   checks.Check
		}{
			{
				title: "Inject Http error 500",
				fault: disruptors.HTTPFault{
					Port:      80,
					ErrorRate: 1.0,
					ErrorCode: 500,
				},
				options: disruptors.HTTPDisruptionOptions{
					ProxyPort: 8080,
				},
				check: checks.HTTPCheck{
					Method:       "GET",
					Path:         "/status/200",
					Body:         []byte{},
					ExpectedCode: 500,
				},
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.title, func(t *testing.T) {
				t.Parallel()

				namespace, err := k8s.NamespaceHelper().CreateRandomNamespace(context.TODO(), "test-pods")
				if err != nil {
					t.Errorf("error creating test namespace: %v", err)
					return
				}
				defer k8s.Client().CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})

				err = fixtures.DeployApp(
					k8s,
					namespace,
					fixtures.BuildHttpbinPod(),
					fixtures.BuildHttpbinService(),
					intstr.FromInt(80),
					30*time.Second,
				)
				if err != nil {
					t.Errorf("error deploying httpbin: %v", err)
					return
				}

				// create pod disruptor
				selector := disruptors.PodSelector{
					Namespace: namespace,
					Select: disruptors.PodAttributes{
						Labels: map[string]string{
							"app": "httpbin",
						},
					},
				}
				options := disruptors.PodDisruptorOptions{}
				disruptor, err := disruptors.NewPodDisruptor(context.TODO(), k8s, selector, options)
				if err != nil {
					t.Errorf("error creating selector: %v", err)
					return
				}

				targets, _ := disruptor.Targets(context.TODO())
				if len(targets) == 0 {
					t.Errorf("No pods matched the selector")
					return
				}

				// apply disruption in a go-routine as it is a blocking function
				go func() {
					err := disruptor.InjectHTTPFaults(context.TODO(), tc.fault, 60*time.Second, tc.options)
					if err != nil {
						t.Logf("failed to setup disruptor: %v", err)
						return
					}
				}()

				err = tc.check.Verify(k8s, cluster.Ingress(), namespace)
				if err != nil {
					t.Errorf("failed to access service: %v", err)
					return
				}
			})
		}
	})

	// Test Fault injection in Grpc service.
	// We must use a NodePort to access the service, so to prevent port collision between
	// tests, we will deploy one service to expose the pods created for each
	// test. Tests cannot be therefore executed in parallel.
	t.Run("Test Grpc fault injection", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			title   string
			fault   disruptors.GrpcFault
			options disruptors.GrpcDisruptionOptions
			host    string
			port    int
			check   checks.Check
		}{
			{
				title: "Inject Grpc error",
				fault: disruptors.GrpcFault{
					Port:       9000,
					ErrorRate:  1.0,
					StatusCode: 14,
					Exclude:    "grpc.reflection.v1alpha.ServerReflection,grpc.reflection.v1.ServerReflection",
				},
				options: disruptors.GrpcDisruptionOptions{
					ProxyPort: 3000,
				},
				check: checks.GrpcCheck{
					Service:        "grpcbin.GRPCBin",
					Method:         "Empty",
					Request:        []byte("{}"),
					ExpectedStatus: 14,
					Delay:          10 * time.Second,
				},
			},
		}

		for _, tc := range testCases {
			tc := tc
			t.Run(tc.title, func(t *testing.T) {
				namespace, err := k8s.NamespaceHelper().CreateRandomNamespace(context.TODO(), "test-pods")
				if err != nil {
					t.Errorf("error creating test namespace: %v", err)
					return
				}
				defer k8s.Client().CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})

				err = fixtures.DeployApp(
					k8s,
					namespace,
					fixtures.BuildGrpcpbinPod(),
					fixtures.BuildGrpcbinService(),
					intstr.FromInt(9000),
					30*time.Second,
				)
				if err != nil {
					t.Errorf("error deploying grpcbin: %v", err)
					return
				}

				// create pod disruptor
				selector := disruptors.PodSelector{
					Namespace: namespace,
					Select: disruptors.PodAttributes{
						Labels: map[string]string{
							"app": "grpcbin",
						},
					},
				}
				options := disruptors.PodDisruptorOptions{}
				disruptor, err := disruptors.NewPodDisruptor(context.TODO(), k8s, selector, options)
				if err != nil {
					t.Errorf("error creating selector: %v", err)
					return
				}

				targets, _ := disruptor.Targets(context.TODO())
				if len(targets) == 0 {
					t.Errorf("No pods matched the selector")
					return
				}

				// apply disruption in a go-routine as it is a blocking function
				go func() {
					err := disruptor.InjectGrpcFaults(context.TODO(), tc.fault, 60*time.Second, tc.options)
					if err != nil {
						t.Logf("failed to setup disruptor: %v", err)
						return
					}
				}()

				err = tc.check.Verify(k8s, cluster.Ingress(), namespace)
				if err != nil {
					t.Errorf("failed to access service: %v", err)
					return
				}
			})
		}
	})
}
