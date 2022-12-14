//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/checks"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fixtures"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var injectHTTP500 = []string{
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

// deploy pod with [httpbin] and the xkdisruptor as sidekick container
func buildHttpbinPodWithDisruptorAgent(args []string) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "httpbin",
			Labels: map[string]string{
				"app": "httpbin",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "httpbin",
					Image:           "kennethreitz/httpbin",
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
				{
					Name:            "xk6-disruptor-agent",
					Image:           "ghcr.io/grafana/xk6-disruptor-agent",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"xk6-disruptor-agent"},
					Args:            args,
					SecurityContext: &corev1.SecurityContext{
						Capabilities: &corev1.Capabilities{
							Add: []corev1.Capability{
								"NET_ADMIN",
							},
						},
					},
				},
			},
		},
	}
}


func Test_Agent(t *testing.T) {
	t.Parallel()

	cluster, err := fixtures.BuildCluster("e2e-xk6-agent")
	if err != nil {
		t.Errorf("failed to create cluster config: %v", err)
		return
	}

	t.Cleanup(func(){
		_ = cluster.Delete()
	})

	k8s, err := kubernetes.NewFromKubeconfig(cluster.Kubeconfig())
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	testCases := []struct{
			// description of the test
			title string
			// command to pass to disruptor agent running in the target pod
			cmd  []string
			// Function that checks the test conditions
			check func(k8s kubernetes.Kubernetes, ns string) error
	}{
		{
			title: "Inject HTTP 500",
			cmd:  injectHTTP500,
			check: func(k8s kubernetes.Kubernetes, ns string) error {
				err = fixtures.ExposeService(k8s, ns, fixtures.BuildHttpbinService(), 20*time.Second)
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
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()
			ns, err := k8s.Helpers().CreateRandomNamespace("test-")
			if err != nil {
				t.Errorf("error creating test namespace: %v", err)
				return
			}
			defer k8s.CoreV1().Namespaces().Delete(context.TODO(), ns, metav1.DeleteOptions{})

			err = fixtures.RunPod(
				k8s,
				ns,
				buildHttpbinPodWithDisruptorAgent(tc.cmd),
				30 * time.Second,
			)
			if err != nil {
				t.Errorf("failed to create pod: %v", err)
				return
			}

			err = tc.check(k8s, ns)
			if err != nil {
				t.Errorf("failed : %v", err)
				return
			}
		})
	}
}
