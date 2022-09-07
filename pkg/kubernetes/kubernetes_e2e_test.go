//go:build e2e
// +build e2e

package kubernetes

import (
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/testutils/cluster"
	corev1 "k8s.io/api/core/v1"
)

const clusterName = "e2e-kubernetes"

var kubeconfig string

func TestMain(m *testing.M) {
	// create cluster exposing node port 32080 on host port 9080
	c, err := cluster.CreateCluster(
		clusterName,
		cluster.ClusterOptions{
			Wait: time.Second * 60,
			NodePorts: []cluster.NodePort{
				{
					HostPort: 9080,
					NodePort: 32080,
				},
			},
		},
	)
	if err != nil {
		fmt.Printf("failed to create cluster: %v", err)
		os.Exit(1)
	}

	// retrieve path to kubeconfig
	kubeconfig, _ = c.Kubeconfig()

	// run tests
	rc := m.Run()

	// delete cluster
	c.Delete()

	os.Exit(rc)
}

const busyboxManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: busybox
  namespace: default
spec:
  containers:
  - name: busybox
    image: busybox
`

const nginxManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  namespace: default
  labels:
    app: e2e-test
spec:
  containers:
  - name: nginx
    image: nginx
`

// expose nginx pod at the node port 32080
const serviceManifest = `
apiVersion: v1
kind: Service
metadata:
  name: nginx
spec:
  type: NodePort
  ports:
  - name: "http"
    port: 80
    nodePort: 32080
    targetPort: 80
  selector:
    app: e2e-test
`

func Test_CreateGetDeletePod(t *testing.T) {
	k8s, err := NewFromKubeconfig(kubeconfig)
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	err = k8s.Create(busyboxManifest)
	if err != nil {
		t.Errorf("failed to create pod: %v", err)
		return
	}

	pod := corev1.Pod{}
	err = k8s.Get("Pod", "busybox", "default", &pod)
	if err != nil {
		t.Errorf("failed to get pod: %v", err)
		return
	}

	err = k8s.Delete("Pod", "busybox", "default")
	if err != nil {
		t.Errorf("failed to delete pod: %v", err)
		return
	}
}

func Test_WaitServiceReady(t *testing.T) {
	k8s, err := NewFromKubeconfig(kubeconfig)
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	err = k8s.Create(nginxManifest)
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
	err = k8s.Helpers().WaitServiceReady("nginx", time.Second*20)
	if err != nil {
		t.Errorf("error waiting for service nginx: %v", err)
		return
	}

	// access service using the local port on which the service was exposed (see ClusterOptions)
	_, err = http.Get("http://127.0.0.1:9080")
	if err != nil {
		t.Errorf("failed to access service: %v", err)
		return
	}
}
