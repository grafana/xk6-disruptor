//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	"github.com/grafana/xk6-disruptor/pkg/disruptors"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/checks"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/cluster"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/deploy"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fixtures"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/kubernetes/namespace"
)

func Test_PodDisruptor(t *testing.T) {
	t.Parallel()

	cluster, err := cluster.BuildE2eCluster(
		cluster.DefaultE2eClusterConfig(),
	)
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
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

	t.Run("Test fault injection", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			title    string
			pod      corev1.Pod
			service  corev1.Service
			port     int
			injector func(d disruptors.PodDisruptor) error
			check    checks.Check
		}{
			{
				title:   "Inject Http error 500",
				pod:     fixtures.BuildHttpbinPod(),
				service: fixtures.BuildHttpbinService(),
				port:    80,
				injector: func(d disruptors.PodDisruptor) error {
					fault := disruptors.HTTPFault{
						Port:      80,
						ErrorRate: 1.0,
						ErrorCode: 500,
					}
					options := disruptors.HTTPDisruptionOptions{
						ProxyPort: 8080,
					}

					return d.InjectHTTPFaults(context.TODO(), fault, 10*time.Second, options)
				},
				check: checks.HTTPCheck{
					Service:      "httpbin",
					Method:       "GET",
					Path:         "/status/200",
					Body:         []byte{},
					ExpectedCode: 500,
					Delay:        5 * time.Second,
				},
			},
			{
				title:   "Inject Grpc error",
				pod:     fixtures.BuildGrpcpbinPod(),
				service: fixtures.BuildGrpcbinService(),
				port:    9000,
				injector: func(d disruptors.PodDisruptor) error {
					fault := disruptors.GrpcFault{
						Port:       9000,
						ErrorRate:  1.0,
						StatusCode: 14,
						Exclude:    "grpc.reflection.v1alpha.ServerReflection,grpc.reflection.v1.ServerReflection",
					}
					options := disruptors.GrpcDisruptionOptions{
						ProxyPort: 3000,
					}

					return d.InjectGrpcFaults(context.TODO(), fault, 10*time.Second, options)
				},
				check: checks.GrpcCheck{
					Service:        "grpcbin",
					GrpcService:    "grpcbin.GRPCBin",
					Method:         "Empty",
					Request:        []byte("{}"),
					ExpectedStatus: 14,
					Delay:          5 * time.Second,
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
					tc.service,
					intstr.FromInt(tc.port),
					30*time.Second,
				)
				if err != nil {
					t.Errorf("error deploying application: %v", err)
					return
				}

				// create pod disruptor that will select the service's pods
				selector := disruptors.PodSelector{
					Namespace: namespace,
					Select: disruptors.PodAttributes{
						Labels: tc.service.Spec.Selector,
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
					err := tc.injector(disruptor)
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

	t.Run("Disruptor errors out if no requests are received", func(t *testing.T) {
		t.Parallel()

		namespace, err := namespace.CreateTestNamespace(context.TODO(), t, k8s.Client())
		if err != nil {
			t.Fatalf("failed to create test namespace: %v", err)
		}

		service := fixtures.BuildHttpbinService()

		err = deploy.ExposeApp(
			k8s,
			namespace,
			fixtures.BuildHttpbinPod(),
			service,
			intstr.FromInt(80),
			30*time.Second,
		)
		if err != nil {
			t.Fatalf("error deploying application: %v", err)
		}

		// create pod disruptor that will select the service's pods
		selector := disruptors.PodSelector{
			Namespace: namespace,
			Select: disruptors.PodAttributes{
				Labels: service.Spec.Selector,
			},
		}
		options := disruptors.PodDisruptorOptions{}
		disruptor, err := disruptors.NewPodDisruptor(context.TODO(), k8s, selector, options)
		if err != nil {
			t.Fatalf("error creating selector: %v", err)
		}

		targets, _ := disruptor.Targets(context.TODO())
		if len(targets) == 0 {
			t.Fatalf("No pods matched the selector")
		}

		fault := disruptors.HTTPFault{
			Port:      80,
			ErrorRate: 1.0,
			ErrorCode: 500,
		}
		disruptorOptions := disruptors.HTTPDisruptionOptions{
			ProxyPort: 8080,
		}
		err = disruptor.InjectHTTPFaults(context.TODO(), fault, 5*time.Second, disruptorOptions)
		if err == nil {
			t.Fatalf("disruptor did not return an error")
		}

		// It is not possible to use errors.Is here, as ErrNoRequests is returned inside the agent pod. The controller
		// only sees the error message printed to stderr.
		if !strings.Contains(err.Error(), protocol.ErrNoRequests.Error()) {
			t.Fatalf("expected ErrNoRequests, got: %v", err)
		}
	})
}
