package disruptors

import (
	"context"
	"fmt"
	"reflect"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
)

// PodSelector defines the criteria for selecting a pod for disruption
type PodSelector struct {
	Namespace string
	// Select Pods that match these PodAttributes
	Select PodAttributes
	// Select Pods that match these PodAttributes
	Exclude PodAttributes
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

// GetTargets retrieves the names of the targets of the disruptor
func (s *PodSelector) GetTargets(ctx context.Context, k8s kubernetes.Kubernetes) ([]string, error) {
	// validate selector
	emptySelect := reflect.DeepEqual(s.Select, PodAttributes{})
	emptyExclude := reflect.DeepEqual(s.Exclude, PodAttributes{})
	if s.Namespace == "" && emptySelect && emptyExclude {
		return nil, fmt.Errorf("namespace, select and exclude attributes in pod selector cannot all be empty")
	}

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
		ctx,
		listOptions,
	)
	if err != nil {
		return nil, err
	}

	podNames := []string{}
	for _, pod := range pods.Items {
		podNames = append(podNames, pod.Name)
	}

	return podNames, nil
}
