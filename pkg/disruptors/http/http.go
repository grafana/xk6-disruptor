// Package http implements an disruptor for http requests.
// The disruptor runs as a proxy, redirecting the traffic from
// a target defined in [DisruptionTarget] using iptables.
// The intercepted requests are forwarded to the target and optionally are
// disrupted according to the disruption options defined in [Disruption].
// The configuration of the proxy is defined in the [DisruptorConfig].
package http

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/iptables"
)

// Disruptor defines the interface disruptor of http requests
type Disruptor interface {
	Apply(duration time.Duration) error
}

// Disruption specifies disruptions in http requests
type Disruption struct {
	// Average delay introduced to requests
	AverageDelay uint
	// Variation in the delay (with respect of the average delay)
	DelayVariation uint
	// Fraction (in the range 0.0 to 1.0) of requests that will return an error
	ErrorRate float32
	// Error code to be returned by requests selected in the error rate
	ErrorCode uint
	// List of url paths to be excluded from disruptions
	Excluded []string
}

// DisruptorConfig defines the configuration options for the Disruptor
type DisruptorConfig struct {
	ProxyConfig ProxyConfig
}

// DisruptionTarget defines the target of the disruptions
type DisruptionTarget struct {
	// Destination port to intercept traffic
	TargetPort uint
	// Network interface where the traffic will be intercepted
	Iface string
}

// disruptor is an instance of a Disruptor that applies a disruption
// to a target
type disruptor struct {
	// Target of the disruption
	target DisruptionTarget
	// Description of the disruption
	disruption Disruption
	// Description of the http disruptor
	config DisruptorConfig
	// Proxy
	proxy Proxy
	// TrafficRedirect
	redirector iptables.TrafficRedirector
}

// validateDisruption validates a Disruption struct
func validateDisruption(d Disruption) error {
	if d.DelayVariation > d.AverageDelay {
		return fmt.Errorf("variation must be less that average delay")
	}

	if d.ErrorRate < 0.0 || d.ErrorRate > 1.0 {
		return fmt.Errorf("error rate must be in the range [0.0, 1.0]")
	}

	if d.ErrorRate > 0.0 && d.ErrorCode == 0 {
		return fmt.Errorf("error code must be a valid http error code")
	}

	return nil
}

// validateDisruptionTarget validates a DisruptionTarget
func validateDisruptionTarget(d DisruptionTarget) error {
	if d.TargetPort == 0 {
		return fmt.Errorf("target port must be valid tcp port")
	}

	if d.Iface == "" {
		return fmt.Errorf("disruption target must specify an interface")
	}

	return nil
}

// NewDefaultDisruptor creates a Disruptor with valid default configuration.
func NewDefaultDisruptor(target DisruptionTarget, disruption Disruption) (Disruptor, error) {
	return NewDisruptor(
		target,
		disruption,
		DisruptorConfig{
			ProxyConfig: ProxyConfig{
				ListeningPort: 8080,
			},
		},
	)
}

// NewDisruptor creates a new instance of a Disruptor that applies a disruptions to a target
// The configuration controls how the disruptor operates.
func NewDisruptor(
	target DisruptionTarget,
	disruption Disruption,
	config DisruptorConfig,
) (Disruptor, error) {
	err := validateDisruption(disruption)
	if err != nil {
		return nil, err
	}

	err = validateDisruptionTarget(target)
	if err != nil {
		return nil, err
	}

	proxyTarget := Target{
		Port: target.TargetPort,
	}
	proxy, err := NewProxy(
		proxyTarget,
		disruption,
		config.ProxyConfig,
	)
	if err != nil {
		return nil, err
	}

	// Redirect traffic to the proxy
	tr := iptables.TrafficRedirectionSpec{
		Iface:           target.Iface,
		DestinationPort: target.TargetPort,
		RedirectPort:    config.ProxyConfig.ListeningPort,
	}
	redirector, err := iptables.NewTrafficRedirector(&tr)
	if err != nil {
		return nil, err
	}

	return &disruptor{
		target:     target,
		disruption: disruption,
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

	err := d.redirector.Start()
	if err != nil {
		return fmt.Errorf(" failed traffic redirection: %s", err)
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
				return fmt.Errorf(" proxy ended with error: %s", err)
			}
		case <-time.After(duration):
			return nil
		}
	}
}
