//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"os/exec"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/testutils/cluster"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"

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

func Test_UseEtcdRamDisk(t *testing.T) {
	// create cluster with default configuration
	config, err := cluster.NewConfig(
		"e2e-etcdramdisk-cluster",
		cluster.Options{
			Wait:           time.Second * 60,
			UseEtcdRAMDisk: true,
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
	busybox := builders.NewContainerBuilder("busybox").
		WithImage("busybox").
		WithPullPolicy(corev1.PullNever).
		Build()

	return builders.NewPodBuilder("busybox").
		WithContainer(*busybox).
		Build()
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

// FIXME: this is a very basic test. Check for error conditions and ensure
// returned cluster is functional.
func Test_GetCluster(t *testing.T) {
	// create cluster with  configuration
	config, err := cluster.NewConfig(
		"e2e-preexisting-cluster",
		cluster.Options{
			Wait: time.Second * 60,
		},
	)
	if err != nil {
		t.Errorf("failed creating cluster configuration: %v", err)
		return
	}

	c, err := config.Create()
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}

	cluster, err := cluster.GetCluster(c.Name(), c.Kubeconfig())
	if err != nil {
		t.Errorf("failed to get cluster: %v", err)
		return
	}

	// delete cluster
	cluster.Delete()
	if err != nil {
		t.Errorf("failed to delete cluster: %v", err)
		return
	}
}
