package iptables

import (
	"fmt"
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

func Test_Start(t *testing.T) {
	TestCases := []struct {
		title       string
		redirect    TrafficRedirectionSpec
		expectError bool
	}{
		{
			title: "Start valid redirect",
			redirect: TrafficRedirectionSpec{
				Iface:           "eth0",
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			expectError: false,
		},
	}

	for _, tc := range TestCases {
		t.Run(tc.title, func(t *testing.T) {
			executor := process.NewFakeProcessExecutor([]byte{}, nil)
			config := TrafficRedirectorConfig{
				Executor: executor,
			}
			redirector, err := newTrafficRedirectorWithConfig(&tc.redirect, config)
			if err != nil {
				t.Errorf("failed creating traffic redirector with error %v", err)
				return
			}

			err = redirector.Start()
			if !tc.expectError && err != nil {
				t.Errorf("failed with error %v", err)
				return
			}

			if executor.Invocations() != 2 {
				t.Errorf("expected 2 invocations but %d executed", executor.Invocations())
				return
			}

			if executor.Cmd() == "" {
				t.Errorf("command expected but none returned")
				return
			}

			cmdRedirect := executor.CmdHistory()[0]
			cmdReset := executor.CmdHistory()[1]

			if !strings.Contains(cmdRedirect, "-A") {
				t.Errorf("invalid iptables action")
				return
			}

			destination := fmt.Sprintf("--dport %d ", tc.redirect.DestinationPort)
			if !strings.Contains(cmdRedirect, destination) {
				t.Errorf("invalid iptables destination for redirect")
				return
			}

			redirect := fmt.Sprintf("--to-port %d ", tc.redirect.DestinationPort)
			if !strings.ContainsAny(cmdRedirect, redirect) {
				t.Errorf("invalid iptables destination")
				return
			}

			if !strings.Contains(cmdReset, destination) {
				t.Errorf("invalid iptables destination for redirect")
				return
			}
		})
	}
}

func Test_Stop(t *testing.T) {
	TestCases := []struct {
		title       string
		redirect    TrafficRedirectionSpec
		expectError bool
	}{
		{
			title: "Start valid redirect",
			redirect: TrafficRedirectionSpec{
				Iface:           "eth0",
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			expectError: false,
		},
	}

	for _, tc := range TestCases {
		t.Run(tc.title, func(t *testing.T) {
			executor := process.NewFakeProcessExecutor([]byte{}, nil)
			config := TrafficRedirectorConfig{
				Executor: executor,
			}
			redirector, err := newTrafficRedirectorWithConfig(&tc.redirect, config)
			if err != nil {
				t.Errorf("failed creating traffic redirector with error %v", err)
				return
			}

			err = redirector.Stop()
			if !tc.expectError && err != nil {
				t.Errorf("failed with error %v", err)
				return
			}

			if executor.Invocations() != 2 {
				t.Errorf("expected 2 invocations but %d executed", executor.Invocations())
				return
			}

			if executor.Cmd() == "" {
				t.Errorf("command expected but none returned")
				return
			}

			cmdRedirect := executor.CmdHistory()[0]
			cmdReset := executor.CmdHistory()[1]

			if !strings.Contains(cmdRedirect, "-D") {
				t.Errorf("invalid iptables action")
				return
			}

			destination := fmt.Sprintf("--dport %d ", tc.redirect.DestinationPort)
			if !strings.Contains(cmdRedirect, destination) {
				t.Errorf("invalid iptables destination for redirect")
				return
			}

			redirect := fmt.Sprintf("--to-port %d ", tc.redirect.DestinationPort)
			if !strings.ContainsAny(cmdRedirect, redirect) {
				t.Errorf("invalid iptables destination")
				return
			}

			resetPort := fmt.Sprintf("--dport %d ", tc.redirect.RedirectPort)
			if !strings.Contains(cmdReset, resetPort) {
				t.Errorf("invalid iptables destination for redirect")
				return
			}
		})
	}
}
