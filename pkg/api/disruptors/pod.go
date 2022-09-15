// Package disruptors implements an API for disrupting targets
package disruptors

import (
	"context"
	"fmt"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
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
	selector PodSelector
	k8s      kubernetes.Kubernetes
	targets  []string
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

// NewPodDisruptor creates a new instance of a PodDisruptor that acts on the pods
// that match the given PodSelector
func NewPodDisruptor(k8s kubernetes.Kubernetes, selector PodSelector) (PodDisruptor, error) {
	targets, err := selector.GetTargets(k8s)
	if err != nil {
		return nil, err
	}

	return &podDisruptor{
		selector: selector,
		k8s:      k8s,
		targets:  targets,
	}, nil
}

// Targets retrieves the list of target pods for the given PodSelector
func (d *podDisruptor) Targets() ([]string, error) {
	return d.targets, nil
}
