package builders

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodBuilder defines the methods for building a Pod
type PodBuilder interface {
	// Build returns a Pod with the attributes defined in the PodBuilder
	Build() *corev1.Pod
	// WithNamespace sets namespace for the pod to be built
	WithNamespace(namespace string) PodBuilder
	// WithLabels sets the labels for the pod to be built
	WithLabels(labels map[string]string) PodBuilder
	// WithStatus sets the PodPhase for the pod  to be built
	WithPhase(status corev1.PodPhase) PodBuilder
	// WithContainer add a container to the pod
	WithContainer(c corev1.Container) PodBuilder
}

// podBuilder defines the attributes for building a pod
type podBuilder struct {
	name       string
	namespace  string
	labels     map[string]string
	phase      corev1.PodPhase
	containers []corev1.Container
}

// NewPodBuilder creates a new instance of PodBuilder with the given pod name
// and default attributes such as containers and namespace
func NewPodBuilder(name string) PodBuilder {
	return &podBuilder{
		name: name,
	}
}

func (b *podBuilder) WithNamespace(namespace string) PodBuilder {
	b.namespace = namespace
	return b
}

func (b *podBuilder) WithPhase(phase corev1.PodPhase) PodBuilder {
	b.phase = phase
	return b
}

func (b *podBuilder) WithLabels(labels map[string]string) PodBuilder {
	b.labels = labels
	return b
}

func (b *podBuilder) WithContainer(c corev1.Container) PodBuilder {
	b.containers = append(b.containers, c)
	return b
}

func (b *podBuilder) Build() *corev1.Pod {
	return &corev1.Pod{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "Pod",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      b.name,
			Namespace: b.namespace,
			Labels:    b.labels,
		},
		Spec: corev1.PodSpec{
			Containers:          b.containers,
			EphemeralContainers: nil,
		},
		Status: corev1.PodStatus{
			Phase: b.phase,
		},
	}
}
