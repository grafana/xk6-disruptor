package iptables

import (
	"fmt"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
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
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			expectError: false,
		},
		{
			title: "Same target and proxy port",
			redirect: TrafficRedirectionSpec{
				DestinationPort: 8080,
				RedirectPort:    8080,
			},
			expectError: true,
		},
		{
			title:       "Ports not specified",
			redirect:    TrafficRedirectionSpec{},
			expectError: true,
		},
	}

	for _, tc := range TestCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			executor := runtime.NewFakeExecutor(nil, nil)
			_, err := NewTrafficRedirector(
				&tc.redirect,
				executor,
			)
			if tc.expectError && err == nil {
				t.Errorf("error expected but none returned")
			}

			if !tc.expectError && err != nil {
				t.Errorf("failed with error %v", err)
			}
		})
	}
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
		testFunction func(protocol.TrafficRedirector) error
	}{
		{
			title: "Start valid redirect",
			redirect: TrafficRedirectionSpec{
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			testFunction: func(tr protocol.TrafficRedirector) error {
				return tr.Start()
			},
			expectedCmds: []string{
				"iptables -D INPUT -p tcp --dport 8080 -j REJECT --reject-with tcp-reset",
				"iptables -A OUTPUT -t nat -s 127.0.0.0/8 -d 127.0.0.1/32 -p tcp --dport 80 -j REDIRECT --to-port 8080",
				"iptables -A PREROUTING -t nat ! -i lo -p tcp --dport 80 -j REDIRECT --to-port 8080",
				"iptables -A INPUT -i lo -s 127.0.0.0/8 -d 127.0.0.1/32 -p tcp --dport 80 -m state --state ESTABLISHED -j REJECT --reject-with tcp-reset",
				"iptables -A INPUT ! -i lo -p tcp --dport 80 -m state --state ESTABLISHED -j REJECT --reject-with tcp-reset",
			},
			expectError: false,
			fakeError:   nil,
			fakeOutput:  []byte{},
		},
		{
			title: "Stop active redirect",
			redirect: TrafficRedirectionSpec{
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			testFunction: func(tr protocol.TrafficRedirector) error {
				return tr.Stop()
			},
			expectedCmds: []string{
				"iptables -D OUTPUT -t nat -s 127.0.0.0/8 -d 127.0.0.1/32 -p tcp --dport 80 -j REDIRECT --to-port 8080",
				"iptables -D PREROUTING -t nat ! -i lo -p tcp --dport 80 -j REDIRECT --to-port 8080",
				"iptables -D INPUT -i lo -s 127.0.0.0/8 -d 127.0.0.1/32 -p tcp --dport 80 -m state --state ESTABLISHED -j REJECT --reject-with tcp-reset",
				"iptables -D INPUT ! -i lo -p tcp --dport 80 -m state --state ESTABLISHED -j REJECT --reject-with tcp-reset",
				"iptables -A INPUT -p tcp --dport 8080 -j REJECT --reject-with tcp-reset",
			},
			expectError: false,
			fakeError:   nil,
			fakeOutput:  []byte{},
		},
		{
			title: "Error invoking iptables command in Start",
			redirect: TrafficRedirectionSpec{
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			testFunction: func(tr protocol.TrafficRedirector) error {
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
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			testFunction: func(tr protocol.TrafficRedirector) error {
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
			redirector, err := NewTrafficRedirector(&tc.redirect, executor)
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

			if diff := cmp.Diff(tc.expectedCmds, executor.CmdHistory()); diff != "" {
				t.Fatalf("Actual commands differ from expected:\n%s", diff)
			}
		})
	}
}
