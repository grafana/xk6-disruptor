package iptables

import (
	"strings"
	"testing"

	"github.com/grafana/xk6-disruptor/pkg/utils/process"
)

func Test_validateTrafficRedirect(t *testing.T) {
	TestCases := []struct {
		title       string
		redirect    TrafficRedirectionSpec
		expectError bool
	}{
		{
			title: "Valid redirect",
			redirect: TrafficRedirectionSpec{
				Iface:           "eth0",
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			expectError: false,
		},
		{
			title: "Ports not specified",
			redirect: TrafficRedirectionSpec{
				Iface:           "eth0",
				DestinationPort: 0,
				RedirectPort:    0,
			},
			expectError: true,
		},
		{
			title: "destination equals redirect port",
			redirect: TrafficRedirectionSpec{
				Iface:           "eth0",
				DestinationPort: 80,
				RedirectPort:    80,
			},
			expectError: true,
		},
		{
			title: "Invalid iface",
			redirect: TrafficRedirectionSpec{
				Iface:           "",
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			expectError: true,
		},
	}

	for _, tc := range TestCases {
		t.Run(tc.title, func(t *testing.T) {
			_, err := NewTrafficRedirector(&tc.redirect)
			if tc.expectError && err == nil {
				t.Errorf("error expected but none returned")
			}

			if !tc.expectError && err != nil {
				t.Errorf("failed with error %v", err)
			}
		})
	}
}

func compareCmds(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}

	for i := 0; i < len(a); i++ {
		if a[i] != b[i] {
			return false
		}
	}

	return true
}

func Test_Commands(t *testing.T) {
	TestCases := []struct {
		title        string
		redirect     TrafficRedirectionSpec
		expectedCmds []string
		expectError  bool
		testFunction func(TrafficRedirector) error
	}{
		{
			title: "Start valid redirect",
			redirect: TrafficRedirectionSpec{
				Iface:           "eth0",
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			testFunction: func(tr TrafficRedirector) error {
				return tr.Start()
			},
			expectedCmds: []string{
				"iptables -D INPUT -i eth0 -p tcp --dport 8080 -m state --state ESTABLISHED -j REJECT --reject-with tcp-reset",
				"iptables -A PREROUTING -t nat -i eth0 -p tcp --dport 80 -j REDIRECT --to-port 8080",
				"iptables -A INPUT -i eth0 -p tcp --dport 80 -m state --state ESTABLISHED -j REJECT --reject-with tcp-reset",
			},
			expectError: false,
		},
		{
			title: "Stop active redirect",
			redirect: TrafficRedirectionSpec{
				Iface:           "eth0",
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			testFunction: func(tr TrafficRedirector) error {
				return tr.Stop()
			},
			expectedCmds: []string{
				"iptables -D PREROUTING -t nat -i eth0 -p tcp --dport 80 -j REDIRECT --to-port 8080",
				"iptables -A INPUT -i eth0 -p tcp --dport 8080 -m state --state ESTABLISHED -j REJECT --reject-with tcp-reset",
				"iptables -D INPUT -i eth0 -p tcp --dport 80 -m state --state ESTABLISHED -j REJECT --reject-with tcp-reset",
			},
			expectError: false,
		},
	}

	for _, tc := range TestCases {
		t.Run(tc.title, func(t *testing.T) {
			executor := process.NewFakeExecutor([]byte{}, nil)
			config := TrafficRedirectorConfig{
				Executor: executor,
			}
			redirector, err := newTrafficRedirectorWithConfig(&tc.redirect, config)
			if err != nil {
				t.Errorf("failed creating traffic redirector with error %v", err)
				return
			}

			// execute test and collect result
			err = tc.testFunction(redirector)

			if !tc.expectError && err != nil {
				t.Errorf("failed with error %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if tc.expectError && err != nil {
				return
			}

			if !compareCmds(tc.expectedCmds, executor.CmdHistory()) {
				t.Errorf(
					"Actual commands differ from expected:\nExpected:\n\t%s\nActual:\n\t%s",
					strings.Join(tc.expectedCmds, "\n\t"),
					strings.Join(executor.CmdHistory(), "\n\t"),
				)
				return
			}
		})
	}
}
