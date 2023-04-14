package disruptors

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/internal/consts"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// AgentController defines the interface for controlling agents in a set of targets
type AgentController interface {
	// InjectDisruptorAgent injects the Disruptor agent in the target pods
	InjectDisruptorAgent() error
	// ExecCommand executes a command in the targets of the AgentController and reports any error
	ExecCommand(cmd []string) error
	// Targets returns the list of targets for the controller
	Targets() ([]string, error)
	// Visit allows executing a different command on each target returned by a visiting function
	Visit(func(target string) []string) error
}

// AgentController controls de agents in a set of target pods
type agentController struct {
	ctx       context.Context
	k8s       kubernetes.Kubernetes
	namespace string
	targets   []string
	timeout   time.Duration
}

// InjectDisruptorAgent injects the Disruptor agent in the target pods
// TODO: use the agent version that matches the extension version
func (c *agentController) InjectDisruptorAgent() error {
	var (
		rootUser     = int64(0)
		rootGroup    = int64(0)
		runAsNonRoot = false
	)

	agentContainer := corev1.EphemeralContainer{
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:            "xk6-agent",
			Image:           consts.AgentImage(),
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

			// check if the container has already been injected
			pod, err := c.k8s.CoreV1().Pods(c.namespace).Get(c.ctx, podName, metav1.GetOptions{})
			if err != nil {
				errors <- err
				return
			}

			// if the container has already been injected, nothing to do
			for _, c := range pod.Spec.EphemeralContainers {
				if c.Name == agentContainer.Name {
					return
				}
			}

			err = c.k8s.NamespacedHelpers(c.namespace).AttachEphemeralContainer(
				c.ctx,
				podName,
				agentContainer,
				c.timeout,
			)

			if err != nil {
				errors <- err
			}
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

// ExecCommand executes a command in the targets of the AgentController and reports any error
func (c *agentController) ExecCommand(cmd []string) error {
	// visit each target with the same command
	return c.Visit(func(string) []string {
		return cmd
	})
}

// Visit allows executing a different command on each target returned by a visiting function
func (c *agentController) Visit(visitor func(string) []string) error {
	var wg sync.WaitGroup
	// ensure errors channel has enough space to avoid blocking gorutines
	errors := make(chan error, len(c.targets))
	for _, pod := range c.targets {
		// get the command to execute in the target
		cmd := visitor(pod)
		wg.Add(1)
		// attach each container asynchronously
		go func(pod string) {
			_, stderr, err := c.k8s.NamespacedHelpers(c.namespace).
				Exec(pod, "xk6-agent", cmd, []byte{})
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

// Targets retrieves the list of target pods for the given PodSelector
func (c *agentController) Targets() ([]string, error) {
	return c.targets, nil
}

// NewAgentController creates a new controller for a list of target pods
func NewAgentController(
	ctx context.Context,
	k8s kubernetes.Kubernetes,
	namespace string,
	targets []string,
	timeout time.Duration,
) AgentController {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	if timeout < 0 {
		timeout = 0
	}
	return &agentController{
		ctx:       ctx,
		k8s:       k8s,
		namespace: namespace,
		targets:   targets,
		timeout:   timeout,
	}
}
