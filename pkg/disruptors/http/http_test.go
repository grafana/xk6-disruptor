package http

import (
	"testing"
	"time"
)

func Test_Validations(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		title       string
		disruption  HttpDisruptionRequest
		expectError bool
	}{
		{
			title: "valid defaults",
			disruption: HttpDisruptionRequest{
				Duration: time.Second * 1,
				HttpDisruption: HttpDisruption{
					AverageDelay:   0,
					DelayVariation: 0,
					ErrorRate:      0.0,
					ErrorCode:      0,
					Excluded:       nil,
				},
				HttpDisruptionTarget: HttpDisruptionTarget{
					Iface:      "eth0",
					TargetPort: 80,
				},
				HttpProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: false,
		},
		{
			title: "duration under 1s",
			disruption: HttpDisruptionRequest{
				Duration: time.Microsecond * 100,
				HttpDisruption: HttpDisruption{
					AverageDelay:   0,
					DelayVariation: 0,
					ErrorRate:      0.0,
					ErrorCode:      0,
					Excluded:       nil,
				},
				HttpDisruptionTarget: HttpDisruptionTarget{
					Iface:      "eth0",
					TargetPort: 80,
				},
				HttpProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: true,
		},
		{
			title: "variation larger than average delay",
			disruption: HttpDisruptionRequest{
				Duration: time.Second * 1,
				HttpDisruption: HttpDisruption{
					AverageDelay:   100,
					DelayVariation: 200,
					ErrorRate:      0.0,
					ErrorCode:      0,
					Excluded:       nil,
				},
				HttpDisruptionTarget: HttpDisruptionTarget{
					Iface:      "eth0",
					TargetPort: 80,
				},
				HttpProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: true,
		},
		{
			title: "valid error rate",
			disruption: HttpDisruptionRequest{
				Duration: time.Second * 1,
				HttpDisruption: HttpDisruption{
					AverageDelay:   0,
					DelayVariation: 0,
					ErrorRate:      0.1,
					ErrorCode:      500,
					Excluded:       nil,
				},
				HttpDisruptionTarget: HttpDisruptionTarget{
					Iface:      "eth0",
					TargetPort: 80,
				},
				HttpProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: false,
		},
		{
			title: "valid delay and variation",
			disruption: HttpDisruptionRequest{
				Duration: time.Second * 1,
				HttpDisruption: HttpDisruption{
					AverageDelay:   100,
					DelayVariation: 10,
					ErrorRate:      0.0,
					ErrorCode:      0,
					Excluded:       nil,
				},
				HttpDisruptionTarget: HttpDisruptionTarget{
					Iface:      "eth0",
					TargetPort: 80,
				},
				HttpProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: false,
		},
		{
			title: "invalid error code",
			disruption: HttpDisruptionRequest{
				Duration: time.Second * 1,
				HttpDisruption: HttpDisruption{
					AverageDelay:   0,
					DelayVariation: 0,
					ErrorRate:      1.0,
					ErrorCode:      0,
					Excluded:       nil,
				},
				HttpDisruptionTarget: HttpDisruptionTarget{
					Iface:      "eth0",
					TargetPort: 80,
				},
				HttpProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: true,
		},
		{
			title: "negative error rate",
			disruption: HttpDisruptionRequest{
				Duration: time.Second * 1,
				HttpDisruption: HttpDisruption{
					AverageDelay:   0,
					DelayVariation: 0,
					ErrorRate:      -1.0,
					ErrorCode:      0,
					Excluded:       nil,
				},
				HttpDisruptionTarget: HttpDisruptionTarget{
					Iface:      "eth0",
					TargetPort: 80,
				},
				HttpProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			err := tc.disruption.validate()
			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
			}
		})
	}
}
