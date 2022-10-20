// Package checks implements functions that verify conditions in a cluster
package checks

import (
	"fmt"
	"net/http"
	"time"
)

const (
	defaultSvcURL  = "http://127.0.0.1"
	defaultSvcPort = 32080
)

// ServiceCheck defines the conditions to check in the access to a service
type ServiceCheck struct {
	// URL to access the service (default http://127.0.0.1)
	URL string
	// Port to access the service (default 32080)
	Port int32
	// Expected return code (default 200)
	ExpectedCode int
	// Delay before attempting access to service
	Delay time.Duration
}

// CheckService verifies access to service returns the expected result
func CheckService(c ServiceCheck) error {
	time.Sleep(c.Delay)

	url := c.URL
	if url == "" {
		url = defaultSvcURL
	}
	port := c.Port
	if port == 0 {
		port = defaultSvcPort
	}
	requestURL := fmt.Sprintf("%s:%d", url, port)
	resp, err := http.Get(requestURL)
	if err != nil {
		return fmt.Errorf("failed to access service at %s: %v", url, err)
	}

	if resp.StatusCode != c.ExpectedCode {
		return fmt.Errorf("expected status code %d but %d received", c.ExpectedCode, resp.StatusCode)
	}

	return nil
}
