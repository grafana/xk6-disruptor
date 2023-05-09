//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"
	"github.com/grafana/xk6-disruptor/pkg/testutils/cluster"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fixtures"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Kubernetes(t *testing.T) {
	cluster, err := fixtures.BuildE2eCluster(
		fixtures.DefaultE2eClusterConfig(),
		fixtures.WithName("e2e-kubernetes"),
		fixtures.WithIngressPort(30081),
	)
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}

	t.Cleanup(func() {
		_ = cluster.Delete()
	})

	k8s, err := kubernetes.NewFromKubeconfig(cluster.Kubeconfig())
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	kubeconfig := cluster.Kubeconfig()

	// Test Creating a random namespace
	t.Run("Create Random Namespace", func(t *testing.T) {
		k8s, err := kubernetes.NewFromKubeconfig(kubeconfig)
		if err != nil {
			t.Errorf("error creating kubernetes client: %v", err)
			return
		}
		prefix := "test"
		ns, err := k8s.NamespaceHelper().CreateRandomNamespace(context.TODO(), prefix)
		if err != nil {
			t.Errorf("unexpected error creating namespace: %v", err)
			return
		}
		if !strings.HasPrefix(ns, prefix) {
			t.Errorf("returned namespace does not have expected prefix '%s': %s", prefix, ns)
		}
	})

	// Test Wait Service Ready helper
	t.Run("Wait Service Ready", func(t *testing.T) {
		namespace, err := k8s.NamespaceHelper().CreateRandomNamespace(context.TODO(), "test-")
		if err != nil {
			t.Errorf("error creating test namespace: %v", err)
			return
		}
		defer k8s.Client().CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})

		// Deploy nginx and expose it as a service. Intentionally not using e2e fixures
		// because these functions rely on WaitPodRunnin and WaitServiceReady which we
		// are testing here.
		_, err = k8s.Client().CoreV1().Pods(namespace).Create(
			context.TODO(),
			fixtures.BuildNginxPod(namespace),
			metav1.CreateOptions{},
		)
		if err != nil {
			t.Errorf("failed to create pod: %v", err)
			return
		}

		_, err = k8s.Client().CoreV1().Services(namespace).Create(
			context.TODO(),
			fixtures.BuildNginxService(namespace),
			metav1.CreateOptions{},
		)
		if err != nil {
			t.Errorf("failed to create nginx service: %v", err)
			return
		}

		// wait for the service to be ready for accepting requests
		err = k8s.ServiceHelper(namespace).WaitServiceReady(context.TODO(), "nginx", time.Second*20)
		if err != nil {
			t.Errorf("error waiting for service nginx: %v", err)
			return
		}
	})

	t.Run("Exec Command", func(t *testing.T) {
		namespace, err := k8s.NamespaceHelper().CreateRandomNamespace(context.TODO(), "test-")
		if err != nil {
			t.Errorf("error creating test namespace: %v", err)
			return
		}
		defer k8s.Client().CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})

		err = fixtures.RunPod(k8s, namespace, fixtures.BuildBusyBoxPod(namespace), 10*time.Second)
		if err != nil {
			t.Errorf("error creating pod: %v", err)
			return
		}

		stdout, _, err := k8s.PodHelper(namespace).Exec(
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
	})

	t.Run("Attach Ephemeral Container", func(t *testing.T) {
		namespace, err := k8s.NamespaceHelper().CreateRandomNamespace(context.TODO(), "test-")
		if err != nil {
			t.Errorf("error creating test namespace: %v", err)
			return
		}
		defer k8s.Client().CoreV1().Namespaces().Delete(context.TODO(), namespace, metav1.DeleteOptions{})

		err = fixtures.RunPod(k8s, namespace, fixtures.BuildPausedPod(namespace), 10*time.Second)
		if err != nil {
			t.Errorf("error running pod %v: ", err)
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

		err = k8s.PodHelper(namespace).AttachEphemeralContainer(
			context.TODO(),
			"paused",
			ephemeral,
			helpers.AttachOptions{
				Timeout: 15 * time.Second,
			},
		)

		if err != nil {
			t.Errorf("error attaching ephemeral container to pod: %v", err)
			return
		}

		stdout, _, err := k8s.PodHelper(namespace).Exec(
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
	})
}

func Test_UnsupportedKubernetesVersion(t *testing.T) {
	config, err := cluster.NewConfig(
		"e2e-v1-22-0-cluster",
		cluster.Options{
			Version: "v1.22.0",
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
	defer cluster.Delete()

	_, err = kubernetes.NewFromKubeconfig(cluster.Kubeconfig())
	if err == nil {
		t.Errorf("should had failed creating kubernetes client")
		return
	}
}
