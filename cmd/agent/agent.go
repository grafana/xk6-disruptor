package main

import (
	"context"
	"fmt"
	"syscall"

	"github.com/grafana/xk6-disruptor/pkg/runtime"
	"github.com/spf13/cobra"
)

// AgentConfig maintains the configuration for the execution of the agent
type AgentConfig struct {
	profiler *runtime.ProfilerConfig
}

// Agent maintains the state required for executing an agent command
type Agent struct {
	env    runtime.Environment
	config *AgentConfig
}

// BuildAgent builds a instance of an agent
func BuildAgent(env runtime.Environment, config *AgentConfig) *Agent {
	return &Agent{
		env:    env,
		config: config,
	}
}

// Do executes a command in the Agent
func (r *Agent) Do(ctx context.Context, cmd *cobra.Command) error {
	sc := r.env.Signal().Notify(syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
	defer func() {
		r.env.Signal().Reset()
	}()

	acquired, err := r.env.Lock().Acquire()
	if err != nil {
		return fmt.Errorf("could not acquire process lock: %w", err)
	}
	if !acquired {
		return fmt.Errorf("another instance of the agent is already running")
	}

	defer func() {
		_ = r.env.Lock().Release()
	}()

	// start profiler
	profiler, err := r.env.Profiler().Start(*r.config.profiler)
	if err != nil {
		return fmt.Errorf("could not create profiler %w", err)
	}

	// ensure the profiler is closed even if there's an error executing the command
	defer func() {
		_ = profiler.Close()
	}()

	// set context for command
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	cmd.SetContext(ctx)

	// execute command in a goroutine to prevent blocking
	cc := make(chan error)
	go func() {
		cc <- cmd.Execute()
	}()

	// wait for command completion or cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-cc:
		return err
	case s := <-sc:
		return fmt.Errorf("received signal %q", s)
	}
}
