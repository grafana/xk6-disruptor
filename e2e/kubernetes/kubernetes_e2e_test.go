//go:build e2e
// +build e2e

package kubernetes

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/cluster"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const clusterName = "e2e-kubernetes"

var kubeconfig string

func TestMain(m *testing.M) {
	// create cluster exposing node port 32080 on host port 9080
	config, err := cluster.NewClusterConfig(
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

	// delete cluster
	cluster.Delete()

	os.Exit(rc)
}

const busyboxManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: busybox
  namespace: %s
spec:
  containers:
  - name: busybox
    image: busybox
    command: ["sleep", "300"]
`

const pausedManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: paused
  namespace: %s
spec:
  containers:
  - name: paused
    image: k8s.gcr.io/pause
`

const nginxManifest = `
apiVersion: v1
kind: Pod
metadata:
  name: nginx
  namespace: %s
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
  namespace: %s
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

	manifest := fmt.Sprintf(busyboxManifest, ns)
	err = k8s.Create(manifest)
	if err != nil {
		t.Errorf("failed to create pod: %v", err)
		return
	}

	pod := corev1.Pod{}
	err = k8s.Get("Pod", "busybox", ns, &pod)
	if err != nil {
		t.Errorf("failed to get pod: %v", err)
		return
	}

	err = k8s.Delete("Pod", "busybox", ns)
	if err != nil {
		t.Errorf("failed to delete pod: %v", err)
		return
	}
}

func Test_WaitServiceReady(t *testing.T) {
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

	manifest := fmt.Sprintf(nginxManifest, ns)
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
	err = k8s.NamespacedHelpers(ns).WaitServiceReady("nginx", time.Second*20)
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

func Test_CreateRandomNamespace(t *testing.T) {
	k8s, err := kubernetes.NewFromKubeconfig(kubeconfig)
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}
	prefix := "test"
	ns, err := k8s.Helpers().CreateRandomNamespace(prefix)
	if err != nil {
		t.Errorf("unexpected error creating namespace: %v", err)
		return
	}
	if !strings.HasPrefix(ns, prefix) {
		t.Errorf("returned namespace does not have expected prefix '%s': %s", prefix, ns)

	}
}

func Test_Exec(t *testing.T) {
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

	manifest := fmt.Sprintf(busyboxManifest, ns)
	err = k8s.Create(manifest)
	if err != nil {
		t.Errorf("failed to create pod: %v", err)
		return
	}

	timeout := time.Second * 15
	running, err := k8s.NamespacedHelpers(ns).WaitPodRunning(
		"busybox",
		timeout,
	)
	if err != nil {
		t.Errorf("error waiting for pod: %v", err)
		return
	}
	if !running {
		t.Errorf("pod not ready after %f: ", timeout.Seconds())
		return
	}

	stdout, _, err := k8s.NamespacedHelpers(ns).Exec(
		"busybox",
		"busybox",
		[]string{"echo", "-n", "hello", "world"},
		nil,
	)
	if err != nil {
		t.Errorf("error executing command in pod: %v", err)
		return
	}

	greetings := "hello world"
	if string(stdout) != "hello world" {
		t.Errorf("stdout does not match expected result:\nexpected: %s\nactual%s\n", greetings, string(stdout))
		return
	}
}

func Test_AttachEphemeral(t *testing.T) {
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

	manifest := fmt.Sprintf(pausedManifest, ns)
	err = k8s.Create(manifest)
	if err != nil {
		t.Errorf("failed to create pod: %v", err)
		return
	}

	timeout := time.Second * 15
	running, err := k8s.NamespacedHelpers(ns).WaitPodRunning(
		"paused",
		timeout,
	)
	if err != nil {
		t.Errorf("error waiting for pod: %v", err)
		return
	}
	if !running {
		t.Errorf("pod not ready after %f: ", timeout.Seconds())
		return
	}

	ephemeral :=  corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            "ephemeral",
			Image:           "busybox",
			Command:         []string{"sleep", "300"},
			TTY:             true,
			Stdin:           true,
		},
	}

	err = k8s.NamespacedHelpers(ns).AttachEphemeralContainer("paused", ephemeral, 15 * time.Second)
	if err != nil {
		t.Errorf("error attaching ephemeral container to pod: %v", err)
		return
	}

	stdout, _, err := k8s.NamespacedHelpers(ns).Exec(
		"paused",
		"ephemeral",
		[]string{"echo", "-n", "hello", "world"},
		nil,
	)
	if err != nil {
		t.Errorf("error executing command in pod: %v", err)
		return
	}

	greetings := "hello world"
	if string(stdout) != "hello world" {
		t.Errorf("stdout does not match expected result:\nexpected: %s\nactual%s\n", greetings, string(stdout))
		return
	}
}

