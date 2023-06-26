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
				LocalAddress: "127.120.107.6",
				Interface:    "lo",
				TargetPort:   "80",
				ProxyPort:    "8080",
			},
			expectError: false,
		},
		{
			title: "Ports not specified",
			redirect: TrafficRedirectionSpec{
				LocalAddress: "127.120.107.6",
				Interface:    "lo",
			},
			expectError: true,
		},
		{
			title: "Same target and proxy port",
			redirect: TrafficRedirectionSpec{
				LocalAddress: "127.120.107.6",
				Interface:    "lo",
				TargetPort:   "8080",
				ProxyPort:    "8080",
			},
			expectError: true,
		},
		{
			title: "Address or interface not specified",
			redirect: TrafficRedirectionSpec{
				TargetPort: "80",
				ProxyPort:  "8080",
			},
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
				LocalAddress: "127.120.107.6",
				Interface:    "lo",
				TargetPort:   "80",
				ProxyPort:    "8080",
			},
			testFunction: func(tr protocol.TrafficRedirector) error {
				return tr.Start()
			},
			expectedCmds: []string{
				"ip addr add 127.120.107.6/32 dev lo",
				"iptables -A OUTPUT -t nat -p tcp --dport 80 ! -s 127.120.107.6/32 -j REDIRECT --to-port 8080",
				"iptables -A PREROUTING -t nat -p tcp --dport 80 ! -s 127.120.107.6/32 -j REDIRECT --to-port 8080",
				"iptables -A INPUT ! -s 127.120.107.6/32 -p tcp --dport 80 -m state --state ESTABLISHED -j REJECT --reject-with tcp-reset",
			},
			expectError: false,
			fakeError:   nil,
			fakeOutput:  []byte{},
		},
		{
			title: "Stop active redirect",
			redirect: TrafficRedirectionSpec{
				LocalAddress: "127.120.107.6",
				Interface:    "lo",
				TargetPort:   "80",
				ProxyPort:    "8080",
			},
			testFunction: func(tr protocol.TrafficRedirector) error {
				return tr.Stop()
			},
			expectedCmds: []string{
				"iptables -D OUTPUT -t nat -p tcp --dport 80 ! -s 127.120.107.6/32 -j REDIRECT --to-port 8080",
				"iptables -D PREROUTING -t nat -p tcp --dport 80 ! -s 127.120.107.6/32 -j REDIRECT --to-port 8080",
				"iptables -D INPUT ! -s 127.120.107.6/32 -p tcp --dport 80 -m state --state ESTABLISHED -j REJECT --reject-with tcp-reset",
				"ip addr del 127.120.107.6/32 dev lo",
			},
			expectError: false,
			fakeError:   nil,
			fakeOutput:  []byte{},
		},
		{
			title: "Error invoking iptables command in Start",
			redirect: TrafficRedirectionSpec{
				LocalAddress: "127.120.107.6",
				Interface:    "lo",
				TargetPort:   "80",
				ProxyPort:    "8080",
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
				LocalAddress: "127.120.107.6",
				Interface:    "lo",
				TargetPort:   "80",
				ProxyPort:    "8080",
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
