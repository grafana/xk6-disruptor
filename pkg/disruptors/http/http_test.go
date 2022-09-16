package http

import (
	"testing"
	"time"
)

func Test_Validations(t *testing.T) {
	t.Parallel()
	testCases := []struct {
		title       string
		duration    time.Duration
		target      HttpDisruptionTarget
		disruption  HttpDisruption
		config      HttpDisruptorConfig
		expectError bool
	}{
		{
			title:    "valid defaults",
			duration: time.Second * 1,
			disruption: HttpDisruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			target: HttpDisruptionTarget{
				Iface:      "eth0",
				TargetPort: 80,
			},
			config: HttpDisruptorConfig{
				ProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: false,
		},
		{
			title:    "invalid listening port",
			duration: time.Second * 1,
			disruption: HttpDisruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			target: HttpDisruptionTarget{
				Iface:      "eth0",
				TargetPort: 80,
			},
			config: HttpDisruptorConfig{
				ProxyConfig: HttpProxyConfig{
					ListeningPort: 0,
				},
			},
			expectError: true,
		},
		{
			title:    "invalid target port",
			duration: time.Second * 1,
			disruption: HttpDisruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			target: HttpDisruptionTarget{
				Iface:      "eth0",
				TargetPort: 0,
			},
			config: HttpDisruptorConfig{
				ProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: true,
		},
		{
			title:    "target port equals listening port",
			duration: time.Second * 1,
			disruption: HttpDisruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			target: HttpDisruptionTarget{
				Iface:      "eth0",
				TargetPort: 8080,
			},
			config: HttpDisruptorConfig{
				ProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: true,
		},
		{
			title:    "missing target iface",
			duration: time.Second * 1,
			disruption: HttpDisruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			target: HttpDisruptionTarget{
				Iface:      "",
				TargetPort: 80,
			},
			config: HttpDisruptorConfig{
				ProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: true,
		},
		{
			title:    "variation larger than average delay",
			duration: time.Second * 1,
			disruption: HttpDisruption{
				AverageDelay:   100,
				DelayVariation: 200,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			target: HttpDisruptionTarget{
				Iface:      "eth0",
				TargetPort: 80,
			},
			config: HttpDisruptorConfig{
				ProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: true,
		},
		{
			title:    "valid error rate",
			duration: time.Second * 1,
			disruption: HttpDisruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      0.1,
				ErrorCode:      500,
				Excluded:       nil,
			},
			target: HttpDisruptionTarget{
				Iface:      "eth0",
				TargetPort: 80,
			},
			config: HttpDisruptorConfig{
				ProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: false,
		},
		{
			title:    "valid delay and variation",
			duration: time.Second * 1,
			disruption: HttpDisruption{
				AverageDelay:   100,
				DelayVariation: 10,
				ErrorRate:      0.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			target: HttpDisruptionTarget{
				Iface:      "eth0",
				TargetPort: 80,
			},
			config: HttpDisruptorConfig{
				ProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: false,
		},
		{
			title:    "invalid error code",
			duration: time.Second * 1,
			disruption: HttpDisruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      1.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			target: HttpDisruptionTarget{
				Iface:      "eth0",
				TargetPort: 80,
			},
			config: HttpDisruptorConfig{
				ProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: true,
		},
		{
			title:    "negative error rate",
			duration: time.Second * 1,
			disruption: HttpDisruption{
				AverageDelay:   0,
				DelayVariation: 0,
				ErrorRate:      -1.0,
				ErrorCode:      0,
				Excluded:       nil,
			},
			target: HttpDisruptionTarget{
				Iface:      "eth0",
				TargetPort: 80,
			},
			config: HttpDisruptorConfig{
				ProxyConfig: HttpProxyConfig{
					ListeningPort: 8080,
				},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			_, err := NewHttpDisruptor(
				tc.target,
				tc.disruption,
				tc.config,
			)
			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
			}
		})
	}
}
