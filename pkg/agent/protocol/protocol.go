// Package protocol implements the agent that injects disruptors in protocols.
// The protocol disruptors run as a proxy. The agent redirects the traffic
// to the proxy using iptables.
package protocol

import (
	"context"
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/iptables"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

// Disruptor defines the interface agent
type Disruptor interface {
	Apply(context.Context, time.Duration) error
}

// DisruptorConfig defines the configuration options for the Disruptor
type DisruptorConfig struct {
	// Destination port to intercept protocol
	TargetPort uint
	// Network interface where the traffic will be intercepted
	Iface string
	// Port to redirect protocol to
	RedirectPort uint
}

// Proxy defines an interface for a proxy
type Proxy interface {
	Start() error
	Stop() error
	Force() error
}

// disruptor is an instance of a Disruptor that applies a disruption
// to a target
type disruptor struct {
	// Description of the http disruptor
	config DisruptorConfig
	// Proxy
	proxy Proxy
	// TrafficRedirect
	redirector iptables.TrafficRedirector
}

// NewDisruptor creates a new instance of a Disruptor that applies a disruptions to a target
// The configuration controls how the disruptor operates.
func NewDisruptor(
	executor runtime.Executor,
	config DisruptorConfig,
	proxy Proxy,
) (Disruptor, error) {
	if config.RedirectPort == 0 {
		return nil, fmt.Errorf("redirect port must be valid tcp port")
	}

	if config.TargetPort == 0 {
		return nil, fmt.Errorf("target port must be valid tcp port")
	}

	if config.Iface == "" {
		return nil, fmt.Errorf("disruption must specify an interface")
	}

	if proxy == nil {
		return nil, fmt.Errorf("proxy cannot be null")
	}

	trCfg := iptables.TrafficRedirectorConfig{
		Executor: executor,
	}
	// Redirect traffic to the proxy
	tr := &iptables.TrafficRedirectionSpec{
		Iface:           config.Iface,
		DestinationPort: config.TargetPort,
		RedirectPort:    config.RedirectPort,
	}
	redirector, err := iptables.NewTrafficRedirectorWithConfig(tr, trCfg)
	if err != nil {
		return nil, err
	}

	return &disruptor{
		config:     config,
		proxy:      proxy,
		redirector: redirector,
	}, nil
}

// Apply applies the Disruption to the target system
func (d *disruptor) Apply(ctx context.Context, duration time.Duration) error {
	if duration < time.Second {
		return fmt.Errorf("duration must be at least one second")
	}

	wc := make(chan error)
	go func() {
		wc <- d.proxy.Start()
	}()

	if err := d.redirector.Start(); err != nil {
		return fmt.Errorf(" failed traffic redirection: %w", err)
	}

	// On termination, restore traffic and stop proxy
	defer func() {
		// ignore errors when stopping. Nothing to do
		_ = d.redirector.Stop()
		_ = d.proxy.Stop()
	}()

	// Wait for request duration, context cancellation or proxy server error
	for {
		select {
		case err := <-wc:
			if err != nil {
				return fmt.Errorf(" proxy ended with error: %w", err)
			}
		case <-time.After(duration):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}
