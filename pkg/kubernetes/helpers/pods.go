package helpers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/selection"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
)

// PodHelper defines helper methods for handling Pods
type PodHelper interface {
	// WaitPodRunning waits for the Pod to be running for up to given timeout and returns a boolean indicating
	// if the status was reached. If the pod is Failed returns error.
	WaitPodRunning(ctx context.Context, name string, timeout time.Duration) (bool, error)
	// Exec executes a non-interactive command described in options and returns the stdout and stderr outputs
	Exec(pod string, container string, command []string, stdin []byte) ([]byte, []byte, error)
	// AttachEphemeralContainer adds an ephemeral container to a running pod
	AttachEphemeralContainer(
		ctx context.Context,
		podName string,
		container corev1.EphemeralContainer,
		options AttachOptions,
	) error
	// List returns a list of pods that match the given PodFilter
	List(ctx context.Context, filter PodFilter) ([]string, error)
}

// helpers struct holds the data required by the helpers
type podHelper struct {
	config    *rest.Config
	client    kubernetes.Interface
	namespace string
}

// NewPodHelper returns a PodHelper
func NewPodHelper(client kubernetes.Interface, config *rest.Config, namespace string) PodHelper {
	return &podHelper{
		client:    client,
		config:    config,
		namespace: namespace,
	}
}

// PodFilter defines the criteria for selecting a pod for disruption
type PodFilter struct {
	// Select Pods that match these labels
	Select map[string]string
	// Select Pods that match these labels
	Exclude map[string]string
}

// AttachOptions defines options for attaching a container
type AttachOptions struct {
	// timeout for waiting until container is ready.
	Timeout time.Duration
	// If container exists, ignore and return
	IgnoreIfExists bool
}

// podConditionChecker defines a function that checks if a pod satisfies a condition
type podConditionChecker func(*corev1.Pod) (bool, error)

// waitForCondition watches a Pod in a namespace until a podConditionChecker is satisfied or a timeout expires
func (h *podHelper) waitForCondition(
	ctx context.Context,
	namespace string,
	name string,
	timeout time.Duration,
	checker podConditionChecker,
) (bool, error) {
	selector := fields.Set{
		"metadata.name": name,
	}.AsSelector()

	watcher, err := h.client.CoreV1().Pods(namespace).Watch(
		ctx,
		metav1.ListOptions{
			FieldSelector: selector.String(),
		},
	)
	if err != nil {
		return false, err
	}
	defer watcher.Stop()

	expired := time.After(timeout)
	for {
		select {
		case <-expired:
			return false, nil
		case event := <-watcher.ResultChan():
			if event.Type == watch.Error {
				return false, fmt.Errorf("error watching for pod: %v", event.Object)
			}
			if event.Type == watch.Modified {
				pod, isPod := event.Object.(*corev1.Pod)
				if !isPod {
					return false, errors.New("received unknown object while watching for pods")
				}
				condition, err := checker(pod)
				if condition || err != nil {
					return condition, err
				}
			}
		}
	}
}

func (h *podHelper) WaitPodRunning(ctx context.Context, name string, timeout time.Duration) (bool, error) {
	return h.waitForCondition(
		ctx,
		h.namespace,
		name,
		timeout,
		func(pod *corev1.Pod) (bool, error) {
			if pod.Status.Phase == corev1.PodFailed {
				return false, errors.New("pod has failed")
			}
			if pod.Status.Phase == corev1.PodRunning {
				return true, nil
			}
			return false, nil
		},
	)
}

func (h *podHelper) Exec(
	pod string,
	container string,
	command []string,
	stdin []byte,
) ([]byte, []byte, error) {
	req := h.client.CoreV1().RESTClient().
		Post().
		Namespace(h.namespace).
		Resource("pods").
		Name(pod).
		SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: container,
			Command:   command,
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       false,
		}, scheme.ParameterCodec)

	exec, err := remotecommand.NewSPDYExecutor(h.config, "POST", req.URL())
	if err != nil {
		return nil, nil, err
	}

	var stdout, stderr bytes.Buffer
	err = exec.Stream(remotecommand.StreamOptions{
		Stdin:  bytes.NewReader(stdin),
		Stdout: &stdout,
		Stderr: &stderr,
		Tty:    false,
	})

	return stdout.Bytes(), stderr.Bytes(), err
}

