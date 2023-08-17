// Package grpc implements a proxy that applies disruptions to gRPC requests
// This package is inspired by and extensively copies code from https://github.com/mwitkow/grpc-proxy
package grpc

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"

	"google.golang.org/grpc"
)

// ProxyConfig configures the Proxy options
type ProxyConfig struct {
	// network used for communication (valid values are "unix" and "tcp")
	Network string
	// Address to listen for incoming requests
	ListenAddress string
	// Address where to redirect requests
	UpstreamAddress string
}

// Disruption specifies disruptions in grpc requests
type Disruption struct {
	// Average delay introduced to requests
	AverageDelay time.Duration
	// Variation in the delay (with respect of the average delay)
	DelayVariation time.Duration
	// Fraction (in the range 0.0 to 1.0) of requests that will return an error
	ErrorRate float32
	// Status code to be returned by requests selected to return an error
	StatusCode int32
	// Status message to be returned in requests selected to return an error
	StatusMessage string
	// List of grpc services to be excluded from disruptions
	Excluded []string
}

// Proxy defines the parameters used by the proxy for processing grpc requests and its execution state
type proxy struct {
	config     ProxyConfig
	disruption Disruption
	srv        *grpc.Server
	metrics    *protocol.MetricMap
}

// NewProxy return a new Proxy
func NewProxy(c ProxyConfig, d Disruption) (protocol.Proxy, error) {
	if c.Network == "" {
		c.Network = "tcp"
	}

	if c.Network != "tcp" && c.Network != "unix" {
		return nil, fmt.Errorf("only 'tcp' and 'unix' (for sockets) networks are supported")
	}

	if c.ListenAddress == "" {
		return nil, fmt.Errorf("proxy's listening address must be provided")
	}

	if c.UpstreamAddress == "" {
		return nil, fmt.Errorf("proxy's forwarding address must be provided")
	}

	if d.DelayVariation > d.AverageDelay {
		return nil, fmt.Errorf("variation must be less that average delay")
	}

	if d.ErrorRate < 0.0 || d.ErrorRate > 1.0 {
		return nil, fmt.Errorf("error rate must be in the range [0.0, 1.0]")
	}

	if d.ErrorRate > 0.0 && d.StatusCode == 0 {
		return nil, fmt.Errorf("status code cannot be 0 (OK)")
	}

	return &proxy{
		disruption: d,
		config:     c,
		metrics:    &protocol.MetricMap{},
	}, nil
}

// Start starts the execution of the proxy
func (p *proxy) Start() error {
	// should receive the context as a parameter of the Start function
	conn, err := grpc.DialContext(
		context.Background(),
		p.config.UpstreamAddress,
		grpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("error dialing %s: %w", p.config.UpstreamAddress, err)
	}
	handler := NewHandler(p.disruption, conn, p.metrics)

	p.srv = grpc.NewServer(
		grpc.UnknownServiceHandler(handler),
	)

	listener, err := net.Listen(p.config.Network, p.config.ListenAddress)
	if err != nil {
		return fmt.Errorf("error listening to %s: %w", p.config.ListenAddress, err)
	}

	err = p.srv.Serve(listener)
	if err != nil {
		return fmt.Errorf("proxy terminated with error: %w", err)
	}

	return nil
}

// Stop stops the execution of the proxy
func (p *proxy) Stop() error {
	if p.srv != nil {
		p.srv.GracefulStop()
	}
	return nil
}

// Metrics returns runtime metrics for the proxy.
// TODO: Add metrics.
func (p *proxy) Metrics() map[string]uint {
	return p.metrics.Map()
}

// Force stops the proxy without waiting for connections to drain
// In grpc this action is a nop
func (p *proxy) Force() error {
	if p.srv != nil {
		p.srv.Stop()
	}
	return nil
}
