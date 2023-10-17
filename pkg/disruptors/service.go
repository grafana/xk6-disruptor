package disruptors

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ErrServiceNoTargets is returned by NewServiceDisruptor when passed a service without any pod matching its selector.
var ErrServiceNoTargets = errors.New("service does not have any backing pods")

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
	service    corev1.Service
	helper     helpers.PodHelper
	options    ServiceDisruptorOptions
	controller *PodController
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

	svc, err := k8s.Client().CoreV1().Services(namespace).Get(ctx, service, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	sh := k8s.ServiceHelper(namespace)

	targets, err := sh.GetTargets(ctx, service)
	if err != nil {
		return nil, err
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("creating disruptor for service %s/%s: %w", service, namespace, ErrServiceNoTargets)
	}

	helper := k8s.PodHelper(namespace)

	controller := NewAgentController(targets)

	return &serviceDisruptor{
		service:    *svc,
		helper:     helper,
		options:    options,
		controller: controller,
	}, nil
}

func (d *serviceDisruptor) InjectHTTPFaults(
	ctx context.Context,
	fault HTTPFault,
	duration time.Duration,
	options HTTPDisruptionOptions,
) error {
	command := ServiceHTTPFaultCommand{
		service:  d.service,
		fault:    fault,
		duration: duration,
		options:  options,
	}

	visitor := NewPodAgentVisitor(
		d.helper,
		PodAgentVisitorOptions{Timeout: d.options.InjectTimeout},
		command,
	)

	return d.controller.Visit(ctx, visitor)
}

func (d *serviceDisruptor) InjectGrpcFaults(
	ctx context.Context,
	fault GrpcFault,
	duration time.Duration,
	options GrpcDisruptionOptions,
) error {
	command := ServiceGrpcFaultCommand{
		service:  d.service,
		fault:    fault,
		duration: duration,
		options:  options,
	}

	visitor := NewPodAgentVisitor(
		d.helper,
		PodAgentVisitorOptions{Timeout: d.options.InjectTimeout},
		command,
	)

	return d.controller.Visit(ctx, visitor)
}

func (d *serviceDisruptor) Targets(ctx context.Context) ([]string, error) {
	return d.controller.Targets(ctx)
}
