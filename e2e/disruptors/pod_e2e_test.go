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

func Test_PodDisruptor(t *testing.T) {
	t.Parallel()

	cluster, err := fixtures.BuildCluster("e2e-pod-disruptor")
	if err != nil {
		t.Errorf("failed to create cluster config: %v", err)
		return
	}

	k8s, err := kubernetes.NewFromKubeconfig(cluster.Kubeconfig())
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	testCases := []struct {
		title     string
		fault     disruptors.HTTPFault
		options   disruptors.HTTPDisruptionOptions
		path      string
		method    string
		body      []byte
		checkCode int
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
			method:    "GET",
			path:      "/status/200",
			body:      []byte{},
			checkCode: 500,
		},
	}

	t.Cleanup(func() {
		cluster.Delete()
	})

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			namespace, err := k8s.Helpers().CreateRandomNamespace("test-pods")
			if err != nil {
				t.Errorf("error creating test namespace: %v", err)
				return
			}
			defer k8s.CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})

			err = fixtures.DeployApp(
				k8s,
				namespace,
				fixtures.BuildHttpbinPod(),
				fixtures.BuildHttpbinService(),
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
				err := disruptor.InjectHTTPFaults(tc.fault, 60, tc.options)
				if err != nil {
					t.Errorf("failed to setup disruptor: %v", err)
					return
				}
			}()

			err = checks.CheckService(
				k8s,
				checks.ServiceCheck{
					Namespace:    namespace,
					Service:      "httpbin",
					Port:         80,
					Method:       tc.method,
					Path:         tc.path,
					Body:         tc.body,
					Delay:        2 * time.Second,
					ExpectedCode: tc.checkCode,
				},
			)
			if err != nil {
				t.Errorf("failed to access service: %v", err)
				return
			}
		})
	}
}
