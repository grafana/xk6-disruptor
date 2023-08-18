//go:build e2e
// +build e2e

package e2e

import (
	"context"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"
	kindcluster "github.com/grafana/xk6-disruptor/pkg/testutils/cluster"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/cluster"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/deploy"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fixtures"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/kubernetes/namespace"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_Kubernetes(t *testing.T) {
	cluster, err := cluster.BuildE2eCluster(
		t,
		cluster.DefaultE2eClusterConfig(),
		cluster.WithName("e2e-kubernetes"),
		cluster.WithIngressPort(30081),
	)
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}

	k8s, err := kubernetes.NewFromKubeconfig(cluster.Kubeconfig())
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	// Test Wait Pod Running
	t.Run("Wait Pod is Running", func(t *testing.T) {
		namespace, err := namespace.CreateTestNamespace(context.TODO(), t, k8s.Client())
		if err != nil {
			t.Errorf("failed to create test namespace: %v", err)
			return
		}

		// Deploy nginx
		_, err = k8s.Client().CoreV1().Pods(namespace).Create(
			context.TODO(),
			fixtures.BuildNginxPod(),
			metav1.CreateOptions{},
		)
		if err != nil {
			t.Errorf("failed to create pod: %v", err)
			return
		}

		// wait for the service to be ready for accepting requests
		running, err := k8s.PodHelper(namespace).WaitPodRunning(context.TODO(), "nginx", time.Second*20)
		if err != nil {
			t.Errorf("error waiting for pod %v", err)
			return
		}
		if !running {
			t.Errorf("timeout expired waiting for pod ready")
			return
		}
	})

	// Test Wait Service Ready helper
	t.Run("Wait Service Ready", func(t *testing.T) {
		namespace, err := namespace.CreateTestNamespace(context.TODO(), t, k8s.Client())
		if err != nil {
			t.Errorf("failed to create test namespace: %v", err)
			return
		}

		// Deploy nginx and expose it as a service. Intentionally not using e2e fixures
		// because these functions rely on WaitPodRunnin and WaitServiceReady which we
		// are testing here.
		_, err = k8s.Client().CoreV1().Pods(namespace).Create(
			context.TODO(),
			fixtures.BuildNginxPod(),
			metav1.CreateOptions{},
		)
		if err != nil {
			t.Errorf("failed to create pod: %v", err)
			return
		}

		_, err = k8s.Client().CoreV1().Services(namespace).Create(
			context.TODO(),
			fixtures.BuildNginxService(),
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
		namespace, err := namespace.CreateTestNamespace(context.TODO(), t, k8s.Client())
		if err != nil {
			t.Errorf("failed to create test namespace: %v", err)
			return
		}

		err = deploy.RunPod(k8s, namespace, fixtures.BuildBusyBoxPod(), 10*time.Second)
		if err != nil {
			t.Errorf("error creating pod: %v", err)
			return
		}

		stdout, _, err := k8s.PodHelper(namespace).Exec(
			context.TODO(),
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
		namespace, err := namespace.CreateTestNamespace(context.TODO(), t, k8s.Client())
		if err != nil {
			t.Errorf("failed to create test namespace: %v", err)
			return
		}

		err = deploy.RunPod(k8s, namespace, fixtures.BuildPausedPod(), 10*time.Second)
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
			context.TODO(),
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
	// TODO: use e2e cluster. This will require an option for setting the K8s version in the e2e cluster
	config, err := kindcluster.NewConfig(
		"e2e-v1-22-0-cluster",
		kindcluster.Options{
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
