package iptables

import (
	"fmt"
	"strings"
	"testing"

	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

func Test_validateTrafficRedirect(t *testing.T) {
	t.Parallel()

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
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

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
	t.Parallel()

	TestCases := []struct {
		title        string
		redirect     TrafficRedirectionSpec
		expectedCmds []string
		expectError  bool
		fakeError    error
		fakeOutput   []byte
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
			fakeError:   nil,
			fakeOutput:  []byte{},
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
			fakeError:   nil,
			fakeOutput:  []byte{},
		},
		{
			title: "Error invoking iptables command in Start",
			redirect: TrafficRedirectionSpec{
				Iface:           "eth0",
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			testFunction: func(tr TrafficRedirector) error {
				return tr.Start()
			},
			expectedCmds: []string{},
			expectError:  true,
			fakeError:    fmt.Errorf("process exited with return code 1"),
			fakeOutput:   []byte{},
		},
		{
			title: "Error invoking iptables command in Stop",
			redirect: TrafficRedirectionSpec{
				Iface:           "eth0",
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			testFunction: func(tr TrafficRedirector) error {
				return tr.Stop()
			},
			expectedCmds: []string{},
			expectError:  true,
			fakeError:    fmt.Errorf("process exited with return code 1"),
			fakeOutput:   []byte{},
		},
	}

	for _, tc := range TestCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			executor := runtime.NewFakeExecutor(tc.fakeOutput, tc.fakeError)
			config := TrafficRedirectorConfig{
				Executor: executor,
			}
			redirector, err := NewTrafficRedirectorWithConfig(&tc.redirect, config)
			if err != nil {
				t.Errorf("failed creating traffic redirector with error %v", err)
				return
			}

			// execute test and collect result
			err = tc.testFunction(redirector)

			if !tc.expectError && err != nil {
				t.Errorf("failed with error: %v", err)
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
