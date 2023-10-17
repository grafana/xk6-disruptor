package disruptors

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/internal/version"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"

	corev1 "k8s.io/api/core/v1"
)

// PodVisitCommand define the commands used for visiting a Pod
type PodVisitCommand interface {
	// Exec defines the command to be executed
	Exec(corev1.Pod) ([]string, error)
	// Cleanup defines the command to execute for cleaning up if command execution fails
	Cleanup(corev1.Pod) []string
}

// PodController defines the interface for controlling a set of target Pods
type PodController struct {
	targets []corev1.Pod
	visitor PodVisitor
}

// PodAgentVisitorOptions defines the options for the PodVisitor
type PodAgentVisitorOptions struct {
	// Defines the timeout for injecting the agent
	Timeout time.Duration
}

// PodVisitor defines the methods for executing actions in a target Pod
type PodVisitor interface {
	Visit(context.Context, corev1.Pod, PodVisitCommand) error
}

// PodAgentVisitor executes actions in a Pod using the Agent
type PodAgentVisitor struct {
	helper    helpers.PodHelper
	namespace string
	options   PodAgentVisitorOptions
}

// NewPodAgentVisitor creates a new pod visitor
func NewPodAgentVisitor(helper helpers.PodHelper, namespace string, options PodAgentVisitorOptions) *PodAgentVisitor {
	// FIXME: handling timeout < 0  is required only to allow tests to skip waiting for the agent injection
	if options.Timeout == 0 {
		options.Timeout = 30 * time.Second
	}
	if options.Timeout < 0 {
		options.Timeout = 0
	}

	return &PodAgentVisitor{
		helper:    helper,
		namespace: namespace,
		options:   options,
	}
}

// injectDisruptorAgent injects the Disruptor agent in the target pods
func (c *PodAgentVisitor) injectDisruptorAgent(ctx context.Context, pod corev1.Pod) error {
	var (
		rootUser     = int64(0)
		rootGroup    = int64(0)
		runAsNonRoot = false
	)

	agentContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            "xk6-agent",
			Image:           version.AgentImage(),
			ImagePullPolicy: corev1.PullIfNotPresent,
			SecurityContext: &corev1.SecurityContext{
				Capabilities: &corev1.Capabilities{
					Add: []corev1.Capability{"NET_ADMIN"},
				},
				RunAsUser:    &rootUser,
				RunAsGroup:   &rootGroup,
				RunAsNonRoot: &runAsNonRoot,
			},
			TTY:   true,
			Stdin: true,
		},
	}

	return c.helper.AttachEphemeralContainer(
		ctx,
		pod.Name,
		agentContainer,
		helpers.AttachOptions{
			Timeout:        c.options.Timeout,
			IgnoreIfExists: true,
		},
	)
}

// Visit allows executing a different command on each target returned by a visiting function
func (c *PodAgentVisitor) Visit(ctx context.Context, pod corev1.Pod, commands PodVisitCommand) error {
	err := c.injectDisruptorAgent(ctx, pod)
	if err != nil {
		return fmt.Errorf("injecting agent in the pod %q: %w", pod.Name, err)
	}

	// get the command to execute in the target
	execCommand, err := commands.Exec(pod)
	if err != nil {
		return fmt.Errorf("unable to get command for pod %q: %w", pod.Name, err)
	}

	_, stderr, err := c.helper.Exec(ctx, pod.Name, "xk6-agent", execCommand, []byte{})

	// if command failed, ensure the agent execution is terminated
	cleanupCommand := commands.Cleanup(pod)
	if err != nil && cleanupCommand != nil {
		// we ignore errors because we are reporting the reason of the exec failure
		// we use a fresh context because the context used in exec may have been cancelled or expired
		//nolint:contextcheck
		_, _, _ = c.helper.Exec(context.TODO(), pod.Name, "xk6-agent", cleanupCommand, []byte{})
	}

	// if the context is cancelled, don't report error (we assume the caller is reporting this error)
	if err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("failed command execution for pod %q: %w \n%s", pod.Name, err, string(stderr))
	}

	return nil
}

// NewAgentController creates a new controller for a fleet of  pods
func NewAgentController(targets []corev1.Pod, visitor PodVisitor) *PodController {
	return &PodController{
		targets: targets,
		visitor: visitor,
	}
}

// Visit allows executing a different command on each target returned by a visiting function
func (c *PodController) Visit(ctx context.Context, commands PodVisitCommand) error {
	// if there are no targets, nothing to do
	if len(c.targets) == 0 {
		return nil
	}

	// create context for the visit, that can be cancelled in case of error
	visitCtx, cancelVisit := context.WithCancel(ctx)
	defer cancelVisit()

	// make space to prevent blocking go routines
	errCh := make(chan error, len(c.targets))

	wg := sync.WaitGroup{}
	for _, pod := range c.targets {
		wg.Add(1)
		go func(pod corev1.Pod) {
			if err := c.visitor.Visit(visitCtx, pod, commands); err != nil {
				errCh <- err
			}

			wg.Done()
		}(pod)
	}

	wg.Wait()

	select {
	case e := <-errCh:
		return e
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

// Targets return the name of the targets
func (c *PodController) Targets(_ context.Context) ([]string, error) {
	names := []string{}
	for _, pod := range c.targets {
		names = append(names, pod.Name)
	}

	return names, nil
}
