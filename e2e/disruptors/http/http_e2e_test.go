//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/cluster"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const clusterName = "e2e-httpdisruptor"

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

// expose ngix pod at the node port 32080
func buildHttpbinService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "httpbin",
			Labels: map[string]string{
				"app": "httpbin",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "httpbin",
			},
			Ports: []corev1.ServicePort{
				{
					Name:     "http",
					Port:     80,
					NodePort: 32080,
				},
			},
		},
	}
}

// path to kubeconfig file for the test cluster
var kubeconfig string

func TestMain(m *testing.M) {
	// Create cluster that exposes the cluster node port 32080 to the local (host) port 9080
	fmt.Printf("creating cluster '%s'\n", clusterName)
	config, err := cluster.NewClusterConfig(
		clusterName,
		cluster.ClusterOptions{
			NodePorts: []cluster.NodePort{
				{
					NodePort: 32080,
					HostPort: 32080,
				},
			},
			Images: []string{"grafana/xk6-disruptor-agent"},
			Wait:   time.Second * 60,
		},
	)
	if err != nil {
		fmt.Printf("failed to create cluster config: %v", err)
		os.Exit(1)
	}

	cluster, err := config.Create()
	if err != nil {
		fmt.Printf("failed to create cluster: %v", err)
		os.Exit(1)
	}

	// retrieve path to kubeconfig
	kubeconfig = cluster.Kubeconfig()

	// run tests
	rc := m.Run()

	// cleanup
	cluster.Delete()

	os.Exit(rc)
}

func Test_Error500(t *testing.T) {
	k8s, err := kubernetes.NewFromKubeconfig(kubeconfig)
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

	_, err = k8s.CoreV1().Services(ns).Create(
		context.TODO(),
		buildHttpbinService(),
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Errorf("failed to create service: %v", err)
		return
	}

	// wait for the service to be ready for accepting requests
	err = k8s.NamespacedHelpers(ns).WaitServiceReady("httpbin", time.Second*30)
	if err != nil {
		t.Errorf("error waiting for service httpbin: %v", err)
		return
	}

	// access service using the local port on which the service was exposed (see ClusterOptions)
	resp, err := http.Get("http://127.0.0.1:32080")
	if err != nil {
		t.Errorf("failed to access service: %v", err)
		return
	}

	if resp.StatusCode != 500 {
		t.Errorf("expected status code 500 but %d received:", resp.StatusCode)
		return
	}
}
