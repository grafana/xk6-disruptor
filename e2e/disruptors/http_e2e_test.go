//go:build e2e
// +build e2e

package e2e

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/cluster"
)

const clusterName = "e2e-httpdisruptor"

// deploy pod with [httpbin] and the httpdisruptor as sidekick container
const podManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: httpbin
  namespace: default
  labels:
    app: httpbin
spec:
  containers:
  - name: httpbin
    image: kennethreitz/httpbin
  - name: httpdisruptor
    image: grafana/xk6-disruptor-agent
    imagePullPolicy: IfNotPresent
    securityContext:
      capabilities:
        add: ["NET_ADMIN"]
    command: ["xk6-disruptor-agent", "http"]
    args: [ "--duration", "300s", "--rate", "1.0", "--error", "500", "--port", "8080", "--target", "80" ]
`

// expose ngix pod at the node port 32080
const serviceManifest = `
apiVersion: v1
kind: Service
metadata:
  name: httpbin
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
	defer cluster.Delete()

	// retrieve path to kubeconfig
	kubeconfig, _ = cluster.Kubeconfig()

	// run tests
	rc := m.Run()

	os.Exit(rc)
}

func Test_Error500(t *testing.T) {
	k8s, err := kubernetes.NewFromKubeconfig(kubeconfig)
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	err = k8s.Create(podManifest)
	if err != nil {
		t.Errorf("failed to create pod: %v", err)
		return
	}

	err = k8s.Create(serviceManifest)
	if err != nil {
		t.Errorf("failed to create service: %v", err)
		return
	}

	// wait for the service to be ready for accepting requests
	err = k8s.Helpers().WaitServiceReady("httpbin", time.Second*20)
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
