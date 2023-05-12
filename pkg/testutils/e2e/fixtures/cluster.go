package fixtures

import (
	"context"
	"fmt"
	"os"
	"path"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/cluster"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/fetch"
	"github.com/grafana/xk6-disruptor/pkg/testutils/e2e/kubectl"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PostInstall defines a function that runs after the cluster is created
// It can be used for adding components (e.g. addons)
type PostInstall func(ctx context.Context, cluster E2eCluster) error

// E2eClusterConfig defines the configuration of a e2e test cluster
type E2eClusterConfig struct {
	Name        string
	Images      []string
	IngressAddr string
	IngressPort int32
	PostInstall []PostInstall
	Reuse       bool
	Wait        time.Duration
	Kubeconfig  string
}

// E2eCluster defines the interface for accessing an e2e cluster
type E2eCluster interface {
	// Delete deletes the cluster
	Delete() error
	// Ingress returns the url to the cluster's ingress
	Ingress() string
	// Kubeconfig returns the path to the cluster's kubeconfig file
	Kubeconfig() string
	// Name returns the name of the cluster
	Name() string
}

const contourConfig = `
apiVersion: v1
kind: ConfigMap
metadata:
  name: contour
  namespace: projectcontour
data:
  contour.yaml: |
    disablePermitInsecure: false
    ingress-status-address: local.projectcontour.io

`

const contourBaseURL = "https://raw.githubusercontent.com/projectcontour/contour/main/examples/contour/"

// InstallContourIngress installs a customized contour ingress
func InstallContourIngress(ctx context.Context, cluster E2eCluster) error {
	manifests := []string{
		"00-common.yaml",
		"01-crds.yaml",
		"02-job-certgen.yaml",
		"02-rbac.yaml",
		"02-role-contour.yaml",
		"02-service-contour.yaml",
		"02-service-envoy.yaml",
		"03-contour.yaml",
		"03-envoy.yaml",
	}

	client, err := kubectl.NewFromKubeconfig(ctx, cluster.Kubeconfig())
	if err != nil {
		return err
	}

	// create contour resources
	for _, manifest := range manifests {
		url := contourBaseURL + manifest
		yaml, err2 := fetch.FromURL(url)
		if err2 != nil {
			return err2
		}

		err2 = client.Apply(ctx, string(yaml))
		if err2 != nil {
			return err2
		}
	}

	// apply custom configuration
	err = client.Apply(ctx, string(contourConfig))
	if err != nil {
		return err
	}

	return nil
}

// DefaultE2eClusterConfig builds the default configuration for an e2e test cluster
// TODO: allow override of default port using an environment variable (E2E_INGRESS_PORT)
func DefaultE2eClusterConfig() E2eClusterConfig {
	return E2eClusterConfig{
		Name:        "e2e-tests",
		Images:      []string{"ghcr.io/grafana/xk6-disruptor-agent:latest"},
		IngressAddr: "localhost",
		IngressPort: 30080,
		Reuse:       true,
		Kubeconfig:  path.Join(os.TempDir(),"e2e-tests"),
		Wait:        60 * time.Second,
		PostInstall: []PostInstall{
			InstallContourIngress,
		},
	}
}

// E2eClusterOption allows modifying an E2eClusterOption
type E2eClusterOption func(E2eClusterConfig) (E2eClusterConfig, error)

// WithIngressPort sets the ingress port
func WithIngressPort(port int32) E2eClusterOption {
	return func(c E2eClusterConfig) (E2eClusterConfig, error) {
		c.IngressPort = port
		return c, nil
	}
}

// WithIngressAddress sets the ingress address
func WithIngressAddress(address string) E2eClusterOption {
	return func(c E2eClusterConfig) (E2eClusterConfig, error) {
		c.IngressAddr = address
		return c, nil
	}
}

// WithName sets the cluster name
func WithName(name string) E2eClusterOption {
	return func(c E2eClusterConfig) (E2eClusterConfig, error) {
		c.Name = name
		return c, nil
	}
}

// WithWait sets the timeout for cluster creation
func WithWait(timeout time.Duration) E2eClusterOption {
	return func(c E2eClusterConfig) (E2eClusterConfig, error) {
		c.Wait = timeout
		return c, nil
	}
}

// WithKubeconfig sets the path for the kubeconfig
func WithKubeconfig(kubeconfig string) E2eClusterOption {
	return func(c E2eClusterConfig) (E2eClusterConfig, error) {
		c.Kubeconfig = kubeconfig
		return c, nil
	}
}

// e2eCluster maintains the status of a cluster
type e2eCluster struct {
	cluster *cluster.Cluster
	ingress string
	name    string
}


// creates a configmap in the cluster with the configuration
func createConfigMap(ctx context.Context, kubeconfig string, config E2eClusterConfig) error {
	k8s, err := kubernetes.NewFromKubeconfig(kubeconfig)
	if err != nil {
		return err
	}

	ingress := fmt.Sprintf("%s:%d", config.IngressAddr, config.IngressPort)

	cfm := builders.NewConfigMapBuilder(config.Name).
		WithDefaultNamespace().
		WithData("ingress", ingress).
		WithData("count", "1").
		Build()

	_, err = k8s.Client().CoreV1().ConfigMaps("default").Create(ctx, cfm, metav1.CreateOptions{})
	return err
}


// BuildE2eCluster builds a cluster for e2e tests
func BuildE2eCluster(e2eConfig E2eClusterConfig, ops ...E2eClusterOption) (E2eCluster, error) {
	var err error
	// apply option functions
	for _, option := range ops {
		e2eConfig, err = option(e2eConfig)
		if err != nil {
			return nil, err
		}
	}

	// Try to retrieve existing cluster, if fail, continue creating it
	// FIXME: we assume the ingress the same than in the passed e2e config.
	// We should retrieve the ingress from the config map
	if e2eConfig.Reuse {
		c, err := cluster.GetCluster(e2eConfig.Name, e2eConfig.Kubeconfig)

		if err == nil {
			return &e2eCluster{
				cluster: c,
				ingress: fmt.Sprintf("%s:%d", e2eConfig.IngressAddr, e2eConfig.IngressPort),
			}, nil
		}
	}

	config, err := cluster.NewConfig(
		e2eConfig.Name,
		cluster.Options{
			Images: e2eConfig.Images,
			Wait:   e2eConfig.Wait,
			NodePorts: []cluster.NodePort{
				{
					HostPort: e2eConfig.IngressPort,
					NodePort: 80,
				},
			},
			Kubeconfig: e2eConfig.Kubeconfig,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster config: %w", err)
	}

	c, err := config.Create()
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster: %w", err)
	}


	err = createConfigMap(context.TODO(), c.Kubeconfig(), e2eConfig)
	if err != nil {
		_ = c.Delete()
		return nil, fmt.Errorf("failed to create ConfigMap for cluster %s: %w", e2eConfig.Name, err)
	}

	ingress := fmt.Sprintf("%s:%d", e2eConfig.IngressAddr, e2eConfig.IngressPort)
	cluster := &e2eCluster{
		cluster: c,
		ingress: ingress,
		name:    e2eConfig.Name,
	}

	// TODO: set a deadline for the context passed to post install functions
	for _, postInstall := range e2eConfig.PostInstall {
		err = postInstall(context.TODO(), cluster)
		if err != nil {
			_ = cluster.Delete()
			return nil, err
		}
	}

	// FIXME: add some form of check to avoid fixed waits
	time.Sleep(e2eConfig.Wait)

	return cluster, nil
}

// BuildDefaultE2eCluster builds an e2e test cluster with the default configuration
func BuildDefaultE2eCluster() (E2eCluster, error) {
	return BuildE2eCluster(DefaultE2eClusterConfig())
}

func (c *e2eCluster) Delete() error {
	return c.cluster.Delete()
}

func (c *e2eCluster) Name() string {
	return c.name
}

func (c *e2eCluster) Ingress() string {
	return c.ingress
}

func (c *e2eCluster) Kubeconfig() string {
	return c.cluster.Kubeconfig()
}

// BuildCluster builds a cluster with the xk6-disruptor-agent image preloaded and
// the given node ports exposed
func BuildCluster(name string, ports ...cluster.NodePort) (*cluster.Cluster, error) {
	config, err := cluster.NewConfig(
		name,
		cluster.Options{
			Images:    []string{"ghcr.io/grafana/xk6-disruptor-agent:latest"},
			Wait:      time.Second * 60,
			NodePorts: ports,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create cluster config: %w", err)
	}

	return config.Create()
}
