package disruptors

import (
	"context"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"
	corev1 "k8s.io/api/core/v1"
)

// PodTerminationVisitor defines a Visitor that terminates its target pod
type PodTerminationVisitor struct {
	helper  helpers.PodHelper
	timeout time.Duration
}

// Visit executes a Terminate action on the target Pod
func (c PodTerminationVisitor) Visit(ctx context.Context, pod corev1.Pod) error {
	if c.timeout == 0 {
		c.timeout = 10 * time.Second
	}
	return c.helper.Terminate(ctx, pod.Name, c.timeout)
}

// PodFaultInjector defines methods for injecting faults into Pods
type PodFaultInjector interface {
	// Terminates a set of pods. Returns the list of pods affected. If any of the target pods
	// is not terminated after the timeout defined in the TerminatePodsFault, an error is returned
	TerminatePods(context.Context, TerminatePodsFault) ([]string, error)
}

// TerminatePodsFault specifies a fault that will terminate a set of pods
type TerminatePodsFault struct {
	// Count indicates how many pods to terminate
	Count int
	// Timeout specifies the maximum time to wait for a pod to terminate
	Timeout time.Duration
}
