//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"testing"
	"time"

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

func Test_ServiceDisruptor(t *testing.T) {
	cluster, err := cluster.BuildE2eCluster(
		t,
		cluster.DefaultE2eClusterConfig(),
		cluster.WithName("e2e-service-disruptor"),
		cluster.WithIngressPort(30083),
	)
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}

	k8s, err := kubernetes.NewFromKubeconfig(cluster.Kubeconfig())
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	t.Run("Test fault injection", func(t *testing.T) {
		t.Parallel()

		testCases := []struct {
			title    string
			pod      *corev1.Pod
			service  *corev1.Service
			port     int
			injector func(d disruptors.ServiceDisruptor) error
			check    checks.Check
		}{
			{
				title:    "Inject Http error 500",
				pod:      fixtures.BuildHttpbinPod(),
				service:  fixtures.BuildHttpbinService(),
				port:     80,
				injector: func(d disruptors.ServiceDisruptor) error {
					fault := disruptors.HTTPFault{
						Port:      80,
						ErrorRate: 1.0,
						ErrorCode: 500,
					}
					httpOptions := disruptors.HTTPDisruptionOptions{}
					return d.InjectHTTPFaults(context.TODO(), fault, 10*time.Second, httpOptions)
				},
				check: checks.HTTPCheck{
					Service:      "httpbin",
					Port:         80,
					Method:       "GET",
					Path:         "/status/200",
					Body:         []byte{},
					ExpectedCode: 500,
					Delay:        5 * time.Second,
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

				options := disruptors.ServiceDisruptorOptions{}
				disruptor, err := disruptors.NewServiceDisruptor(
					context.TODO(),
					k8s, 
					tc.service.Name,
					namespace,
					options,
				)
				if err != nil {
					t.Errorf("error creating disruptor: %v", err)
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
						t.Logf("failed to inject fault: %v", err)
						return
					}
				}()

				err = tc.check.Verify(k8s, cluster.Ingress(), namespace)
				if err != nil {
					t.Errorf("failed: %v", err)
					return
				}
			})
		}
	})
}
