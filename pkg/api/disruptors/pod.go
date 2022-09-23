// Package disruptors implements an API for disrupting targets
package disruptors

import (
	"fmt"
	"sync"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// PodAttributes defines the attributes a Pod must match for being selected/excluded
type PodAttributes struct {
	Labels map[string]string
}

// PodSelector defines the criteria for selecting a pod for disruption
type PodSelector struct {
	Namespace string
	// Select Pods that match these PodAttributes
	Select PodAttributes
	// Select Pods that match these PodAttributes
	Exclude PodAttributes
}

// HttpDisruptionOptions defines options for the injection of Http faults in a target pod
type HttpDisruptionOptions struct {
	TargetPort uint
	ProxyPort  uint
	Iface      string
}

// PodDisruptor defines the types of faults that can be injected in a Pod
type PodDisruptor interface {
	// Targets returns the list of targets for the disruptor
	Targets() ([]string, error)
	// InjectHttpFault injects faults in the http requests sent to the disruptor's targets
	// for the specified duration (in seconds)
	InjectHttpFaults(fault HttpFault, duration uint, options HttpDisruptionOptions) error
}

// PodDisruptorOptions defines options that controls the PodDisruptor's behavior
type PodDisruptorOptions struct {
	// timeout when waiting agent to be injected in seconds. 0 means no wait
	InjectTimeout uint
}

// podDisruptor is an instance of a PodDisruptor initialized with a list ot target pods
type podDisruptor struct {
	selector   PodSelector
	controller AgentController
	k8s        kubernetes.Kubernetes
	targets    []string
}

// buildLabelSelector builds a label selector to be used in the k8s api, from a PodSelector
func (s *PodSelector) buildLabelSelector() (labels.Selector, error) {
	labelsSelector := labels.NewSelector()
	for label, value := range s.Select.Labels {
		req, err := labels.NewRequirement(label, selection.Equals, []string{value})
		if err != nil {
			return nil, err
		}
		labelsSelector = labelsSelector.Add(*req)
	}

	for label, value := range s.Exclude.Labels {
		req, err := labels.NewRequirement(label, selection.NotEquals, []string{value})
		if err != nil {
			return nil, err
		}
		labelsSelector = labelsSelector.Add(*req)
	}

	return labelsSelector, nil
}

// getTargets retrieves the names of the targets of the disruptor
func (s *PodSelector) GetTargets(k8s kubernetes.Kubernetes) ([]string, error) {
	namespace := s.Namespace
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}

	labelSelector, err := s.buildLabelSelector()
	if err != nil {
		return nil, err
	}

	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	}
	pods, err := k8s.CoreV1().Pods(namespace).List(
		k8s.Context(),
		listOptions,
	)
	if err != nil {
		return nil, err
	}
	if len(pods.Items) == 0 {
		return nil, fmt.Errorf("no pods match the selector")
	}

	podNames := []string{}
	for _, pod := range pods.Items {
		podNames = append(podNames, pod.Name)
	}

	return podNames, nil
}

// agentController controls de agents in a set of target pods
type AgentController struct {
	k8s       kubernetes.Kubernetes
	namespace string
	targets   []string
	timeout   time.Duration
}

// InjectDisruptorAgent injects the Disruptor agent in the target pods
func (c *AgentController) InjectDisruptorAgent() error {
	agentContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            "xk6-agent",
			Image:           "grafana/xk6-disruptor-agent",
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"NET_ADMIN", "NET_RAW"},
				},
			},
			TTY:   true,
			Stdin: true,
		},
	}

	var wg sync.WaitGroup
	// ensure errors channel has enough space to avoid blocking gorutines
	errors := make(chan error, len(c.targets))
	for _, pod := range c.targets {
		wg.Add(1)
		// attach each container asynchronously
		go func(podName string) {
			defer wg.Done()

			// check if the container has already been injected
			pod, err := c.k8s.CoreV1().Pods(c.namespace).Get(c.k8s.Context(), podName, metav1.GetOptions{})
			if err != nil {
				errors <- err
				return
			}

			// if the container has already been injected, nothing to do
			for _, c := range pod.Spec.EphemeralContainers {
				if c.Name == agentContainer.Name {
					return
				}
			}

			err = c.k8s.NamespacedHelpers(c.namespace).AttachEphemeralContainer(
				podName,
				agentContainer,
				c.timeout,
			)

			if err != nil {
				errors <- err
			}
		}(pod)
	}

	wg.Wait()

	select {
	case err := <-errors:
		return err
	default:
		return nil
	}
}

