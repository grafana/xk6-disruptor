//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/testutils/cluster"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func Test_DefaultConfig(t *testing.T) {
	// create cluster with default configuration
	config, err := cluster.NewConfig(
		"e2e-default-cluster",
		cluster.Options{
			Wait: time.Second * 60,
		},
	)
	if err != nil {
		t.Errorf("failed creating cluster configuration: %v", err)
		return
	}

	cluster, err := config.Create()
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}

	// delete cluster
	cluster.Delete()
}

func getKubernetesClient(kubeconfig string) (kubernetes.Interface, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

// buildBusyboxPod returns a pod specification for running Busybox from a local image
func buildBusyboxPod() *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name: "busybox",
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{
				{
					Name:            "busybox",
					Image:           "busybox",
					ImagePullPolicy: corev1.PullNever,
				},
			},
		},
	}
}

func Test_PreloadImages(t *testing.T) {
	// ensure image is available locally
	output, err := exec.Command("docker", "pull", "busybox").CombinedOutput()
	if err != nil {
		t.Errorf("error pulling image: %s", string(output))
		return
	}

	// create cluster with preloaded images
	config, err := cluster.NewConfig(
		"e2e-cluster-with-images",
		cluster.Options{
			Wait:   time.Second * 60,
			Images: []string{"busybox"},
		},
	)
	if err != nil {
		t.Errorf("failed to create cluster config: %v", err)
		return
	}

	cluster, err := config.Create()
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}

	defer cluster.Delete()

	kubeconfig := cluster.Kubeconfig()

	k8s, err := getKubernetesClient(kubeconfig)
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	pod := buildBusyboxPod()
	_, err = k8s.CoreV1().Pods("default").Create(context.TODO(), pod, metav1.CreateOptions{})
	if err != nil {
		t.Errorf("failed to create pod: %v", err)
		return
	}
	// FIXME: using hardcoded waits is flaky
	time.Sleep(time.Second * 5)

	pod, err = k8s.CoreV1().Pods("default").Get(context.TODO(), "busybox", metav1.GetOptions{})
	if err != nil {
		t.Errorf("failed to get pod: %v", err)
		return
	}

	waiting := pod.Status.ContainerStatuses[0].State.Waiting
	if waiting != nil && (waiting.Reason == "ErrImageNeverPull") {
		t.Errorf("pod is waiting for image")
		return
	}
}

func Test_PortAllocation(t *testing.T) {
	// create cluster with default configuration
	nodePort := cluster.NodePort{
		HostPort: 32090,
		NodePort: 32090,
	}
	config, err := cluster.NewConfig(
		"e2e-default-cluster",
		cluster.Options{
			Wait: time.Second * 60,
			NodePorts: []cluster.NodePort{
				nodePort,
			},
		},
	)
	if err != nil {
		t.Errorf("failed creating cluster configuration: %v", err)
		return
	}

	cluster, err := config.Create()
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}
	// delete cluster
	defer cluster.Delete()

	firstNodePort := cluster.AllocatePort()
	if firstNodePort.HostPort == 0 {
		t.Errorf("should have allocated a node port")
		return
	}

	secondNodePort := cluster.AllocatePort()
	if secondNodePort.HostPort != 0 {
		t.Errorf("should have failed allocating node port")
		return
	}

	cluster.ReleasePort(firstNodePort)
	secondNodePort = cluster.AllocatePort()
	if secondNodePort.HostPort == 0 {
		t.Errorf("should have allocated a node port")
		return
	}
}

func Test_KubernetesVersion(t *testing.T) {
	// create cluster with default configuration
	config, err := cluster.NewConfig(
		"e2e-default-cluster",
		cluster.Options{
			Version: "v1.24.0",
			Wait:    time.Second * 60,
		},
	)
	if err != nil {
		t.Errorf("failed creating cluster configuration: %v", err)
		return
	}

	cluster, err := config.Create()
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}

	// delete cluster
	cluster.Delete()
}

func Test_InvalidKubernetesVersion(t *testing.T) {
	// create cluster with default configuration
	config, err := cluster.NewConfig(
		"e2e-default-cluster",
		cluster.Options{
			Version: "v0.0.0",
			Wait:    time.Second * 60,
		},
	)
	if err != nil {
		t.Errorf("failed creating cluster configuration: %v", err)
		return
	}

	cluster, err := config.Create()
	if err == nil {
		t.Errorf("Should have failed creating cluster")
		cluster.Delete()
		return
	}

}
