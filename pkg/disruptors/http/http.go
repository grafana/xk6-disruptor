package http

import (
	"fmt"
	"time"
)

// HttpDisruptionRequest specifies a http disruption requests
type HttpDisruptionRequest struct {
	// Duration of the disruption. Must be at least 1s
	Duration time.Duration
	// Target of the disruption
	HttpDisruptionTarget
	// Description of the http disruption
	HttpDisruption
	// Configuration of the proxy
	HttpProxyConfig
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

// ProxyConfig specifies the configuration for the http proxy
type HttpProxyConfig struct {
	// Port on which the proxy will be running
	ListeningPort uint
}

type HttpDisruptionTarget struct {
	// Destination port to intercept traffic
	TargetPort uint
	// Network interface where the traffic will be intercepted
	Iface string
}

// NewHttpDisruptionRequest created a HttpDisruptionRequest with valid default values.
// This defaults are safe: calling Run using the default values has no effect.
func NewHttpDisruptionRequest() *HttpDisruptionRequest {

	return &HttpDisruptionRequest{
		Duration: time.Second * 1,
		HttpDisruptionTarget: HttpDisruptionTarget{
			TargetPort: 80,
		},
		HttpDisruption: HttpDisruption{
			AverageDelay:   0,
			DelayVariation: 0,
			ErrorRate:      0.0,
			ErrorCode:      0,
			Excluded:       nil,
		},
		HttpProxyConfig: HttpProxyConfig{
			ListeningPort: 8080,
		},
	}
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

	return nil
}

func validateHttpProxyConfig(c HttpProxyConfig) error {
	if c.ListeningPort == 0 {
		return fmt.Errorf("proxy's listening port must be valid tcp port")
	}
	return nil
}

func (d *HttpDisruptionRequest) validate() error {
	duration := int(d.Duration.Seconds())
	if duration < 1 {
		return fmt.Errorf("duration must be at least one second")
	}

	err := validateHttpDisruption(d.HttpDisruption)
	if err != nil {
		return err
	}

	err = validateHttpProxyConfig(d.HttpProxyConfig)
	if err != nil {
		return err
	}

	err = validateHttpDisruptionTarget(d.HttpDisruptionTarget)
	if err != nil {
		return err
	}

	return nil
}

// run applies the HttpDisruption to the target system
func (d *HttpDisruptionRequest) Run() error {

	err := d.validate()
	if err != nil {
		return err
	}

	return nil
}
