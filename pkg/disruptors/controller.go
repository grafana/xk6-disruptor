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

// AgentController defines the interface for controlling agents in a set of targets
type AgentController interface {
	// InjectDisruptorAgent injects the Disruptor agent in the target pods
	InjectDisruptorAgent(ctx context.Context) error
	// Targets retrieves the names of the target of the controller
	Targets(ctx context.Context) ([]string, error)
	// Visit allows executing a different command on each target returned by a visiting function
	Visit(ctx context.Context, visitor PodVisitor) error
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

// Visit allows executing a different command on each target returned by a visiting function
func (c *agentController) Visit(ctx context.Context, visitor PodVisitor) error {
	// if there are no targets, nothing to do
	if len(c.targets) == 0 {
		return nil
	}

	execContext, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ensure errCh channel has enough space to avoid blocking gorutines
	errCh := make(chan error, len(c.targets))
	for _, pod := range c.targets {
		pod := pod
		// visit each target asynchronously
		go func() {
			errCh <- func(pod corev1.Pod) error {
				// get the command to execute in the target
				visitCommands, err := visitor.Visit(pod)
				if err != nil {
					return fmt.Errorf("unable to get command for pod %q: %w", pod.Name, err)
				}

				_, stderr, err := c.helper.Exec(execContext, pod.Name, "xk6-agent", visitCommands.Exec, []byte{})

				// if command failed, ensure the agent execution is terminated
				if err != nil && visitCommands.Cleanup != nil {
					// we ignore errors because k6 was cancelled, so there's no point in reporting
					// use a fresh context because the exec context may have been cancelled or expired
					//nolint:contextcheck
					_, _, _ = c.helper.Exec(context.TODO(), pod.Name, "xk6-agent", visitCommands.Cleanup, []byte{})
				}

				// if the context is cancelled, it is reported in the main loop
				if err != nil && !errors.Is(err, context.Canceled) {
					return fmt.Errorf("failed command execution for pod %q: %w \n%s", pod.Name, err, string(stderr))
				}

				return nil
			}(pod)
		}()
	}

	var err error
	pending := len(c.targets)
	for {
		select {
		case e := <-errCh:
			pending--
			if e != nil {
				// cancel ongoing commands
				cancel()
				// Save first received error as reason for ending execution
				err = e
			}

			if pending == 0 {
				return err
			}
		case <-ctx.Done():
			// cancel ongoing commands
			cancel()
			// save the reason for ending execution
			err = ctx.Err()
		}
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