// RunCommand executes a command in the targets of the AgentController and reports any error
func (c *AgentController) ExecCommand(cmd ...string) error {
	var wg sync.WaitGroup
	// ensure errors channel has enough space to avoid blocking gorutines
	errors := make(chan error, len(c.targets))
	for _, pod := range c.targets {
		wg.Add(1)
		// attach each container asynchronously
		go func(pod string) {
			_, _, err := c.k8s.NamespacedHelpers(c.namespace).
				Exec(pod, "xk6-agent", cmd, []byte{})
			if err != nil {
				errors <- err
			}

			wg.Done()
		}(pod)
	}

	wg.Wait()

	select {
	case err := <-errors:
		return err
	default:
		return nil
	}
}

// NewPodDisruptor creates a new instance of a PodDisruptor that acts on the pods
// that match the given PodSelector
func NewPodDisruptor(k8s kubernetes.Kubernetes, selector PodSelector, options PodDisruptorOptions) (PodDisruptor, error) {
	targets, err := selector.GetTargets(k8s)
	if err != nil {
		return nil, err
	}

	// ensure selector and controller use default namespace if none specified
	namespace := selector.Namespace
	if namespace == "" {
		namespace = metav1.NamespaceDefault
	}
	controller := AgentController{
		k8s:       k8s,
		namespace: namespace,
		targets:   targets,
		timeout:   time.Duration(options.InjectTimeout * uint(time.Second)),
	}
	err = controller.InjectDisruptorAgent()
	if err != nil {
		return nil, err
	}

	return &podDisruptor{
		selector:   selector,
		controller: controller,
		k8s:        k8s,
		targets:    targets,
	}, nil
}

// Targets retrieves the list of target pods for the given PodSelector
func (d *podDisruptor) Targets() ([]string, error) {
	return d.targets, nil
}

func buildHttpFaultCmd(fault HttpFault, duration uint, options HttpDisruptionOptions) ([]string, error) {
	cmd := []string{
		"xk6-disruptor-agent",
		"http",
		"-d", fmt.Sprintf("%ds", duration),
	}

	if fault.AverageDelay > 0 {
		cmd = append(cmd, "-a", fmt.Sprint(fault.AverageDelay), "-v", fmt.Sprint(fault.DelayVariation))
	}

	if fault.ErrorRate > 0 {
		cmd = append(cmd, "-e", fmt.Sprint(fault.ErrorCode), "-r", fmt.Sprint(fault.ErrorRate))
	}

	if options.ProxyPort != 0 {
		cmd = append(cmd, "-p", fmt.Sprint(options.ProxyPort))
	}

	if options.TargetPort != 0 {
		cmd = append(cmd, "-t", fmt.Sprint(options.TargetPort))
	}

	if options.Iface != "" {
		cmd = append(cmd, "-i", options.Iface)
	}

	return cmd, nil
}

//InjectHttpFault injects faults in the http requests sent to the disruptor's targets
func (d *podDisruptor) InjectHttpFaults(fault HttpFault, duration uint, options HttpDisruptionOptions) error {
	cmd, err := buildHttpFaultCmd(fault, duration, options)
	if err != nil {
		return err
	}

	err = d.controller.ExecCommand(cmd...)
	return err
}
