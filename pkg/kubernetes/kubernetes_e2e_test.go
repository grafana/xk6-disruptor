//go:build e2e
// +build e2e

package kubernetes

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/testutils/cluster"
	corev1 "k8s.io/api/core/v1"
)

const clusterName = "e2e-kubernetes"

var kubeconfig string

func TestMain(m *testing.M) {
	c, err := cluster.CreateCluster(
		clusterName,
		cluster.ClusterOptions{
			Wait: time.Second * 60,
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
