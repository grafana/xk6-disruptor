package http

import (
	"testing"
	"time"
)

func Test_Validations(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		title       string
		disruption  HttpDisruption
		expectError bool
	}{
		{
			title: "valid defaults",
			disruption: HttpDisruption{
				Duration:       time.Second * 1,
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Iface:          "eth0",
				Port:           8080,
				Target:         80,
				Excluded:       nil,
			},
			expectError: false,
		},
		{
			title: "duration under 1s",
			disruption: HttpDisruption{
				Duration:       time.Microsecond * 100,
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Iface:          "eth0",
				Port:           8080,
				Target:         80,
				Excluded:       nil,
			},
			expectError: true,
		},
		{
			title: "variation larger than average delay",
			disruption: HttpDisruption{
				Duration:       time.Second * 1,
				DelayVariation: 200,
				AverageDelay:   100,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Iface:          "eth0",
				Port:           8080,
				Target:         80,
				Excluded:       nil,
			},
			expectError: true,
		},
		{
			title: "valid error rate",
			disruption: HttpDisruption{
				Duration:       time.Second * 1,
				DelayVariation: 0,
				AverageDelay:   0,
				ErrorRate:      0.1,
				ErrorCode:      500,
				Iface:          "eth0",
				Port:           8080,
				Target:         80,
				Excluded:       nil,
			},
			expectError: false,
		},
		{
			title: "valid delay and variation",
			disruption: HttpDisruption{
				Duration:       time.Second * 1,
				DelayVariation: 10,
				AverageDelay:   100,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Iface:          "eth0",
				Port:           8080,
				Target:         80,
				Excluded:       nil,
			},
			expectError: false,
		},
		{
			title: "invalid error code",
			disruption: HttpDisruption{
				Duration:       time.Second * 1,
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      0,
				Iface:          "eth0",
				Port:           8080,
				Target:         80,
				Excluded:       nil,
			},
			expectError: true,
		},
		{
			title: "negative error rate",
			disruption: HttpDisruption{
				Duration:       time.Second * 1,
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      -1.0,
				ErrorCode:      0,
				Iface:          "eth0",
				Port:           8080,
				Target:         80,
				Excluded:       nil,
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			err := tc.disruption.Run()
			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
			}
		})
	}
}
