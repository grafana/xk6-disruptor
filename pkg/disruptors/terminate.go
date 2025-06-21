package disruptors

import (
	"context"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"
	"github.com/grafana/xk6-disruptor/pkg/types/intstr"
	corev1 "k8s.io/api/core/v1"
)

// PodTerminationVisitor defines a Visitor that terminates its target pod
type PodTerminationVisitor struct {
	helper      helpers.PodHelper
	timeout     time.Duration
	gracePeriod time.Duration
	force       bool
}

// Visit executes a Terminate action on the target Pod
func (c PodTerminationVisitor) Visit(ctx context.Context, pod corev1.Pod) error {
	if c.timeout == 0 {
		c.timeout = 10 * time.Second
	}
	return c.helper.Terminate(ctx, pod.Name, c.timeout, c.gracePeriod, c.force)
}

// PodFaultInjector defines methods for injecting faults into Pods
type PodFaultInjector interface {
	// Terminates a set of pods. Returns the list of pods affected. If any of the target pods
	// is not terminated after the timeout defined in the TerminatePodsFault, an error is returned
	TerminatePods(context.Context, PodTerminationFault) ([]string, error)
}

// PodTerminationFault specifies a fault that will terminate a set of pods
type PodTerminationFault struct {
	// Count indicates how many pods to terminate. Can be a number or a percentage or targets
	Count intstr.IntOrString
	// Timeout specifies the maximum time to wait for a pod to terminate.
	// After the timeout, the fault injection completes with an error.
	Timeout time.Duration
	// GracePeriod specifies the grace period to wait for a pod to terminate.
	// After the grace period, the pod is terminated forcefully. Set to 0 to terminate immediately.
	GracePeriod time.Duration
	// Force specifies if the pod should be terminated forcefully.
	// GracePeriod is ignored if Force is true.
	Force bool
}
