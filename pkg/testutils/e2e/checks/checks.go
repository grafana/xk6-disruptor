// Package checks implements functions that verify conditions in a cluster
package checks

import (
	"bytes"
	"fmt"
	"net/http"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
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

// CheckService verifies access to service returns the expected result
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
