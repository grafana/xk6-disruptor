package disruptors

import (
	"fmt"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceDisruptor defines operations for injecting faults in services
type ServiceDisruptor interface {
	// InjectHttpFault injects faults in the http requests sent to the disruptor's target
	// for the specified duration (in seconds)
	InjectHttpFaults(fault HttpFault, duration uint) error
	// Targets returns the list of targets for the disruptor
	Targets() ([]string, error)
}

// ServiceDisruptorOptions defines options that controls the behavior of the ServiceDisruptor
type ServiceDisruptorOptions struct {
	// timeout when waiting agent to be injected in seconds (default 30s). A zero value forces default. A Negative value forces no waiting.
	InjectTimeout int
	// port used by the agent's proxy
	ProxyPort uint
	// interface the agent's proxy will be receiving traffic from
	Iface string
}

// serviceDisruptor is an instance of a ServiceDisruptor
type serviceDisruptor struct {
	service      string
	namespace    string
	k8s          kubernetes.Kubernetes
	options      ServiceDisruptorOptions
	httpOptions  HttpDisruptionOptions
	podDisruptor PodDisruptor
}

func getTargetPort(ports []corev1.ServicePort, port uint) (uint, error) {
	var targetPort uint
	if port != 0 {
		for _, p := range ports {
			if uint(p.Port) == port {
				targetPort = uint(p.TargetPort.IntVal)
				return targetPort, nil
			}
		}
		return 0, fmt.Errorf("the service does not expose the given port: %d", port)
	} else {
		if len(ports) > 1 {
			return 0, fmt.Errorf("service exposes multiple ports. Port option must be defined")
		}

		targetPort = uint(ports[0].TargetPort.IntVal)
		return targetPort, nil
	}
}

// NewServiceDisruptor creates a new instance of a ServiceDisruptor that targets the given service
func NewServiceDisruptor(k8s kubernetes.Kubernetes, service string, namespace string, options ServiceDisruptorOptions) (ServiceDisruptor, error) {
	svc, err := k8s.CoreV1().
		Services(namespace).
		Get(k8s.Context(), service, metav1.GetOptions{})

	if err != nil {
		return nil, err
	}

	podSelector := PodSelector{
		Namespace: svc.Namespace,
		Select: PodAttributes{
			Labels: svc.Spec.Selector,
		},
	}

	podOpts := PodDisruptorOptions{
		InjectTimeout: options.InjectTimeout,
	}

	podDisruptor, err := NewPodDisruptor(k8s, podSelector, podOpts)
	if err != nil {
		return nil, fmt.Errorf("error creating pod disruptor %w", err)
	}

	httpOptions := HttpDisruptionOptions{
		ProxyPort: options.ProxyPort,
		Iface:     options.Iface,
	}

	return &serviceDisruptor{
		service:      service,
		namespace:    namespace,
		k8s:          k8s,
		options:      options,
		httpOptions:  httpOptions,
		podDisruptor: podDisruptor,
	}, nil
}

func (d *serviceDisruptor) InjectHttpFaults(fault HttpFault, duration uint) error {
	svc, err := d.k8s.CoreV1().
		Services(d.namespace).
		Get(d.k8s.Context(), d.service, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// Get the target port for the service. This port will be used when injecting http faults in the target pods
	port, err := getTargetPort(svc.Spec.Ports, fault.Port)
	if err != nil {
		return err
	}

	fault.Port = port
	return d.podDisruptor.InjectHttpFaults(fault, duration, d.httpOptions)
}

func (d *serviceDisruptor) Targets() ([]string, error) {
	return d.podDisruptor.Targets()
}
