//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/grafana/xk6-disruptor/pkg/disruptors"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/checks"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fixtures"
)

func Test_ServiceDisruptor(t *testing.T) {
	cluster, err := fixtures.BuildCluster("e2e-service-disruptor")
	if err != nil {
		t.Errorf("failed to create cluster config: %v", err)
		return
	}
	defer cluster.Delete()

	k8s, err := kubernetes.NewFromKubeconfig(context.TODO(), cluster.Kubeconfig())
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	t.Run("Inject HTTP error 500", func(t *testing.T) {
		ns, err := k8s.Helpers().CreateRandomNamespace("test-pods")
		if err != nil {
			t.Errorf("error creating test namespace: %v", err)
			return
		}
		defer k8s.CoreV1().Namespaces().Delete(context.TODO(), ns, metav1.DeleteOptions{})

		svc := fixtures.BuildHttpbinService()
		err = fixtures.DeployApp(
			k8s,
			ns,
			fixtures.BuildHttpbinPod(),
			svc,
			20*time.Second,
		)
		if err != nil {
			t.Errorf("error deploying application httpbin: %v", err)
			return
		}

		options := disruptors.ServiceDisruptorOptions{}
		disruptor, err := disruptors.NewServiceDisruptor(k8s, svc.Name, ns, options)
		if err != nil {
			t.Errorf("error creating service disruptor: %v", err)
			return
		}

		targets, _ := disruptor.Targets()
		if len(targets) == 0 {
			t.Errorf("No pods matched the selector")
			return
		}

		// apply disruption in a go-routine as it is a blocking function
		go func() {
			fault := disruptors.HTTPFault{
				ErrorRate: 1.0,
				ErrorCode: 500,
			}
			httpOptions := disruptors.HTTPDisruptionOptions{}
			err := disruptor.InjectHTTPFaults(fault, 10, httpOptions)
			if err != nil {
				t.Errorf("error injecting fault: %v", err)
			}
		}()

		err = checks.CheckService(
			k8s,
			checks.ServiceCheck{
				Namespace:    ns,
				Service:      "httpbin",
				Port:         80,
				Method:       "GET",
				Path:         "/status/200",
				Body:         []byte{},
				Delay:        2 * time.Second,
				ExpectedCode: 500,
			},
		)
		if err != nil {
			t.Errorf("failed to access service: %v", err)
			return
		}
	})
}
