package http

import (
	"fmt"
	"time"
)

// HttpDisruptions specifies disruptions in http requests
type HttpDisruption struct {
	// Duration of the disruption. Must be at least 1s
	Duration time.Duration
	// AverageDelay delay introduced to requests
	AverageDelay uint
	// DelayVariation in the delay (with respect of the average delay)
	DelayVariation uint
	// Fraction (in the range 0.0 to 1.0) of requests that will return an error
	ErrorRate float32
	// Error code to be returned by requests selected in the error rate
	ErrorCode uint
	// Network interface where the traffic will be intercepted
	Iface string
	// Port on which the proxy will be running
	Port uint
	// Port on which requests must be intercepted
	Target uint
	// List of url paths to be excluded from disruptions
	Excluded []string
}

// NewHttpDisruption created a HttpDisruption with valid default values.
// This defaults are safe: calling Run using the default values has no effect.
func NewHttpDisruption() *HttpDisruption {

	return &HttpDisruption{
		Duration:       time.Second * 1,
		AverageDelay:   0,
		DelayVariation: 0,
		ErrorRate:      0.0,
		ErrorCode:      0,
		Iface:          "eth0",
		Port:           8080,
		Target:         80,
		Excluded:       nil,
	}
}

// run applies the HttpDisruption to the target system
func (d *HttpDisruption) Run() error {
	duration := int(d.Duration.Seconds())
	if duration < 1 {
		return fmt.Errorf("duration must be at least one second")
	}

	if d.DelayVariation > d.AverageDelay {
		return fmt.Errorf("variation must be less that average delay")
	}

	if d.ErrorRate < 0.0 || d.ErrorRate > 1.0 {
		return fmt.Errorf("error rate must be in the range [0.0, 1.0]")
	}

	if d.ErrorRate > 0.0 && d.ErrorCode == 0 {
		return fmt.Errorf("error code must be a valid http error code")
	}

	if d.Port == 0 {
		return fmt.Errorf("port must be valid tcp port")
	}

	if d.Target == 0 {
		return fmt.Errorf("target port must be valid tcp port")
	}

	if d.Iface == "" {
		return fmt.Errorf("a network interface must be specified")
	}

	return nil
}
