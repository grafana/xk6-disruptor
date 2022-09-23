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

	"github.com/grafana/xk6-disruptor/pkg/api/disruptors"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/cluster"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const clusterName = "e2e-pod-disruptor"

// deploy pod with httpbin
const podManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: httpbin
  namespace: %s
  labels:
    app: httpbin
spec:
  containers:
  - name: httpbin
    image: kennethreitz/httpbin
`

// expose httpbin pod at the node port 32080
const serviceManifest = `
apiVersion: v1
kind: Service
metadata:
  name: httpbin
  namespace: %s
spec:
  type: NodePort
  ports:
  - name: "http"
    port: 80
    nodePort: 32080
    targetPort: 80
  selector:
    app: httpbin
`

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
	kubeconfig, _ = cluster.Kubeconfig()

	// run tests
	rc := m.Run()

	// clean up
	cluster.Delete()

	os.Exit(rc)
}

func Test_Error500(t *testing.T) {
	k8s, err := kubernetes.NewFromKubeconfig(kubeconfig)
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	ns, err := k8s.Helpers().CreateRandomNamespace("test-pods")
	if err != nil {
		t.Errorf("error creating test namespace: %v", err)
		return
	}
	defer k8s.CoreV1().Namespaces().Delete(context.TODO(), ns, metav1.DeleteOptions{})

	manifest := fmt.Sprintf(podManifest, ns)
	err = k8s.Create(manifest)
	if err != nil {
		t.Errorf("failed to create pod: %v", err)
		return
	}

	manifest = fmt.Sprintf(serviceManifest, ns)
	err = k8s.Create(manifest)
	if err != nil {
		t.Errorf("failed to create service: %v", err)
		return
	}

	// wait for the service to be ready for accepting requests
	err = k8s.NamespacedHelpers(ns).WaitServiceReady("httpbin", time.Second*20)
	if err != nil {
		t.Errorf("error waiting for service httpbin: %v", err)
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

	time.Sleep(2 * time.Second)

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
