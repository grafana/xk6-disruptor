//go:build e2e
// +build e2e

package cluster

import (
	"context"
	"os/exec"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func Test_DefaultConfig(t *testing.T) {
	// create cluster with default configuration
	config, err := NewClusterConfig(
		"e2e-default-cluster",
		ClusterOptions{
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
	config, err := NewClusterConfig(
		"e2e-cluster-with-images",
		ClusterOptions{
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

	kubeconfig, err := cluster.Kubeconfig()
	if err != nil {
		t.Errorf("failed to retrieve kubeconfig: %v", err)
		return
	}

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
