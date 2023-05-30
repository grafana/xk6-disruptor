package protocol

import (
	"testing"
	"time"
)

type fakeProxy struct{}

func (f *fakeProxy) Start() error {
	return nil
}

func (f *fakeProxy) Stop() error {
	return nil
}

func (f *fakeProxy) Force() error {
	return nil
}

func Test_Validations(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		duration    time.Duration
		config      DisruptorConfig
		proxy       Proxy
		expectError bool
	}{
		{
			title:    "valid defaults",
			duration: time.Second * 1,
			config: DisruptorConfig{
				TargetPort:   80,
				Iface:        "eth0",
				RedirectPort: 8080,
			},
			proxy:       &fakeProxy{},
			expectError: false,
		},
		{
			title:    "invalid RedirectPort port",
			duration: time.Second * 1,
			config: DisruptorConfig{
				TargetPort:   80,
				Iface:        "eth0",
				RedirectPort: 0,
			},
			proxy:       &fakeProxy{},
			expectError: true,
		},
		{
			title:    "invalid target port",
			duration: time.Second * 1,
			config: DisruptorConfig{
				TargetPort:   0,
				Iface:        "eth0",
				RedirectPort: 8080,
			},
			proxy:       &fakeProxy{},
			expectError: true,
		},
		{
			title:    "target port equals redirect port",
			duration: time.Second * 1,
			config: DisruptorConfig{
				Iface:        "eth0",
				TargetPort:   8080,
				RedirectPort: 8080,
			},
			proxy:       &fakeProxy{},
			expectError: true,
		},
		{
			title:    "missing  iface",
			duration: time.Second * 1,
			config: DisruptorConfig{
				Iface:        "",
				TargetPort:   80,
				RedirectPort: 8080,
			},
			proxy:       &fakeProxy{},
			expectError: true,
		},
		{
			title:    "missing proxy",
			duration: time.Second * 1,
			config: DisruptorConfig{
				Iface:        "",
				TargetPort:   80,
				RedirectPort: 8080,
			},
			proxy:       nil,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			_, err := NewDisruptor(
				nil, // TODO: pass a fake executor
				tc.config,
				tc.proxy,
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
