package builders

import (
	"fmt"
	"math/rand"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

// ServiceBuilder defines the methods for building a service
type ServiceBuilder interface {
	// Build returns a Service with the attributes defined in the ServiceBuilder
	Build() *corev1.Service
	// WithNamespace sets namespace for the pod to be built
	WithNamespace(namespace string) ServiceBuilder
	// WithPorts sets the ports exposed by the service
	WithPorts(ports []corev1.ServicePort) ServiceBuilder
	// WithSelector sets the service's selector labels
	WithSelector(labels map[string]string) ServiceBuilder
	// WithServiceType sets the type of the service (default is NodePort)
	WithServiceType(t corev1.ServiceType) ServiceBuilder
	// WithAnnotation adds an annotation to the service
	WithAnnotation(key string, value string) ServiceBuilder
}

// serviceBuilder defines the attributes for building a service
type serviceBuilder struct {
	name        string
	namespace   string
	serviceType corev1.ServiceType
	ports       []corev1.ServicePort
	selector    map[string]string
	annotations map[string]string
}

// DefaultServicePorts returns an array of ServicePort with default values
func DefaultServicePorts() []corev1.ServicePort {
	return []corev1.ServicePort{
		{
			Port:       80,
			TargetPort: intstr.FromInt(80),
		},
	}
}

// NewServiceBuilder creates a new instance of ServiceBuilder with the given pod name
// and default attributes
func NewServiceBuilder(name string) ServiceBuilder {
	return &serviceBuilder{
		name:        name,
		serviceType: corev1.ServiceTypeNodePort,
		ports:       DefaultServicePorts(),
		selector:    map[string]string{},
		annotations: map[string]string{},
	}
}

func (s *serviceBuilder) WithNamespace(namespace string) ServiceBuilder {
	s.namespace = namespace
	return s
}

func (s *serviceBuilder) WithPorts(ports []corev1.ServicePort) ServiceBuilder {
	s.ports = ports
	return s
}

func (s *serviceBuilder) WithServiceType(serviceType corev1.ServiceType) ServiceBuilder {
	s.serviceType = serviceType
	return s
}

func (s *serviceBuilder) WithSelector(labels map[string]string) ServiceBuilder {
	s.selector = labels
	return s
}

func (s *serviceBuilder) WithAnnotation(key string, value string) ServiceBuilder {
	s.annotations[key] = value
	return s
}

func (s *serviceBuilder) Build() *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:        s.name,
			Namespace:   s.namespace,
			Annotations: s.annotations,
		},
		Spec: corev1.ServiceSpec{
			Selector: s.selector,
			Type:     s.serviceType,
			Ports:    s.ports,
		},
	}
}

// EndpointsBuilder defines the methods for building a service EndPoints
type EndpointsBuilder interface {
	// WithNamespace sets namespace for the pod to be built
	WithNamespace(namespace string) EndpointsBuilder
	// WithSubset adds a subset to the Endpoints
	WithSubset(ports []corev1.EndpointPort, pods []string) EndpointsBuilder
	// Build builds the Endpoints
	Build() *corev1.Endpoints
}

type endpointsBuilder struct {
	service   string
	namespace string
	subsets   []corev1.EndpointSubset
}

// NewEndPointsBuilder creates a new EndpointsBuilder for a given service
func NewEndPointsBuilder(service string) EndpointsBuilder {
	return &endpointsBuilder{
		service: service,
		subsets: []corev1.EndpointSubset{},
	}
}

func (b *endpointsBuilder) WithNamespace(namespace string) EndpointsBuilder {
	b.namespace = namespace
	return b
}

func randomIP() string {
	b := make([]byte, 4)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%d.%d.%d.%d", b[0], b[1], b[2], b[3])
}

func (b *endpointsBuilder) WithSubset(ports []corev1.EndpointPort, pods []string) EndpointsBuilder {
	addresses := []corev1.EndpointAddress{}
	for _, p := range pods {
		addresses = append(
			addresses,
			corev1.EndpointAddress{
				IP: randomIP(),
				TargetRef: &corev1.ObjectReference{
					Kind:      "Pod",
					Namespace: b.namespace,
					Name:      p,
				},
			},
		)
	}

	subset := corev1.EndpointSubset{
		Ports:     ports,
		Addresses: addresses,
	}
	b.subsets = append(b.subsets, subset)

	return b
}

func (b *endpointsBuilder) Build() *corev1.Endpoints {
	return &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "EndPoints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.service,
			Namespace: b.namespace,
		},
		Subsets: b.subsets,
	}
}
