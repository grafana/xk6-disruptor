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

// PodVisitor defines the interface for visiting Pods
type PodVisitor interface {
	// Visit returns the VisitComands for visiting the Pod
	Visit(pod corev1.Pod) (VisitCommands, error)
}

// VisitCommands define the commands used for visiting a Pod
type VisitCommands struct {
	// Exec defines the command to be executed
	Exec []string
	// Cleanup defines the command to execute for cleaning up if command execution fails
	Cleanup []string
}

// AgentFleet defines the interface for controlling agents in a set of targets
type AgentFleet struct {
	targets    []corev1.Pod
	controller *AgentController
}

// AgentControllerOptions defines the options for the AgentController
type AgentControllerOptions struct {
	// Defines the timeout for injecting the agent
	Timeout time.Duration
}

// AgentController controls de agents in a set of target pods
type AgentController struct {
	helper    helpers.PodHelper
	namespace string
	options   AgentControllerOptions
}

// NewAgentController creates a new controller
func NewAgentController(helper helpers.PodHelper, namespace string, options AgentControllerOptions) *AgentController {
	// FIXME: handling timeout < 0  is required only to allow tests to skip waiting for the agent injection
	if options.Timeout == 0 {
		options.Timeout = 30 * time.Second
	}
	if options.Timeout < 0 {
		options.Timeout = 0
	}

	return &AgentController{
		helper:    helper,
		namespace: namespace,
		options:   options,
	}
}

// InjectDisruptorAgent injects the Disruptor agent in the target pods
func (c *AgentController) InjectDisruptorAgent(ctx context.Context, pod corev1.Pod) error {
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
func (c *AgentController) Visit(ctx context.Context, pod corev1.Pod, visitor PodVisitor) error {
	// get the command to execute in the target
	visitCommands, err := visitor.Visit(pod)
	if err != nil {
		return fmt.Errorf("unable to get command for pod %q: %w", pod.Name, err)
	}

	_, stderr, err := c.helper.Exec(ctx, pod.Name, "xk6-agent", visitCommands.Exec, []byte{})

	// if command failed, ensure the agent execution is terminated
	if err != nil && visitCommands.Cleanup != nil {
		// we ignore errors because we are reporting the reason of the exec failure
		// we use a fresh context because the context used in exec may have been cancelled or expired
		//nolint:contextcheck
		_, _, _ = c.helper.Exec(context.TODO(), pod.Name, "xk6-agent", visitCommands.Cleanup, []byte{})
	}

	// if the context is cancelled, don't report error (we assume the caller is reporting this error)
	if err != nil && !errors.Is(err, context.Canceled) {
		return fmt.Errorf("failed command execution for pod %q: %w \n%s", pod.Name, err, string(stderr))
	}

	return nil
}

// NewAgentFleet creates a new controller for a fleet of  pods
func NewAgentFleet(targets []corev1.Pod, controller *AgentController) *AgentFleet {
	return &AgentFleet{
		targets:    targets,
		controller: controller,
	}
}

// Visit allows executing a different command on each target returned by a visiting function
func (c *AgentFleet) Visit(ctx context.Context, visitor PodVisitor) error {
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
			defer wg.Done()

			err := c.controller.InjectDisruptorAgent(ctx, pod)
			if err != nil {
				errCh <- fmt.Errorf("injecting agent in the pod %q: %w", pod.Name, err)
				return
			}

			if err := c.controller.Visit(visitCtx, pod, visitor); err != nil {
				errCh <- err
			}
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
func (c *AgentFleet) Targets(_ context.Context) ([]string, error) {
	names := []string{}
	for _, pod := range c.targets {
		names = append(names, pod.Name)
	}

	return names, nil
}
