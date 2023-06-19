// Package iptables implements helpers for manipulating the iptables.
// Requires the iptables command to be installed.
// Requires 'NET_ADMIN' capabilities for manipulating the iptables.
package iptables

import (
	"fmt"
	"strings"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

const redirectCommand = "%s PREROUTING -t nat -i %s -p tcp --dport %d -j REDIRECT --to-port %d"

const resetCommand = "%s INPUT -i %s -p tcp --dport %d -m state --state ESTABLISHED -j REJECT --reject-with tcp-reset"

// TrafficRedirectionSpec specifies the redirection of traffic to a destination
type TrafficRedirectionSpec struct {
	// Interface on which the traffic will be intercepted
	Iface string
	// Destination port of the traffic to be redirected
	DestinationPort uint
	// Port the traffic will be redirected to
	RedirectPort uint
}

// trafficRedirect defines an instance of a TrafficRedirector
type redirector struct {
	*TrafficRedirectionSpec
	executor runtime.Executor
}

// NewTrafficRedirector creates instances of an iptables traffic redirector
func NewTrafficRedirector(
	tr *TrafficRedirectionSpec,
	executor runtime.Executor,
) (protocol.TrafficRedirector, error) {
	if tr.DestinationPort == 0 || tr.RedirectPort == 0 {
		return nil, fmt.Errorf("the DestinationPort and RedirectPort must be specified")
	}

	if tr.DestinationPort == tr.RedirectPort {
		return nil, fmt.Errorf(
			"the DestinationPort (%d) and RedirectPort (%d) must be different",
			tr.DestinationPort,
			tr.RedirectPort,
		)
	}

	if tr.Iface == "" {
		return nil, fmt.Errorf("the Iface must be specified")
	}

	return &redirector{
		TrafficRedirectionSpec: tr,
		executor:               executor,
	}, nil
}

// delete iptables rules for redirection
func (tr *redirector) deleteRedirectRules() error {
	return tr.execRedirectCmd("-D")
}

// add iptables rules for redirection
func (tr *redirector) addRedirectRules() error {
	return tr.execRedirectCmd("-A")
}

// add iptables rules for reset connections to port
func (tr *redirector) addResetRules(port uint) error {
	return tr.execResetCmd("-A", port)
}

// delete iptables rules for reset connections to port
func (tr *redirector) deleteResetRules(port uint) error {
	return tr.execResetCmd("-D", port)
}

// buildRedirectCmd builds a command for adding or removing a transparent proxy using iptables
func (tr *redirector) execRedirectCmd(action string) error {
	cmd := fmt.Sprintf(
		redirectCommand,
		action,
		tr.Iface,
		tr.DestinationPort,
		tr.RedirectPort,
	)

	out, err := tr.executor.Exec("iptables", strings.Split(cmd, " ")...)
	if err != nil {
		return fmt.Errorf("error executing iptables command: %w %s", err, string(out))
	}

	return nil
}

func (tr *redirector) execResetCmd(action string, port uint) error {
	cmd := fmt.Sprintf(
		resetCommand,
		action,
		tr.Iface,
		port,
	)

	out, err := tr.executor.Exec("iptables", strings.Split(cmd, " ")...)
	if err != nil {
		return fmt.Errorf("error executing iptables command: %s", string(out))
	}

	return nil
}

// Starts applies the TrafficRedirect
func (tr *redirector) Start() error {
	// error is ignored as the rule may not exist
	_ = tr.deleteResetRules(tr.RedirectPort)
	if err := tr.addRedirectRules(); err != nil {
		return err
	}
	return tr.addResetRules(tr.DestinationPort)
}

// Stops removes the TrafficRedirect
func (tr *redirector) Stop() error {
	err := tr.deleteRedirectRules()
	if err != nil {
		return err
	}

	err = tr.addResetRules(tr.RedirectPort)
	if err != nil {
		return err
	}

	return tr.deleteResetRules(tr.DestinationPort)
}
