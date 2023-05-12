package builders

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)


// ConfigMapBuilder defines the interface for building ConfigMaps
type ConfigMapBuilder interface {
	// WithNamespace sets the namespace
	WithNamespace(namespace string) ConfigMapBuilder
	// WithDefaultNamespace sets the namespace to the default
	WithDefaultNamespace() ConfigMapBuilder
	// WithData adds a key,value pair to the ConfigMap's data
	WithData(key, value string) ConfigMapBuilder
	// Build returns the ConfigMap
	Build() *corev1.ConfigMap
}

type configMapBuilder struct {
	name      string
	namespace string
	data      map[string]string
}

// NewConfigMapBuilder returns a new ConfigMapBuilder
func NewConfigMapBuilder(name string) ConfigMapBuilder {
	return &configMapBuilder{
		name: name,
		data: map[string]string{},
	}
}

func (c *configMapBuilder) WithNamespace(namespace string) ConfigMapBuilder {
	c.namespace = namespace
	return c
}

func (c *configMapBuilder) WithDefaultNamespace() ConfigMapBuilder {
	c.namespace = metav1.NamespaceDefault
	return c
}

func (c *configMapBuilder) WithData(key, value string) ConfigMapBuilder {
	c.data[key] = value
	return c
}

func (c *configMapBuilder) Build() *corev1.ConfigMap {

	return &corev1.ConfigMap{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "ConfigMap",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        c.name,
			Namespace:   c.namespace,
		},
		Data: c.data,
	}
}