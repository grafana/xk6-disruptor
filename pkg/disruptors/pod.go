// Package disruptors implements an API for disrupting targets
package disruptors

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"
	"github.com/grafana/xk6-disruptor/pkg/utils"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DefaultTargetPort defines default target port if not specified in Fault
const DefaultTargetPort = 80

// PodDisruptor defines the types of faults that can be injected in a Pod
type PodDisruptor interface {
	Disruptor
	ProtocolFaultInjector
}

// PodDisruptorOptions defines options that controls the PodDisruptor's behavior
type PodDisruptorOptions struct {
	// timeout when waiting agent to be injected in seconds. A zero value forces default.
	// A Negative value forces no waiting.
	InjectTimeout time.Duration `js:"injectTimeout"`
}

// podDisruptor is an instance of a PodDisruptor initialized with a list of target pods
type podDisruptor struct {
	controller AgentController
	podFilter  helpers.PodFilter
	podHelper  helpers.PodHelper
}

// PodSelector defines the criteria for selecting a pod for disruption
type PodSelector struct {
	Namespace string
	// Select Pods that match these PodAttributes
	Select PodAttributes
	// Select Pods that match these PodAttributes
	Exclude PodAttributes
}

// PodAttributes defines the attributes a Pod must match for being selected/excluded
type PodAttributes struct {
	Labels map[string]string
}

// NewPodDisruptor creates a new instance of a PodDisruptor that acts on the pods
// that match the given PodSelector
func NewPodDisruptor(
	ctx context.Context,
	k8s kubernetes.Kubernetes,
	selector PodSelector,
	options PodDisruptorOptions,
) (PodDisruptor, error) {
	// validate selector
	emptySelect := reflect.DeepEqual(selector.Select, PodAttributes{})
	emptyExclude := reflect.DeepEqual(selector.Exclude, PodAttributes{})
	if selector.Namespace == "" && emptySelect && emptyExclude {
		return nil, fmt.Errorf("namespace, select and exclude attributes in pod selector cannot all be empty")
	}

	// ensure selector and controller use default namespace if none specified
	namespace := selector.Namespace
	if selector.Namespace == "" {
		selector.Namespace = metav1.NamespaceDefault
	}
	helper := k8s.PodHelper(namespace)

	filter := helpers.PodFilter{
		Select:  selector.Select.Labels,
		Exclude: selector.Exclude.Labels,
	}

	targets, err := helper.List(ctx, filter)
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
	err = controller.InjectDisruptorAgent(ctx)
	if err != nil {
		return nil, err
	}

	return &podDisruptor{
		controller: controller,
		podFilter:  filter,
		podHelper:  helper,
	}, nil
}

func (d *podDisruptor) Targets(ctx context.Context) ([]string, error) {
	return d.controller.Targets(ctx)
}

// InjectHTTPFault injects faults in the http requests sent to the disruptor's targets
func (d *podDisruptor) InjectHTTPFaults(
	ctx context.Context,
	fault HTTPFault,
	duration time.Duration,
	options HTTPDisruptionOptions,
) error {
	// TODO: make port mandatory instead of using a default
	if fault.Port == 0 {
		fault.Port = DefaultTargetPort
	}

	return d.controller.Visit(ctx, func(pod corev1.Pod) ([]string, error) {
		if !utils.HasPort(pod, fault.Port) {
			return nil, fmt.Errorf("pod %q does not expose port %d", pod.Name, fault.Port)
		}

		targetAddress, err := utils.PodIP(pod)
		if err != nil {
			return nil, err
		}

		cmd := buildHTTPFaultCmd(targetAddress, fault, duration, options)

		return cmd, nil
	})
}

// InjectGrpcFaults injects faults in the grpc requests sent to the disruptor's targets
func (d *podDisruptor) InjectGrpcFaults(
	ctx context.Context,
	fault GrpcFault,
	duration time.Duration,
	options GrpcDisruptionOptions,
) error {
	return d.controller.Visit(ctx, func(pod corev1.Pod) ([]string, error) {
		if !utils.HasPort(pod, fault.Port) {
			return nil, fmt.Errorf("pod %q does not expose port %d", pod.Name, fault.Port)
		}

		targetAddress, err := utils.PodIP(pod)
		if err != nil {
			return nil, err
		}

		cmd := buildGrpcFaultCmd(targetAddress, fault, duration, options)
		return cmd, nil
	})
}
