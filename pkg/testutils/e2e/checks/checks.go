// Package checks implements functions that verify conditions in a cluster
package checks

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/grpc/dynamic"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

// Check defines an interface for verifying conditions in a test
type Check interface {
	// Verify asserts is the check is satisfied or some error occurs
	Verify(k8s kubernetes.Kubernetes, ingress string, namespace string) error
}

// HTTPCheck defines the operation and conditions to check in the access to a service
// TODO: add support for passing headers to the request
// TODO: add checks for expected response body
type HTTPCheck struct {
	// Service name
	Service string
	// Port to access the service (default 80)
	Port int
	// Request Method (default GET)
	Method string
	// Request body
	Body []byte
	// request path
	Path string
	// Expected return code (default 200)
	ExpectedCode int
	// Delay before attempting access to service
	Delay time.Duration
}

// GrpcCheck defines the operation and conditions to check in the access to a service
type GrpcCheck struct {
	// Service name
	Service string
	// Port to access the service (default 3000)
	Port int
	// Grpc service to invoke
	GrpcService string
	// Method to invoke
	Method string
	// Request message
	Request []byte
	// Expected return code (default OK)
	ExpectedStatus int32
	// Delay before attempting access to service
	Delay time.Duration
}

// Verify verifies a HTTPCheck
func (c HTTPCheck) Verify(k kubernetes.Kubernetes, ingress string, namespace string) error {
	time.Sleep(c.Delay)

	request, err := http.NewRequest(c.Method, ingress, bytes.NewReader(c.Body))
	if err != nil {
		return err
	}
	request.Host = c.Service

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed request to service %s: %w", c.Service, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != c.ExpectedCode {
		return fmt.Errorf("expected status code %d but %d received", c.ExpectedCode, resp.StatusCode)
	}

	return nil
}

// Verify verifies a GrpcServiceCheck
func (c GrpcCheck) Verify(k kubernetes.Kubernetes, ingress string, namespace string) error {
	time.Sleep(c.Delay)

	client, err := dynamic.NewClientWithDialOptions(
		ingress,
		c.GrpcService,
		grpc.WithInsecure(),
		grpc.WithAuthority(c.Service),
	)
	if err != nil {
		return fmt.Errorf("error creating client for service %s: %w", c.Service, err)
	}

	err = client.Connect(context.TODO())
	if err != nil {
		return fmt.Errorf("error connecting to service %s: %w", c.Service, err)
	}

	input := [][]byte{}
	input = append(input, c.Request)

	_, err = client.Invoke(context.TODO(), c.Method, input)
	// got an error but it is not due to the grpc status
	s, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("unexpected error %w", err)
	}

	if int32(s.Code()) != c.ExpectedStatus {
		return fmt.Errorf("expected status code %d but %d received", c.ExpectedStatus, int32(s.Code()))
	}

	return nil
}
