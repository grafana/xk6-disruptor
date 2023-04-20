// Package disruptors implements an API for disrupting targets
package disruptors

import (
	"context"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PodAttributes defines the attributes a Pod must match for being selected/excluded
type PodAttributes struct {
	Labels map[string]string
}

// HTTPDisruptionOptions defines options for the injection of HTTP faults in a target pod
type HTTPDisruptionOptions struct {
	// Port used by the agent for listening
	ProxyPort uint `js:"proxyPort"`
	// Network interface the agent will be listening traffic from
	Iface string
}

// GrpcDisruptionOptions defines options for the injection of grpc faults in a target pod
type GrpcDisruptionOptions struct {
	// Port used by the agent for listening
	ProxyPort uint `js:"proxyPort"`
	// Network interface the agent will be listening traffic from
	Iface string
}

// PodDisruptor defines the types of faults that can be injected in a Pod
type PodDisruptor interface {
	// Targets returns the list of targets for the disruptor
	Targets() ([]string, error)
	// InjectHTTPFault injects faults in the HTTP requests sent to the disruptor's targets
	// for the specified duration
	InjectHTTPFaults(fault HTTPFault, duration time.Duration, options HTTPDisruptionOptions) error
	// InjectGrpcFault injects faults in the grpc requests sent to the disruptor's targets
	// for the specified duration
	InjectGrpcFaults(fault GrpcFault, duration time.Duration, options GrpcDisruptionOptions) error
}

// PodDisruptorOptions defines options that controls the PodDisruptor's behavior
type PodDisruptorOptions struct {
	// timeout when waiting agent to be injected in seconds. A zero value forces default.
	// A Negative value forces no waiting.
	InjectTimeout time.Duration `js:"injectTimeout"`
}

// podDisruptor is an instance of a PodDisruptor initialized with a list ot target pods
type podDisruptor struct {
	ctx        context.Context
	selector   PodSelector
	controller AgentController
}

// NewPodDisruptor creates a new instance of a PodDisruptor that acts on the pods
// that match the given PodSelector
func NewPodDisruptor(
	ctx context.Context,
	k8s kubernetes.Kubernetes,
	selector PodSelector,
	options PodDisruptorOptions,
) (PodDisruptor, error) {
	targets, err := selector.GetTargets(ctx, k8s)
	if err != nil {
		return nil, err
	}

	// ensure selector and controller use default namespace if none specified
	namespace := selector.Namespace
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}

	controller := NewAgentController(
		ctx,
		k8s,
		namespace,
		targets,
		options.InjectTimeout,
	)
	err = controller.InjectDisruptorAgent()
	if err != nil {
		return nil, err
	}

	return &podDisruptor{
		ctx:        ctx,
		selector:   selector,
		controller: controller,
	}, nil
}

// Targets retrieves the list of target pods for the given PodSelector
func (d *podDisruptor) Targets() ([]string, error) {
	return d.controller.Targets()
}

// InjectHTTPFault injects faults in the http requests sent to the disruptor's targets
func (d *podDisruptor) InjectHTTPFaults(
	fault HTTPFault,
	duration time.Duration,
	options HTTPDisruptionOptions,
) error {
	cmd := buildHTTPFaultCmd(fault, duration, options)

	err := d.controller.ExecCommand(cmd)
	return err
}

// InjectGrpcFaults injects faults in the grpc requests sent to the disruptor's targets
func (d *podDisruptor) InjectGrpcFaults(
	fault GrpcFault,
	duration time.Duration,
	options GrpcDisruptionOptions,
) error {
	cmd := buildGrpcFaultCmd(fault, duration, options)
	err := d.controller.ExecCommand(cmd)
	return err
}
