// Package http implements an disruptor for http requests.
// The disruptor runs as a proxy, redirecting the traffic from
// a target defined in [HttpDisruptionTarget] using iptables.
// The intercepted requests are forwarded to the target and optionally are
// disrupted according to the disruption options defined in [HttpDisruption].
// The configuration of the proxy is defined in the [HttpDisruptorConfig].
package http

import (
	"fmt"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/iptables"
)

// HttpDisruptor defines the interface disruptor of http requests
type HttpDisruptor interface {
	Apply(duration time.Duration) error
}

// HttpDisruption specifies disruptions in http requests
type HttpDisruption struct {
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

// HttpDisruptorConfig defines the configuration options for the HttpDisruptor
type HttpDisruptorConfig struct {
	ProxyConfig HttpProxyConfig
}

// HttpDisruptionTarget defines the target of the disruptions
type HttpDisruptionTarget struct {
	// Destination port to intercept traffic
	TargetPort uint
	// Network interface where the traffic will be intercepted
	Iface string
}

// httpDisruptor is an instance of a HttpDisruptor that applies a disruption
// to a target
type httpDisruptor struct {
	// Target of the disruption
	target HttpDisruptionTarget
	// Description of the disruption
	disruption HttpDisruption
	// Description of the http disruptor
	config HttpDisruptorConfig
	// HttpProxy
	proxy HttpProxy
	// TrafficRedirect
	redirector iptables.TrafficRedirector
}

// validateHttpDisruption validates a HttpDisruption struct
func validateHttpDisruption(d HttpDisruption) error {
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

// validateHttpDisruptionTarget validates a HttpDisruptionTarget
func validateHttpDisruptionTarget(d HttpDisruptionTarget) error {
	if d.TargetPort == 0 {
		return fmt.Errorf("target port must be valid tcp port")
	}

	if d.Iface == "" {
		return fmt.Errorf("disruption target must specify an interface")
	}

	return nil
}

// NewDefaultHttpDisruptor creates a HttpDisruptor with valid default configuration.
func NewDefaultHttpDisruptor(target HttpDisruptionTarget, disruption HttpDisruption) (HttpDisruptor, error) {
	return NewHttpDisruptor(
		target,
		disruption,
		HttpDisruptorConfig{
			ProxyConfig: HttpProxyConfig{
				ListeningPort: 8080,
			},
		},
	)
}

// NewHttpDisruptor creates a new instance of a HttpDisruptor that applies a disruptions to a target
// The configuration controls how the disruptor operates.
func NewHttpDisruptor(
	target HttpDisruptionTarget,
	disruption HttpDisruption,
	config HttpDisruptorConfig,
) (HttpDisruptor, error) {
	err := validateHttpDisruption(disruption)
	if err != nil {
		return nil, err
	}

	err = validateHttpDisruptionTarget(target)
	if err != nil {
		return nil, err
	}

	proxyTarget := HttpProxyTarget{
		Port: target.TargetPort,
	}
	proxy, err := NewHttpProxy(
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

	return &httpDisruptor{
		target:     target,
		disruption: disruption,
		config:     config,
		proxy:      proxy,
		redirector: redirector,
	}, nil
}

// Run applies the HttpDisruption to the target system
func (d *httpDisruptor) Apply(duration time.Duration) error {
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
