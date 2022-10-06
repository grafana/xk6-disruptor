//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/checks"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fixtures"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// deploy pod with [httpbin] and the httpdisruptor as sidekick container
func buildHttpbinPodWithDisruptorAgent() *corev1.Pod {
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
					Name:            "httpdisruptor",
					Image:           "grafana/xk6-disruptor-agent",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"xk6-disruptor-agent"},
					Args: []string{
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
					},
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

// Test_InjectHttp500 tests in the Httpbin pod by running the xk6-disruptor agent as a sidekick container
func Test_InjectHttp500(t *testing.T) {
	t.Parallel()

	cluster, err := fixtures.BuildCluster("e2e-http-disruptor")
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

	ns, err := k8s.Helpers().CreateRandomNamespace("test-")
	if err != nil {
		t.Errorf("error creating test namespace: %v", err)
		return
	}
	defer k8s.CoreV1().Namespaces().Delete(context.TODO(), ns, metav1.DeleteOptions{})

	_, err = k8s.CoreV1().Pods(ns).Create(
		context.TODO(),
		buildHttpbinPodWithDisruptorAgent(),
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Errorf("failed to create pod: %v", err)
		return
	}

	err = fixtures.ExposeService(k8s, ns, fixtures.BuildHttpbinService(), 20*time.Second)
	if err != nil {
		t.Errorf("failed to create service: %v", err)
		return
	}

	err = checks.CheckService(checks.ServiceCheck{
		ExpectedCode: 500,
	})

	if err != nil {
		t.Errorf("failed : %v", err)
		return
	}
}
