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

func Test_ServiceDisruptor(t *testing.T) {
	cluster, err := fixtures.BuildE2eCluster(
		fixtures.DefaultE2eClusterConfig(),
		fixtures.WithName("e2e-service-disruptor"),
		fixtures.WithIngressPort(30083),
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

	t.Run("Inject HTTP error 500", func(t *testing.T) {
		namespace, err := k8s.NamespaceHelper().CreateRandomNamespace(context.TODO(), "test-pods")
		if err != nil {
			t.Errorf("error creating test namespace: %v", err)
			return
		}
		defer k8s.Client().CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})

		svc := fixtures.BuildHttpbinService(namespace)
		err = fixtures.DeployApp(
			k8s,
			namespace,
			fixtures.BuildHttpbinPod(namespace),
			svc,
			intstr.FromInt(80),
			20*time.Second,
		)
		if err != nil {
			t.Errorf("error deploying application httpbin: %v", err)
			return
		}

		options := disruptors.ServiceDisruptorOptions{}
		disruptor, err := disruptors.NewServiceDisruptor(context.TODO(), k8s, svc.Name, namespace, options)
		if err != nil {
			t.Errorf("error creating service disruptor: %v", err)
			return
		}

		targets, _ := disruptor.Targets(context.TODO())
		if len(targets) == 0 {
			t.Errorf("No pods matched the selector")
			return
		}

		// apply disruption in a go-routine as it is a blocking function
		go func() {
			fault := disruptors.HTTPFault{
				Port:      80,
				ErrorRate: 1.0,
				ErrorCode: 500,
			}
			httpOptions := disruptors.HTTPDisruptionOptions{}
			err := disruptor.InjectHTTPFaults(context.TODO(), fault, 10*time.Second, httpOptions)
			if err != nil {
				t.Errorf("error injecting fault: %v", err)
			}
		}()

		check := checks.HTTPCheck{
			Service:      "httpbin",
			Port:         80,
			Method:       "GET",
			Path:         "/status/200",
			Body:         []byte{},
			Delay:        2 * time.Second,
			ExpectedCode: 500,
		}
		err = check.Verify(k8s, cluster.Ingress(), namespace)
		if err != nil {
			t.Errorf("failed to access service: %v", err)
			return
		}
	})
}
