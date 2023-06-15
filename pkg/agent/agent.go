// Package agent implements functions for injecting faults in a target
package agent

import (
	"context"
	"fmt"
	"syscall"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol/grpc"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol/http"
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

// do executes a command in the Agent
func (r *Agent) do(ctx context.Context, action func(context.Context) error) error {
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
		cc <- action(ctx)
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

// ApplyHTTPDisruption applies a disruption to the gRPC requests serviced by the target
func (r *Agent) ApplyHTTPDisruption(
	ctx context.Context,
	proxyConfig http.ProxyConfig,
	disruption http.Disruption,
	config protocol.DisruptorConfig,
	transparent bool,
	duration time.Duration,
) error {
	proxy, err := http.NewProxy(proxyConfig, disruption)
	if err != nil {
		return err
	}

	return r.applyDisruption(
		ctx,
		proxy,
		config,
		transparent,
		duration,
	)
}

// ApplyGrpcDisruption applies a disruption to the gRPC requests serviced by the target
func (r *Agent) ApplyGrpcDisruption(
	ctx context.Context,
	proxyConfig grpc.ProxyConfig,
	disruption grpc.Disruption,
	config protocol.DisruptorConfig,
	transparent bool,
	duration time.Duration,
) error {
	proxy, err := grpc.NewProxy(proxyConfig, disruption)
	if err != nil {
		return err
	}

	return r.applyDisruption(
		ctx,
		proxy,
		config,
		transparent,
		duration,
	)
}

func (r *Agent) applyDisruption(
	ctx context.Context,
	proxy protocol.Proxy,
	config protocol.DisruptorConfig,
	transparent bool,
	duration time.Duration,
) error {
	// if not transparent run as a regular proxy
	if !transparent {
		// TODO: pass a context with a timeout using the duration argument
		return r.do(ctx, func(ctx context.Context) error {
			return proxy.Start()
		})
	}

	disruptor, err := protocol.NewDisruptor(
		r.env.Executor(),
		config,
		proxy,
	)
	if err != nil {
		return err
	}

	return r.do(ctx, func(ctx context.Context) error {
		return disruptor.Apply(ctx, duration)
	})
}
