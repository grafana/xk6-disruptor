// Package agent implements functions for injecting faults in a target
package agent

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

// Config maintains the configuration for the execution of the agent
type Config struct {
	Profiler *runtime.ProfilerConfig
}

// Agent maintains the state required for executing an agent command
type Agent struct {
	env    runtime.Environment
	config *Config
}

// BuildAgent builds a instance of an agent
func BuildAgent(env runtime.Environment, config *Config) *Agent {
	return &Agent{
		env:    env,
		config: config,
	}
}

// ApplyDisruption applies a disruption to the target
func (r *Agent) ApplyDisruption(ctx context.Context, disruptor protocol.Disruptor, duration time.Duration) error {
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
	profiler, err := r.env.Profiler().Start(*r.config.Profiler)
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

	// execute action goroutine to prevent blocking
	cc := make(chan error)
	go func() {
		cc <- disruptor.Apply(ctx, duration)
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
