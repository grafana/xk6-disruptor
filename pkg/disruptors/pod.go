// Package disruptors implements an API for disrupting targets
package disruptors

import (
	"context"
	"fmt"
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

// PodDisruptor defines the types of faults that can be injected in a Pod
type PodDisruptor interface {
	// Targets returns the list of targets for the disruptor
	Targets() ([]string, error)
	// InjectHTTPFault injects faults in the HTTP requests sent to the disruptor's targets
	// for the specified duration (in seconds)
	InjectHTTPFaults(fault HTTPFault, duration uint, options HTTPDisruptionOptions) error
}

// PodDisruptorOptions defines options that controls the PodDisruptor's behavior
type PodDisruptorOptions struct {
	// timeout when waiting agent to be injected in seconds (default 30s). A zero value forces default.
	// A Negative value forces no waiting.
	InjectTimeout int `js:"injectTimeout"`
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

	timeout := options.InjectTimeout
	if timeout == 0 {
		timeout = 30
	}
	if timeout < 0 {
		timeout = 0
	}
	controller := &agentController{
		ctx:       ctx,
		k8s:       k8s,
		namespace: namespace,
		targets:   targets,
		timeout:   time.Duration(timeout * int(time.Second)),
	}
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

func buildHTTPFaultCmd(fault HTTPFault, duration uint, options HTTPDisruptionOptions) []string {
	cmd := []string{
		"xk6-disruptor-agent",
		"http",
		"-d", fmt.Sprintf("%ds", duration),
	}

	if fault.AverageDelay > 0 {
		cmd = append(cmd, "-a", fmt.Sprint(fault.AverageDelay), "-v", fmt.Sprint(fault.DelayVariation))
	}

	if fault.ErrorRate > 0 {
		cmd = append(
			cmd,
			"-e",
			fmt.Sprint(fault.ErrorCode),
			"-r",
			fmt.Sprint(fault.ErrorRate),
		)
		if fault.ErrorBody != "" {
			cmd = append(cmd, "-b", fault.ErrorBody)
		}
	}

	if fault.Port != 0 {
		cmd = append(cmd, "-t", fmt.Sprint(fault.Port))
	}

	if len(fault.Exclude) > 0 {
		cmd = append(cmd, "-x", fault.Exclude)
	}

	if options.ProxyPort != 0 {
		cmd = append(cmd, "-p", fmt.Sprint(options.ProxyPort))
	}

	if options.Iface != "" {
		cmd = append(cmd, "-i", options.Iface)
	}

	return cmd
}

// InjectHTTPFault injects faults in the http requests sent to the disruptor's targets
func (d *podDisruptor) InjectHTTPFaults(fault HTTPFault, duration uint, options HTTPDisruptionOptions) error {
	cmd := buildHTTPFaultCmd(fault, duration, options)

	err := d.controller.ExecCommand(cmd...)
	return err
}
