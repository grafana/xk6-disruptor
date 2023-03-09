// Package grpc implements a proxy that applies disruptions to gRPC requests
// This package is inspired by and extensively copies code from https://github.com/mwitkow/grpc-proxy
package grpc

import (
	"context"
	"fmt"
	"net"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"

	"google.golang.org/grpc"
)

// ProxyConfig configures the Proxy options
type ProxyConfig struct {
	// port the proxy will listen to
	ListeningPort uint
	// port the proxy will redirect to
	Port uint
}

// Disruption specifies disruptions in grpc requests
type Disruption struct {
	// Average delay introduced to requests
	AverageDelay uint
	// Variation in the delay (with respect of the average delay)
	DelayVariation uint
	// Fraction (in the range 0.0 to 1.0) of requests that will return an error
	ErrorRate float32
	// Status code to be returned by requests selected to return an error
	StatusCode int32
	// Status message to be returned in requests selected to return an error
	StatusMessage string
}

// Proxy defines the parameters used by the proxy for processing grpc requests and its execution state
type proxy struct {
	config     ProxyConfig
	disruption Disruption
	srv        *grpc.Server
}

// NewProxy return a new Proxy
func NewProxy(c ProxyConfig, d Disruption) (protocol.Proxy, error) {
	if c.ListeningPort == 0 {
		return nil, fmt.Errorf("proxy's listening port must be valid tcp port")
	}

	if c.Port == 0 {
		return nil, fmt.Errorf("proxy's target port must be valid tcp port")
	}

	if c.Port == c.ListeningPort {
		return nil, fmt.Errorf("target port and listening port cannot be the same")
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
	}, nil
}

// Start starts the execution of the proxy
func (p *proxy) Start() error {
	upstreamURL := fmt.Sprintf(":%d", p.config.Port)

	// should receive the context as a parameter of the Start function
	conn, err := grpc.DialContext(
		context.Background(),
		upstreamURL,
		grpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("error dialing %s: %w", upstreamURL, err)
	}
	handler := NewHandler(p.disruption, conn)

	p.srv = grpc.NewServer(
		grpc.UnknownServiceHandler(handler),
	)

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", p.config.ListeningPort))
	if err != nil {
		return fmt.Errorf("error binding to listening port %d: %w", p.config.ListeningPort, err)
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

// Force stops the proxy without waiting for connections to drain
// In grpc this action is a nop
func (p *proxy) Force() error {
	if p.srv != nil {
		p.srv.Stop()
	}
	return nil
}
