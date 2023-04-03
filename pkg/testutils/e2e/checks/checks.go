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

// ServiceCheck defines the operation and conditions to check in the access to a service
// TODO: add support for passing headers to the request
// TODO: add checks for expected response body
type ServiceCheck struct {
	// Service name
	Service string
	// Namespace
	Namespace string
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

// GrpcServiceCheck defines the operation and conditions to check in the access to a service
type GrpcServiceCheck struct {
	// Host to connect to the grpc service (default localhost)
	Host string
	// Port to access the service (default 3000)
	Port int
	// Grpc service to invoke
	Service string
	// Method to invoke
	Method string
	// Request message
	Request []byte
	// Expected return code (default OK)
	ExpectedStatus int32
	// Delay before attempting access to service
	Delay time.Duration
}

// CheckService verifies a request to a service returns the expected result
func CheckService(k8s kubernetes.Kubernetes, c ServiceCheck) error {
	time.Sleep(c.Delay)

	namespace := c.Namespace
	if namespace == "" {
		namespace = "default"
	}

	port := c.Port
	if port == 0 {
		port = 80
	}

	serviceClient, err := k8s.NamespacedHelpers(namespace).GetServiceProxy(c.Service, port)
	if err != nil {
		return err
	}

	request, err := http.NewRequest(c.Method, c.Path, bytes.NewReader(c.Body))
	if err != nil {
		return err
	}

	resp, err := serviceClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed to access service at %s: %w", c.Service, err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != c.ExpectedCode {
		return fmt.Errorf("expected status code %d but %d received", c.ExpectedCode, resp.StatusCode)
	}

	return nil
}

// CheckGrpcService verifies a request to a grpc service returns the expected result
func CheckGrpcService(k8s kubernetes.Kubernetes, c GrpcServiceCheck) error {
	time.Sleep(c.Delay)

	port := c.Port
	if port == 0 {
		port = 3000
	}

	host := c.Host
	if host == "" {
		host = "localhost"
	}

	target := fmt.Sprintf("%s:%d", host, port)
	client, err := dynamic.NewClientWithDialOptions(
		target,
		c.Service,
		grpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("error creating client for service %s on %s: %w", c.Service, target, err)
	}

	err = client.Connect(context.TODO())
	if err != nil {
		return fmt.Errorf("error connecting to service %s on %s: %w", c.Service, target, err)
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
