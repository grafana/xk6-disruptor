package disruptors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/internal/version"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"

	corev1 "k8s.io/api/core/v1"
)

// AgentController defines the interface for controlling agents in a set of targets
type AgentController interface {
	// InjectDisruptorAgent injects the Disruptor agent in the target pods
	InjectDisruptorAgent(ctx context.Context) error
	// ExecCommand executes a command in the targets of the AgentController and reports any error
	ExecCommand(ctx context.Context, cmd []string) error
	// Targets retrieves the names of the target of the controller
	Targets(ctx context.Context) ([]string, error)
	// Visit allows executing a different command on each target returned by a visiting function
	Visit(ctx context.Context, visitor func(target corev1.Pod) ([]string, error)) error
}

// AgentController controls de agents in a set of target pods
type agentController struct {
	helper    helpers.PodHelper
	namespace string
	targets   []corev1.Pod
	timeout   time.Duration
}

// InjectDisruptorAgent injects the Disruptor agent in the target pods
func (c *agentController) InjectDisruptorAgent(ctx context.Context) error {
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

	var wg sync.WaitGroup
	// ensure errors channel has enough space to avoid blocking gorutines
	errors := make(chan error, len(c.targets))
	for _, pod := range c.targets {
		wg.Add(1)
		// attach each container asynchronously
		go func(podName string) {
			defer wg.Done()

			err := c.helper.AttachEphemeralContainer(
				ctx,
				podName,
				agentContainer,
				helpers.AttachOptions{
					Timeout:        c.timeout,
					IgnoreIfExists: true,
				},
			)
			if err != nil {
				errors <- err
			}
		}(pod.Name)
	}

	wg.Wait()

	select {
	case err := <-errors:
		return err
	default:
		return nil
	}
}

// ExecCommand executes a command in the targets of the AgentController and reports any error
func (c *agentController) ExecCommand(ctx context.Context, cmd []string) error {
	// visit each target with the same command
	return c.Visit(ctx, func(corev1.Pod) ([]string, error) {
		return cmd, nil
	})
}

// Visit allows executing a different command on each target returned by a visiting function
func (c *agentController) Visit(_ context.Context, visitor func(corev1.Pod) ([]string, error)) error {
	var wg sync.WaitGroup
	// ensure errors channel has enough space to avoid blocking gorutines
	errors := make(chan error, len(c.targets))
	for _, pod := range c.targets {
		wg.Add(1)
		// attach each container asynchronously
		go func(pod corev1.Pod) {
			// get the command to execute in the target
			cmd, err := visitor(pod)
			if err != nil {
				errors <- fmt.Errorf("error building command for pod %s: %w", pod.Name, err)
			}

			_, stderr, err := c.helper.Exec(pod.Name, "xk6-agent", cmd, []byte{})
			if err != nil {
				errors <- fmt.Errorf("error invoking agent: %w \n%s", err, string(stderr))
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

// Targets retrieves the list of names of the target pods
func (c *agentController) Targets(_ context.Context) ([]string, error) {
	names := []string{}
	for _, p := range c.targets {
		names = append(names, p.Name)
	}
	return names, nil
}

// NewAgentController creates a new controller for a list of target pods
func NewAgentController(
	_ context.Context,
	helper helpers.PodHelper,
	namespace string,
	targets []corev1.Pod,
	timeout time.Duration,
) AgentController {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if timeout < 0 {
		timeout = 0
	}
	return &agentController{
		helper:    helper,
		namespace: namespace,
		targets:   targets,
		timeout:   timeout,
	}
}
