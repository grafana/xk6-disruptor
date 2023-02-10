// Package helpers offers functions to simplify dealing with kubernetes resources.
package helpers

import (
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Helpers offers Helper functions grouped by the objects they handle
type Helpers interface {
	NamespaceHelper
	ServiceHelper
	PodHelper
}

// helpers struct holds the data required by the helpers
type helpers struct {
	config    *rest.Config
	client    kubernetes.Interface
	namespace string
}

// NewHelper creates a set of helpers on the default namespace
func NewHelper(client kubernetes.Interface, config *rest.Config, namespace string) Helpers {
	return &helpers{
		client:    client,
		config:    config,
		namespace: namespace,
	}
}
