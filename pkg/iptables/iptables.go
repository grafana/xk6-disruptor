// Package iptables implements helpers for manipulating the iptables.
// Requires the iptables command to be installed.
// Requires 'NET_ADMIN' capabilities for manipulating the iptables.
package iptables

import (
	"fmt"
	"os/exec"
)

type action string

const (
	ADD    action = "-A"
	DELETE action = "-D"
)

const redirectCommand = "iptables %s PREROUTING -t nat -i %s -p tcp --dport %d -j REDIRECT --to-port %d"

const resetCommand = "iptables %s INPUT -i %s -p tcp --dport %d -m state --state ESTABLISHED -j REJECT --reject-with tcp-reset"

// TrafficRedirect specifies the redirection of traffic to a destination
type TrafficRedirect struct {
	// Interface on which the traffic will be intercepted
	Iface string
	// Destination port of the traffic to be redirected
	DestinationPort uint
	// Port the traffic will be redirected to
	RedirectPort uint
}

func (tr *TrafficRedirect) validate() error {
	if tr.DestinationPort == 0 || tr.RedirectPort == 0 {
		return fmt.Errorf("the DestinationPort and RedirectPort must be specified")
	}

	if tr.DestinationPort == tr.RedirectPort {
		return fmt.Errorf("the DestinationPort and RedirectPort must be different")
	}

	if tr.Iface == "" {
		return fmt.Errorf("the Iface  must be specified")
	}

	return nil
}

// buildRedirectCmd builds a command for adding or removing a transparent proxy using iptables
func (tr *TrafficRedirect) buildRedirectCmd(a action) *exec.Cmd {
	cmd := fmt.Sprintf(
		redirectCommand,
		string(a),
		tr.Iface,
		tr.DestinationPort,
		tr.RedirectPort,
	)
	return exec.Command(cmd)
}

func (tr *TrafficRedirect) buildResetCmd(a action, port uint) *exec.Cmd {
	cmd := fmt.Sprintf(
		resetCommand,
		string(a),
		tr.Iface,
		port,
	)
	return exec.Command(cmd)
}

// Add adds iptables rules for redirecting traffic
func (tr *TrafficRedirect) Redirect() error {
	err := tr.validate()
	if err != nil {
		return err
	}

	err = tr.buildRedirectCmd(ADD).Run()
	if err != nil {
		return err
	}

	err = tr.buildResetCmd(ADD, tr.DestinationPort).Run()
	return err
}

// Delete removes iptables rules for redirecting traffic
// Existing redirected connections are always reset
func (tr *TrafficRedirect) Restore() error {
	err := tr.validate()
	if err != nil {
		return err
	}

	err = tr.buildRedirectCmd(DELETE).Run()
	if err != nil {
		return err
	}

	err = tr.buildResetCmd(ADD, tr.RedirectPort).Run()
	return err
}
