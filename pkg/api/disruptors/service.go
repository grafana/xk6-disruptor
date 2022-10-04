package disruptors

import (
	"fmt"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceDisruptor defines operations for injecting faults in services
type ServiceDisruptor interface {
	// InjectHttpFault injects faults in the http requests sent to the disruptor's target
	// for the specified duration (in seconds)
	InjectHttpFaults(fault HttpFault, duration uint, options HttpDisruptionOptions) error
	// Targets returns the list of targets for the disruptor
	Targets() ([]string, error)
}

// ServiceDisruptorOptions defines options that controls the behavior of the ServiceDisruptor
type ServiceDisruptorOptions struct {
}

// serviceDisruptor is an instance of a ServiceDisruptor
type serviceDisruptor struct {
	k8s          kubernetes.Kubernetes
	options      ServiceDisruptorOptions
	podDisruptor PodDisruptor
}

// returns the labels used for the service for selecting the target pod(s)
func getServiceSelectorLabels(k8s kubernetes.Kubernetes, service string, namespace string) (map[string]string, error) {
	svc, err := k8s.CoreV1().
		Services(namespace).
		Get(k8s.Context(), service, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	return svc.Spec.Selector, nil
}

// NewServiceDisruptor creates a new instance of a ServiceDisruptor that targets the given service
func NewServiceDisruptor(k8s kubernetes.Kubernetes, service string, namespace string, options ServiceDisruptorOptions) (ServiceDisruptor, error) {
	svcSelector, err := getServiceSelectorLabels(k8s, service, namespace)
	if err != nil {
		return nil, fmt.Errorf("error retrieving service selector %w", err)
	}

	podSelector := PodSelector{
		Select: PodAttributes{
			Labels: svcSelector,
		},
	}

	podDisruptor, err := NewPodDisruptor(k8s, podSelector, PodDisruptorOptions{})
	if err != nil {
		return nil, fmt.Errorf("error creating pod disruptor %w", err)
	}

	return &serviceDisruptor{
		k8s:          k8s,
		options:      options,
		podDisruptor: podDisruptor,
	}, nil
}

func (d *serviceDisruptor) InjectHttpFaults(fault HttpFault, duration uint, options HttpDisruptionOptions) error {
	return d.podDisruptor.InjectHttpFaults(fault, duration, options)
}

func (d *serviceDisruptor) Targets() ([]string, error) {
	return d.podDisruptor.Targets()
}
