// Package builders offers functions for building test objects
package builders

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ServiceBuilder interface {
	// Build returns a Service with the attributes defined in the ServiceBuilder
	Build() *corev1.Service
	// WithNamespace sets namespace for the pod to be built
	WithNamespace(namespace string) ServiceBuilder
	// WithSelector sets the service's selector labels
	WithSelector(labels map[string]string) ServiceBuilder
}

// serviceBuilder defines the attributes for building a service
type serviceBuilder struct {
	name      string
	namespace string
	selector  map[string]string
}

// NewServiceBuilder creates a new instance of ServiceBuilder with the given pod name
// and default attributes
func NewServiceBuilder(name string) ServiceBuilder {
	return &serviceBuilder{
		name:      name,
		namespace: metav1.NamespaceDefault,
	}
}

func (s *serviceBuilder) WithNamespace(namespace string) ServiceBuilder {
	s.namespace = namespace
	return s
}

func (s *serviceBuilder) WithSelector(labels map[string]string) ServiceBuilder {
	s.selector = labels
	return s
}

func (s *serviceBuilder) Build() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      s.name,
			Namespace: s.namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: s.selector,
		},
	}
}
