// Package helpers offers functions to simplify dealing with kubernetes resources.
package helpers

import (
	"context"

	"k8s.io/client-go/kubernetes"
)

// Helpers offers Helper functions grouped by the objects they handle
type Helpers interface {
	NamespaceHelper
	ServiceHelper
}

// helpers struct holds the data required by the helpers
type helpers struct {
	client    kubernetes.Interface
	namespace string
	ctx       context.Context
}

// NewHelpers creates a set of helpers on the default namespace
func NewHelper(c kubernetes.Interface, namespace string) Helpers {
	return &helpers{
		client:    c,
		namespace: namespace,
		ctx:       context.TODO(),
	}
}
