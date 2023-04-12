package disruptors

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ServiceDisruptor defines operations for injecting faults in services
type ServiceDisruptor interface {
	PodDisruptor
}

// ServiceDisruptorOptions defines options that controls the behavior of the ServiceDisruptor
type ServiceDisruptorOptions struct {
	// timeout when waiting agent to be injected in seconds (default 30s). A zero value forces default.
	// A Negative value forces no waiting.
	InjectTimeout int `js:"injectTimeout"`
}

// serviceDisruptor is an instance of a ServiceDisruptor
type serviceDisruptor struct {
	ctx        context.Context
	name       string
	namespace  string
	options    ServiceDisruptorOptions
	service    *corev1.Service
	endpoints  *corev1.Endpoints
	k8s        kubernetes.Kubernetes
	mapper     PortMapper
	controller AgentController
}

// NewServiceDisruptor creates a new instance of a ServiceDisruptor that targets the given service
func NewServiceDisruptor(
	ctx context.Context,
	k8s kubernetes.Kubernetes,
	name string,
	namespace string,
	options ServiceDisruptorOptions,
) (ServiceDisruptor, error) {
	if name == "" {
		return nil, fmt.Errorf("must specify a service name")
	}
	service, err := k8s.CoreV1().
		Services(namespace).
		Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve target service %s: %w", service, err)
	}

	ep, err := k8s.CoreV1().Endpoints(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve endpoints for service %s: %w", service, err)
	}

	mapper := NewPortMapper(ctx, service, ep)

	targets := getTargetPods(ep)
	controller := NewAgentController(
		ctx,
		k8s,
		namespace,
		targets,
		time.Duration(options.InjectTimeout*int(time.Second)),
	)

	err = controller.InjectDisruptorAgent()
	if err != nil {
		return nil, err
	}

	return &serviceDisruptor{
		ctx:        ctx,
		name:       name,
		namespace:  namespace,
		options:    options,
		k8s:        k8s,
		service:    service,
		endpoints:  ep,
		mapper:     mapper,
		controller: controller,
	}, nil
}

// getTargetPods collects the name of all the target pods
func getTargetPods(ep *corev1.Endpoints) []string {
	var targets []string
	for _, subset := range ep.Subsets {
		for _, addr := range subset.Addresses {
			if addr.TargetRef.Kind == "Pod" {
				targets = append(targets, addr.TargetRef.Name)
			}
		}
	}

	return targets
}

// PortMapper maps ports of a service to a list of pod, port pairs
type PortMapper struct {
	ctx       context.Context
	service   *corev1.Service
	endpoints *corev1.Endpoints
}

// getTargetPort returns the service port that corresponds to the given port number
// if the given port is 0, it will return the default port or error if more than one port is defined
func (m *PortMapper) getTargetPort(port uint) (corev1.ServicePort, error) {
	ports := m.service.Spec.Ports
	if port != 0 {
		for _, p := range ports {
			if uint(p.Port) == port {
				return p, nil
			}
		}
		return corev1.ServicePort{}, fmt.Errorf("the service does not expose the given port: %d", port)
	}

	if len(ports) > 1 {
		return corev1.ServicePort{}, fmt.Errorf("service exposes multiple ports. Port option must be defined")
	}

	return ports[0], nil
}

// Map returns for a port in a service, the EndpointPort and the list pod, port pairs
func (m *PortMapper) Map(port uint) (map[string]uint, error) {
	targets := map[string]uint{}
	tp, err := m.getTargetPort(port)
	if err != nil {
		return targets, err
	}

	// iterate over sub-ranges looking for those that have the target port
	// and retrieve the name of the pods and the target por
	for _, subset := range m.endpoints.Subsets {
		for _, p := range subset.Ports {
			if p.Name == tp.Name {
				for _, addr := range subset.Addresses {
					if addr.TargetRef.Kind == "Pod" {
						targets[addr.TargetRef.Name] = uint(p.Port)
					}
				}
				break
			}
		}
	}

	return targets, nil
}

// NewPortMapper creates a new port mapper for a service
func NewPortMapper(
	ctx context.Context,
	service *corev1.Service,
	endpoints *corev1.Endpoints,
) PortMapper {
	return PortMapper{
		ctx:       ctx,
		service:   service,
		endpoints: endpoints,
	}
}

func (d *serviceDisruptor) InjectHTTPFaults(fault HTTPFault, duration uint, options HTTPDisruptionOptions) error {
	targets, err := d.mapper.Map(fault.Port)
	if err != nil {
		return fmt.Errorf("error getting target for fault injection: %w", err)
	}

	// for each target, the port to inject the fault can be different
	// we use the Visit function and generate a command for each pod
	err = d.controller.Visit(func(pod string) []string {
		// copy fault to change target port for the pod
		podFault := fault
		podFault.Port = targets[pod]
		cmd := buildHTTPFaultCmd(podFault, duration, options)
		return cmd
	})

	return err
}

func (d *serviceDisruptor) InjectGrpcFaults(fault GrpcFault, duration uint, options GrpcDisruptionOptions) error {
	targets, err := d.mapper.Map(fault.Port)
	if err != nil {
		return fmt.Errorf("error getting target for fault injection: %w", err)
	}

	// for each target, the port to inject the fault can be different
	// we use the Visit function and generate a command for each pod
	err = d.controller.Visit(func(pod string) []string {
		podFault := fault
		podFault.Port = targets[pod]
		cmd := buildGrpcFaultCmd(fault, duration, options)
		return cmd
	})

	return err
}

func (d *serviceDisruptor) Targets() ([]string, error) {
	return d.controller.Targets()
}
