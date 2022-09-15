// Package disruptors implements an API for disrupting targets
package disruptors

import (
	"context"
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

// PodDisruptor defines the types of faults that can be injected in a Pod
type PodDisruptor interface {
	// Targets returns the list of targets for the disruptor
	Targets() ([]string, error)
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
		context.TODO(),
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
			Name:  "xk6-agent",
			Image: "grafana/xk6-disruptor-agent",
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
		go func(pod string) {
			err := c.k8s.NamespacedHelpers(c.namespace).AttachEphemeralContainer(
				pod,
				agentContainer,
				c.timeout,
			)

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
func NewPodDisruptor(k8s kubernetes.Kubernetes, selector PodSelector) (PodDisruptor, error) {
	targets, err := selector.GetTargets(k8s)
	if err != nil {
		return nil, err
	}

	controller := AgentController{
		k8s:       k8s,
		namespace: selector.Namespace,
		targets:   targets,
		timeout:   10 * time.Second, // FIXME: take from some configuration
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
