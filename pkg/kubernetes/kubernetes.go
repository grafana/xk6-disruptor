// Package kubernetes implements helper functions for manipulating resources in a
// Kubernetes cluster.
package kubernetes

import (
	"context"
	"errors"
	"fmt"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"

	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

// Kubernetes defines an interface that extends kubernetes interface[k8s.io/client-go/kubernetes.Interface]
// Adding helper functions for common tasks
type Kubernetes interface {
	kubernetes.Interface
	Context() context.Context
	Helpers() helpers.Helpers
	NamespacedHelpers(namespace string) helpers.Helpers
}

// k8s Holds the reference to the helpers for interacting with kubernetes
type k8s struct {
	config *rest.Config
	kubernetes.Interface
	ctx context.Context
}

// newFromConfig returns a Kubernetes instance configured with the provided kubeconfig.
func newFromConfig(ctx context.Context, config *rest.Config) (Kubernetes, error) {
	// As per the discussion in [1] client side rate limiting is no longer required.
	// Setting a large limit
	// [1] https://github.com/kubernetes/kubernetes/issues/111880
	config.QPS = 100
	config.Burst = 150

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	err = checkK8sVersion(config)
	if err != nil {
		return nil, err
	}

	return &k8s{
		config:    config,
		Interface: client,
		ctx:       ctx,
	}, nil
}

// NewFromKubeconfig returns a Kubernetes instance configured with the kubeconfig pointed by the given path
func NewFromKubeconfig(ctx context.Context, kubeconfig string) (Kubernetes, error) {
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, err
	}

	return newFromConfig(ctx, config)
}

// New returns a Kubernetes instance or an error when no config is eligible to be used.
// there are three ways of loading the kubernetes config, using the order as they are described below
// 1. in-cluster config, from serviceAccount token.
// 2. KUBECONFIG environment variable.
// 3. $HOME/.kube/config file.
func New(ctx context.Context) (Kubernetes, error) {
	k8sConfig, err := rest.InClusterConfig()
	if err == nil {
		return newFromConfig(ctx, k8sConfig)
	}

	if !errors.Is(err, rest.ErrNotInCluster) {
		return nil, err
	}

	kubeConfigPath, getConfigErr := getConfigPath()
	if getConfigErr != nil {
		return nil, fmt.Errorf("error getting kubernetes config path: %w", getConfigErr)
	}

	return NewFromKubeconfig(ctx, kubeConfigPath)
}

func checkK8sVersion(config *rest.Config) error {
	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return err
	}

	version, err := discoveryClient.ServerVersion()
	if err != nil {
		return err
	}

	semver := fmt.Sprintf("v%s.%s", version.Major, version.Minor)
	// TODO: implement proper semver check
	if semver < "v1.23" {
		return fmt.Errorf("unsupported Kubernetes version. Expected >= v1.23 but actual is %s", semver)
	}
	return nil
}

// Returns the context for executing k8s actions
func (k *k8s) Context() context.Context {
	return k.ctx
}

// Helpers returns Helpers for the default namespace
func (k *k8s) Helpers() helpers.Helpers {
	return helpers.NewHelper(
		k.ctx,
		k.Interface,
		k.config,
		"default",
	)
}

// NamespacedHelpers returns helpers for the given namespace
func (k *k8s) NamespacedHelpers(namespace string) helpers.Helpers {
	return helpers.NewHelper(
		k.ctx,
		k.Interface,
		k.config,
		namespace,
	)
}
