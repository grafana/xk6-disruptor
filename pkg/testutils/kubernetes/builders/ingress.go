package builders

import (
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// IngressBuilder defines the interface for building Ingresses for a service backend
type IngressBuilder interface {
	// WithNamespace sets the namespace for the ingres
	WithNamespace(namespace string) IngressBuilder
	// WithClass sets the ingress class
	WithClass(class string) IngressBuilder
	// WithHost sets the host for the ingress rule
	WithHost(host string) IngressBuilder
	// WithPath sets the path for the ingress
	WithPath(path string) IngressBuilder
	// WithAnnotations add annotations to the Ingress
	WithAnnotation(key, value string) IngressBuilder
	// Build returns the Ingress
	Build() *networking.Ingress
}

// ingressBuilder maintains the configuration for building an Ingress
type ingressBuilder struct {
	service     string
	port        intstr.IntOrString
	namespace   string
	class       *string
	host        string
	path        string
	annotations map[string]string
}

// NewIngressBuilder creates a new IngressBuilder for a given serviceBackend
func NewIngressBuilder(service string, port intstr.IntOrString) IngressBuilder {
	return &ingressBuilder{
		service:     service,
		port:        port,
		namespace:   metav1.NamespaceDefault,
		annotations: map[string]string{},
		path:        "/",
	}
}

func (b *ingressBuilder) WithNamespace(namespace string) IngressBuilder {
	b.namespace = namespace
	return b
}

func (b *ingressBuilder) WithClass(class string) IngressBuilder {
	b.class = &class
	return b
}

func (b *ingressBuilder) WithAnnotation(key, value string) IngressBuilder {
	b.annotations[key] = value
	return b
}

func (b *ingressBuilder) WithPath(path string) IngressBuilder {
	b.path = path
	return b
}

func (b *ingressBuilder) WithHost(host string) IngressBuilder {
	b.host = host
	return b
}

func (b *ingressBuilder) Build() *networking.Ingress {
	pathType := networking.PathType("Prefix")

	return &networking.Ingress{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "networking/v1",
			Kind:       "Ingress",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.service,
			Namespace: b.namespace,
		},
		Spec: networking.IngressSpec{
			IngressClassName: b.class,
			Rules: []networking.IngressRule{
				{
					Host: b.host,
					IngressRuleValue: networking.IngressRuleValue{
						HTTP: &networking.HTTPIngressRuleValue{
							Paths: []networking.HTTPIngressPath{
								{
									Path:     b.path,
									PathType: &pathType,
									Backend: networking.IngressBackend{
										Service: &networking.IngressServiceBackend{
											Name: b.service,
											Port: networking.ServiceBackendPort{
												Name:   b.port.StrVal,
												Number: b.port.IntVal,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
