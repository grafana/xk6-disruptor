//go:build e2e
// +build e2e

package e2e

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/cluster"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/deploy"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/kubectl"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/kubernetes/namespace"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
)

func Test_Kubectl(t *testing.T) {
	t.Parallel()

	cluster, err := cluster.BuildE2eCluster(
		cluster.DefaultE2eClusterConfig(),
	)
	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}
	t.Cleanup(func() {
		_ = cluster.Cleanup()
	})

	k8s, err := kubernetes.NewFromKubeconfig(cluster.Kubeconfig())
	if err != nil {
		t.Errorf("error creating kubernetes client: %v", err)
		return
	}

	// Test Wait Pod Running
	t.Run("Test local random port", func(t *testing.T) {
		namespace, err := namespace.CreateTestNamespace(context.TODO(), t, k8s.Client())
		if err != nil {
			t.Errorf("failed to create test namespace: %v", err)
			return
		}

		// Deploy nginx
		nginx := builders.NewPodBuilder("nginx").
			WithContainer(
				builders.NewContainerBuilder("nginx").
					WithImage("nginx").
					WithPort("http", 80).
					Build(),
			).
			Build()

		err = deploy.RunPod(k8s, namespace, nginx, 20*time.Second)
		if err != nil {
			t.Errorf("failed to create test pod: %v", err)
			return
		}

		client, err := kubectl.NewFromKubeconfig(context.TODO(), cluster.Kubeconfig())
		if err != nil {
			t.Errorf("failed to create kubectl client: %v", err)
			return
		}

		ctx, stopper := context.WithCancel(context.TODO())
		// ensure por forwarder is cancelled
		defer stopper()

		port, err := client.ForwardPodPort(ctx, namespace, nginx.GetName(), 80)
		if err != nil {
			t.Errorf("failed to forward local port: %v", err)
			return
		}

		url := fmt.Sprintf("http://localhost:%d", port)
		request, err := http.NewRequest("GET", url, bytes.NewReader([]byte{}))
		if err != nil {
			t.Errorf("failed to create request: %v", err)
			return
		}

		resp, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Errorf("failed make request: %v", err)
			return
		}
		defer func() {
			_ = resp.Body.Close()
		}()

		if resp.StatusCode != http.StatusOK {
			t.Errorf("expected status code %d but %d received", http.StatusOK, resp.StatusCode)
			return
		}
	})
}
