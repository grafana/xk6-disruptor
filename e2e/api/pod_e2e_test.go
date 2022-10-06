//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/grafana/xk6-disruptor/pkg/api/disruptors"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/checks"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fixtures"
)

func Test_PodDisruptor(t *testing.T) {
	cluster, err := fixtures.BuildCluster("e2e-pod-disruptor")
	if err != nil {
		t.Errorf("failed to create cluster config: %v", err)
		return
	}
	defer cluster.Delete()

	k8s, err := kubernetes.NewFromKubeconfig(cluster.Kubeconfig())
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	t.Run("Inject Http error 500", func(t *testing.T) {
		ns, err := k8s.Helpers().CreateRandomNamespace("test-pods")
		if err != nil {
			t.Errorf("error creating test namespace: %v", err)
			return
		}
		defer k8s.CoreV1().Namespaces().Delete(context.TODO(), ns, metav1.DeleteOptions{})

		err = fixtures.DeployApp(
			k8s,
			ns,
			fixtures.BuildHttpbinPod(),
			fixtures.BuildHttpbinService(),
			20*time.Second,
		)
		if err != nil {
			t.Errorf("error deploying httpbin: %v", err)
			return
		}

		// create pod disruptor
		selector := disruptors.PodSelector{
			Namespace: ns,
			Select: disruptors.PodAttributes{
				Labels: map[string]string{
					"app": "httpbin",
				},
			},
		}
		options := disruptors.PodDisruptorOptions{
			InjectTimeout: 10,
		}
		disruptor, err := disruptors.NewPodDisruptor(k8s, selector, options)
		if err != nil {
			t.Errorf("error creating selector: %v", err)
			return
		}

		targets, _ := disruptor.Targets()
		if len(targets) == 0 {
			t.Errorf("No pods matched the selector")
			return
		}

		// apply disruption in a go-routine as it is a blocking function
		go func() {
			// apply httpfailure
			fault := disruptors.HttpFault{
				ErrorRate: 1.0,
				ErrorCode: 500,
			}
			opts := disruptors.HttpDisruptionOptions{
				TargetPort: 80,
				ProxyPort:  8080,
			}
			err := disruptor.InjectHttpFaults(fault, 10, opts)
			if err != nil {
				t.Errorf("error injecting fault: %v", err)
			}
		}()

		err = checks.CheckService(checks.ServiceCheck{
			Delay:        2 * time.Second,
			ExpectedCode: 500,
		})
		if err != nil {
			t.Errorf("failed to access service: %v", err)
			return
		}
	})
}
