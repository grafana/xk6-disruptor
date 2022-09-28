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

func buildBusyBoxPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "busybox",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "busybox",
					Image:           "busybox",
					ImagePullPolicy: corev1.PullIfNotPresent,
					Command:         []string{"sleep", "300"},
				},
			},
		},
	}
}

func buildPausedPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "paused",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "paused",
					Image:           "k8s.gcr.io/pause",
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
			},
		},
	}
}

func buildNginxPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
			Labels: map[string]string{
				"app": "e2e-test",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "nginx",
					Image:           "nginx",
					ImagePullPolicy: corev1.PullIfNotPresent,
				},
			},
		},
	}
}

func buildNginxService() *corev1.Service {
	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name: "nginx",
			Labels: map[string]string{
				"app": "e2e-test",
			},
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeNodePort,
			Selector: map[string]string{
				"app": "e2e-test",
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

func Test_WaitServiceReady(t *testing.T) {
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
		buildNginxPod(),
		metav1.CreateOptions{},
	)
	if err != nil {
		t.Errorf("failed to create pod: %v", err)
		return
	}

	_, err = k8s.CoreV1().Services(ns).Create(
		context.TODO(),
		buildNginxService(),
		metav1.CreateOptions{},
	)
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

	ns, err := k8s.Helpers().CreateRandomNamespace("test-")
	if err != nil {
		t.Errorf("error creating test namespace: %v", err)
		return
	}
	defer k8s.CoreV1().Namespaces().Delete(context.TODO(), ns, metav1.DeleteOptions{})

	_, err = k8s.CoreV1().Pods(ns).Create(
		context.TODO(),
		buildBusyBoxPod(),
		metav1.CreateOptions{},
	)
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

	ns, err := k8s.Helpers().CreateRandomNamespace("test-")
	if err != nil {
		t.Errorf("error creating test namespace: %v", err)
		return
	}
	defer k8s.CoreV1().Namespaces().Delete(context.TODO(), ns, metav1.DeleteOptions{})

	_, err = k8s.CoreV1().Pods(ns).Create(
		context.TODO(),
		buildPausedPod(),
		metav1.CreateOptions{},
	)
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

	ephemeral := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:    "ephemeral",
			Image:   "busybox",
			Command: []string{"sleep", "300"},
			TTY:     true,
			Stdin:   true,
		},
	}

	err = k8s.NamespacedHelpers(ns).AttachEphemeralContainer("paused", ephemeral, 15*time.Second)
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
