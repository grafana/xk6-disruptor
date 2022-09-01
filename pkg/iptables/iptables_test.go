package iptables

import (
	"fmt"
	"strings"
	"testing"
)

// FIXME: private methods are tested because the public methods
// call os.exec methods to start processes. There is no easy way
// to mock these methods.

func Test_validateTrafficRedirect(t *testing.T) {
	TestCases := []struct {
		title       string
		redirect    TrafficRedirect
		expectError bool
	}{
		{
			title: "Valid redirect",
			redirect: TrafficRedirect{
				Iface:           "eth0",
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			expectError: false,
		},
		{
			title: "Ports not specified",
			redirect: TrafficRedirect{
				Iface:           "eth0",
				DestinationPort: 0,
				RedirectPort:    0,
			},
			expectError: true,
		},
		{
			title: "destination equals redirect port",
			redirect: TrafficRedirect{
				Iface:           "eth0",
				DestinationPort: 80,
				RedirectPort:    80,
			},
			expectError: true,
		},
		{
			title: "Invalid iface",
			redirect: TrafficRedirect{
				Iface:           "",
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			expectError: true,
		},
	}

	for _, tc := range TestCases {

		t.Run(tc.title, func(t *testing.T) {
			err := tc.redirect.validate()

			if tc.expectError && err == nil {
				t.Errorf("error expected but none returned")
			}

			if !tc.expectError && err != nil {
				t.Errorf("failed with error %v", err)
			}
		})
	}
}

// tests the construction of reset commands from a valid TrafficRedirect
func Test_buildRedirectCommand(t *testing.T) {
	TestCases := []struct {
		title    string
		redirect TrafficRedirect
		action   action
	}{
		{
			title: "Add redirect",
			redirect: TrafficRedirect{
				Iface:           "eth0",
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			action: ADD,
		},
		{
			title: "Delete redirect",
			redirect: TrafficRedirect{
				Iface:           "eth0",
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			action: DELETE,
		},
	}

	for _, tc := range TestCases {
		t.Run(tc.title, func(t *testing.T) {
			cmd := tc.redirect.buildRedirectCmd(tc.action)
			if cmd == nil {
				t.Errorf("command expected but none returned")
				return
			}

			if !strings.ContainsAny(cmd.String(), string(tc.action)) {
				t.Errorf("invalid iptables action")
				return
			}

			destination := fmt.Sprintf("--dport %d", tc.redirect.DestinationPort)
			if !strings.ContainsAny(cmd.String(), destination) {
				t.Errorf("invalid iptables destination")
				return
			}

			redirect := fmt.Sprintf("--to-port %d", tc.redirect.DestinationPort)
			if !strings.ContainsAny(cmd.String(), redirect) {
				t.Errorf("invalid iptables destination")
				return
			}
		})
	}
}

// tests the construction of reset commands from a valid TrafficRedirect
func Test_buildResetCommand(t *testing.T) {
	TestCases := []struct {
		title    string
		redirect TrafficRedirect
		action   action
		port     uint
	}{
		{
			title: "Add reset redirect",
			redirect: TrafficRedirect{
				Iface:           "eth0",
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			action: ADD,
			port:   80,
		},
		{
			title: "Delete reset redirect",
			redirect: TrafficRedirect{
				Iface:           "eth0",
				DestinationPort: 80,
				RedirectPort:    8080,
			},
			action: DELETE,
			port:   80,
		},
	}

	for _, tc := range TestCases {
		t.Run(tc.title, func(t *testing.T) {
			cmd := tc.redirect.buildResetCmd(tc.action, tc.port)
			if cmd == nil {
				t.Errorf("command expected but none returned")
				return
			}

			if !strings.ContainsAny(cmd.String(), string(tc.action)) {
				t.Errorf("invalid iptables action")
				return
			}

			destination := fmt.Sprintf("--dport %d", tc.port)
			if !strings.ContainsAny(cmd.String(), destination) {
				t.Errorf("invalid iptables destination")
				return
			}
		})
	}
}
