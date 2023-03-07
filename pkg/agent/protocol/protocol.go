// Package protocol implements the agent that injects disruptors in protocols.
// The protocol disruptors run as a proxy. The agent redirects the traffic
// to the proxy using iptables.
package protocol

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/iptables"
)

// Disruptor defines the interface agent
type Disruptor interface {
	Apply(duration time.Duration) error
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

// NewDefaultDisruptor creates a Disruptor with valid default configuration.
func NewDefaultDisruptor(proxy Proxy) (Disruptor, error) {
	return NewDisruptor(
		DisruptorConfig{
			RedirectPort: 8080,
			TargetPort:   80,
			Iface:        "eth0",
		},
		proxy,
	)
}

// NewDisruptor creates a new instance of a Disruptor that applies a disruptions to a target
// The configuration controls how the disruptor operates.
func NewDisruptor(
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

	// Redirect traffic to the proxy
	tr := iptables.TrafficRedirectionSpec{
		Iface:           config.Iface,
		DestinationPort: config.TargetPort,
		RedirectPort:    config.RedirectPort,
	}
	redirector, err := iptables.NewTrafficRedirector(&tr)
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
func (d *disruptor) Apply(duration time.Duration) error {
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

	// Wait for request duration or proxy server error
	for {
		select {
		case err := <-wc:
			if err != nil {
				return fmt.Errorf(" proxy ended with error: %w", err)
			}
		case <-time.After(duration):
			return nil
		}
	}
}