func (h *podHelper) AttachEphemeralContainer(
	ctx context.Context,
	podName string,
	container corev1.EphemeralContainer,
	options AttachOptions,
) error {
	pod, err := h.client.CoreV1().Pods(h.namespace).Get(
		ctx,
		podName,
		metav1.GetOptions{},
	)
	if err != nil {
		return fmt.Errorf("retrieving pod %q in %q: %w", podName, h.namespace, err)
	}

	// check if container already exists
	for _, c := range pod.Spec.EphemeralContainers {
		if c.Name == container.Name {
			if options.IgnoreIfExists {
				return nil
			}
			return fmt.Errorf("ephemeral container %s already exists", container.Name)
		}
	}

	podJSON, err := json.Marshal(pod)
	if err != nil {
		return fmt.Errorf("json marshalling pod %q: %w", pod.Name, err)
	}

	updatedPod := pod.DeepCopy()
	updatedPod.Spec.EphemeralContainers = append(updatedPod.Spec.EphemeralContainers, container)
	updateJSON, err := json.Marshal(updatedPod)
	if err != nil {
		return fmt.Errorf("json marshalling patched pod %q: %w", pod.Name, err)
	}

	patch, err := strategicpatch.CreateTwoWayMergePatch(podJSON, updateJSON, pod)
	if err != nil {
		return fmt.Errorf("creating ephemeral container patch for %q: %w", pod.Name, err)
	}

	_, err = h.client.CoreV1().Pods(h.namespace).Patch(
		ctx,
		pod.Name,
		types.StrategicMergePatchType,
		patch,
		metav1.PatchOptions{},
		"ephemeralcontainers",
	)
	if err != nil {
		return fmt.Errorf("patching ephemeral container into pod %q: %w", pod.Name, err)
	}

	if options.Timeout == 0 {
		return nil
	}
	running, err := h.waitForCondition(
		ctx,
		h.namespace,
		podName,
		options.Timeout,
		checkEphemeralContainerState,
	)
	if err != nil {
		return fmt.Errorf("waiting for ephemeral container of %q to start: %w", pod.Name, err)
	}
	if !running {
		return fmt.Errorf("ephemeral container for pod %q has not started after %fs", pod.Name, options.Timeout.Seconds())
	}
	return nil
}

func checkEphemeralContainerState(pod *corev1.Pod) (bool, error) {
	if pod.Status.EphemeralContainerStatuses != nil {
		for _, cs := range pod.Status.EphemeralContainerStatuses {
			if cs.State.Running != nil {
				return true, nil
			}
		}
	}

	return false, nil
}

// buildLabelSelector builds a label selector to be used in the k8s api, from a PodSelector
func buildLabelSelector(f PodFilter) (labels.Selector, error) {
	labelsSelector := labels.NewSelector()
	for label, value := range f.Select {
		req, err := labels.NewRequirement(label, selection.Equals, []string{value})
		if err != nil {
			return nil, err
		}
		labelsSelector = labelsSelector.Add(*req)
	}

	for label, value := range f.Exclude {
		req, err := labels.NewRequirement(label, selection.NotEquals, []string{value})
		if err != nil {
			return nil, err
		}
		labelsSelector = labelsSelector.Add(*req)
	}

	return labelsSelector, nil
}

func (h *podHelper) List(ctx context.Context, filter PodFilter) ([]string, error) {
	labelSelector, err := buildLabelSelector(filter)
	if err != nil {
		return nil, err
	}

	listOptions := metav1.ListOptions{
		LabelSelector: labelSelector.String(),
	}
	pods, err := h.client.CoreV1().Pods(h.namespace).List(
		ctx,
		listOptions,
	)
	if err != nil {
		return nil, err
	}

	names := []string{}
	for _, p := range pods.Items {
		names = append(names, p.Name)
	}

	return names, nil
}
