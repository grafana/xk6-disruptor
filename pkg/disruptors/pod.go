// Package disruptors implements an API for disrupting targets
package disruptors

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// DefaultTargetPort defines default target port if not specified in Fault
const DefaultTargetPort = 80

// ErrSelectorNoPods is returned by NewPodDisruptor when the selector passed to it does not match any pod in the
// cluster.
var ErrSelectorNoPods = errors.New("no pods found matching selector")

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
	helper     helpers.PodHelper
	options    PodDisruptorOptions
	controller *PodController
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

// NamespaceOrDefault returns the configured namespace for this selector, and the name of the default namespace if it
// is not configured.
func (p PodSelector) NamespaceOrDefault() string {
	if p.Namespace != "" {
		return p.Namespace
	}

	return metav1.NamespaceDefault
}

// String returns a human-readable explanation of the pods matched by a PodSelector.
func (p PodSelector) String() string {
	var str string

	if len(p.Select.Labels) == 0 && len(p.Exclude.Labels) == 0 {
		str = "all pods"
	} else {
		str = "pods "
		str += p.groupLabels("including", p.Select.Labels)
		str += p.groupLabels("excluding", p.Exclude.Labels)
		str = strings.TrimSuffix(str, ", ")
	}

	str += fmt.Sprintf(" in ns %q", p.NamespaceOrDefault())

	return str
}

// groupLabels returns a group of labels as a string, giving that group a name. The returned string has the form of:
// `groupName(foo=bar, boo=baz), `, including the trailing space and comma.
// An empty group of labels produces an empty string.
func (PodSelector) groupLabels(groupName string, labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}

	group := groupName + "("
	for k, v := range labels {
		group += fmt.Sprintf("%s=%s, ", k, v)
	}
	group = strings.TrimSuffix(group, ", ")
	group += "), "

	return group
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
	namespace := selector.NamespaceOrDefault()
	helper := k8s.PodHelper(namespace)

	filter := helpers.PodFilter{
		Select:  selector.Select.Labels,
		Exclude: selector.Exclude.Labels,
	}

	targets, err := helper.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	if len(targets) == 0 {
		return nil, fmt.Errorf("finding pods matching '%s': %w", selector, ErrSelectorNoPods)
	}

	controller := NewAgentController(targets)

	return &podDisruptor{
		helper:     helper,
		options:    options,
		controller: controller,
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

	command := PodHTTPFaultCommand{
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

// InjectGrpcFaults injects faults in the grpc requests sent to the disruptor's targets
func (d *podDisruptor) InjectGrpcFaults(
	ctx context.Context,
	fault GrpcFault,
	duration time.Duration,
	options GrpcDisruptionOptions,
) error {
	command := PodGrpcFaultCommand{
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
