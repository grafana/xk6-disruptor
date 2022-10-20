// Package kubernetes implements helper functions for manipulating resources in a
// Kubernetes cluster.
package kubernetes

import (
	"context"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"

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

// Config defines the configuration for creating a Kubernetes instance
type Config struct {
	// Context for executing kubernetes operations
	Context context.Context
	// Path to Kubernetes access configuration
	Kubeconfig string
}

// k8s Holds the reference to the helpers for interacting with kubernetes
type k8s struct {
	config *rest.Config
	kubernetes.Interface
	ctx context.Context
}

// NewFromKubeconfig returns a Kubernetes instance configured with the kubeconfig pointed by the given path
func NewFromKubeconfig(kubeconfig string) (Kubernetes, error) {
	return NewFromConfig(Config{
		Kubeconfig: kubeconfig,
	})
}

// NewFromConfig returns a Kubernetes instance configured with the given options
func NewFromConfig(c Config) (Kubernetes, error) {
	config, err := clientcmd.BuildConfigFromFlags("", c.Kubeconfig)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	ctx := c.Context
	if ctx == nil {
		ctx = context.TODO()
	}

	return &k8s{
		config:    config,
		Interface: client,
		ctx:       ctx,
	}, nil
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
