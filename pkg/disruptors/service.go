package disruptors

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"
)

// ServiceDisruptor defines operations for injecting faults in services
type ServiceDisruptor interface {
	Disruptor
	ProtocolFaultInjector
}

// ServiceDisruptorOptions defines options that controls the behavior of the ServiceDisruptor
type ServiceDisruptorOptions struct {
	// timeout when waiting agent to be injected (default 30s). A zero value forces default.
	// A Negative value forces no waiting.
	InjectTimeout time.Duration `js:"injectTimeout"`
}

// serviceDisruptor is an instance of a ServiceDisruptor
type serviceDisruptor struct {
	ctx        context.Context
	service    string
	namespace  string
	options    ServiceDisruptorOptions
	helper     helpers.ServiceHelper
	controller AgentController
}

// NewServiceDisruptor creates a new instance of a ServiceDisruptor that targets the given service
func NewServiceDisruptor(
	ctx context.Context,
	k8s kubernetes.Kubernetes,
	service string,
	namespace string,
	options ServiceDisruptorOptions,
) (ServiceDisruptor, error) {
	if service == "" {
		return nil, fmt.Errorf("must specify a service name")
	}

	helper := k8s.NamespacedHelpers(namespace)

	targets, err := helper.GetTargets(ctx, service)
	if err != nil {
		return nil, err
	}

	controller := NewAgentController(
		ctx,
		helper,
		namespace,
		targets,
		options.InjectTimeout,
	)

	err = controller.InjectDisruptorAgent()
	if err != nil {
		return nil, err
	}

	return &serviceDisruptor{
		ctx:        ctx,
		service:    service,
		namespace:  namespace,
		options:    options,
		controller: controller,
	}, nil
}

func (d *serviceDisruptor) InjectHTTPFaults(
	fault HTTPFault,
	duration time.Duration,
	options HTTPDisruptionOptions,
) error {
	targets, err := d.helper.MapPort(d.ctx, d.service, fault.Port)
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

func (d *serviceDisruptor) InjectGrpcFaults(
	fault GrpcFault,
	duration time.Duration,
	options GrpcDisruptionOptions,
) error {
	targets, err := d.helper.MapPort(d.ctx, d.service, fault.Port)
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
