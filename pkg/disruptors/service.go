package disruptors

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/utils"

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

	ph := k8s.PodHelper(namespace)
	controller := NewAgentController(
		ctx,
		ph,
		namespace,
		targets,
		options.InjectTimeout,
	)

	err = controller.InjectDisruptorAgent(ctx)
	if err != nil {
		return nil, err
	}

	return &serviceDisruptor{
		service:    *svc,
		controller: controller,
	}, nil
}

func (d *serviceDisruptor) InjectHTTPFaults(
	ctx context.Context,
	fault HTTPFault,
	duration time.Duration,
	options HTTPDisruptionOptions,
) error {
	// for each target, the port to inject the fault can be different
	// we use the Visit function and generate a command for each pod
	return d.controller.Visit(ctx, func(pod corev1.Pod) (VisitCommands, error) {
		port, err := utils.MapPort(d.service, fault.Port, pod)
		if err != nil {
			return VisitCommands{}, err
		}

		if utils.HasHostNetwork(pod) {
			return VisitCommands{}, fmt.Errorf("pod %q cannot be safely injected as it has hostNetwork set to true", pod.Name)
		}

		targetAddress, err := utils.PodIP(pod)
		if err != nil {
			return VisitCommands{}, err
		}

		// copy fault to change target port for the pod
		podFault := fault
		podFault.Port = port
		visitCommands := VisitCommands{
			Exec:    buildHTTPFaultCmd(targetAddress, podFault, duration, options),
			Cleanup: buildCleanupCmd(),
		}

		return visitCommands, nil
	})
}

func (d *serviceDisruptor) InjectGrpcFaults(
	ctx context.Context,
	fault GrpcFault,
	duration time.Duration,
	options GrpcDisruptionOptions,
) error {
	// for each target, the port to inject the fault can be different
	// we use the Visit function and generate a command for each pod
	return d.controller.Visit(ctx, func(pod corev1.Pod) (VisitCommands, error) {
		port, err := utils.MapPort(d.service, fault.Port, pod)
		if err != nil {
			return VisitCommands{}, err
		}

		if utils.HasHostNetwork(pod) {
			return VisitCommands{}, fmt.Errorf("pod %q cannot be safely injected as it has hostNetwork set to true", pod.Name)
		}

		targetAddress, err := utils.PodIP(pod)
		if err != nil {
			return VisitCommands{}, err
		}

		// copy fault to change target port for the pod
		podFault := fault
		podFault.Port = port
		visitCommands := VisitCommands{
			Exec:    buildGrpcFaultCmd(targetAddress, podFault, duration, options),
			Cleanup: buildCleanupCmd(),
		}

		return visitCommands, nil
	})
}

func (d *serviceDisruptor) Targets(ctx context.Context) ([]string, error) {
	return d.controller.Targets(ctx)
}
