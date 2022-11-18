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

	testCases := []struct{
		title       string
		fault       disruptors.HTTPFault
		options     disruptors.HTTPDisruptionOptions
		expectError bool
	} {
		{
			title:  "Inject Http error 500",
			fault: 	disruptors.HTTPFault{
				Port:      80,
				ErrorRate: 1.0,
				ErrorCode: 500,
			},
			options:  disruptors.HTTPDisruptionOptions{
				ProxyPort: 8080,
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			ns, err := k8s.Helpers().CreateRandomNamespace("test-pods")
			if err != nil {
				t.Errorf("error creating test namespace: %v", err)
				return
			}
			defer k8s.CoreV1().Namespaces().Delete(context.TODO(), ns, metav1.DeleteOptions{})

			nodePort := cluster.AllocatePort()
			if nodePort.HostPort == 0 {
				t.Errorf("no nodeport available for test")
				return
			}
			defer cluster.ReleasePort(nodePort)

			err = fixtures.DeployApp(
				k8s,
				ns,
				fixtures.BuildHttpbinPod(),
				fixtures.BuildHttpbinService(nodePort.NodePort),
				30*time.Second,
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
				err := disruptor.InjectHTTPFaults(tc.fault, 10, tc.options)
				if err != nil {
					t.Errorf("error injecting fault: %v", err)
				}
			}()

			err = checks.CheckService(checks.ServiceCheck{
				Port:         nodePort.HostPort,
				Delay:        2 * time.Second,
				ExpectedCode: 500,
			})
			if err != nil {
				t.Errorf("failed to access service: %v", err)
				return
			}
		})
	}
}

