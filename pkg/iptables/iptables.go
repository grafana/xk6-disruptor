// Package iptables implements helpers for manipulating the iptables.
// Requires the iptables command to be installed.
// Requires 'NET_ADMIN' capabilities for manipulating the iptables.
package iptables

import (
	"fmt"
	"strings"

	"github.com/grafana/xk6-disruptor/pkg/utils/process"
)

type action string

const (
	ADD    action = "-A"
	DELETE action = "-D"
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

// TrafficRedirector defines the interface for a traffic redirector
type TrafficRedirector interface {
	// Start initiates the redirection of traffic and resets existing connections
	Start() error
	// Stop restores the traffic to the original target and resets existing connections
	// to the redirection target
	Stop() error
}

// trafficRedirect defines an instance of a TrafficRedirector
type redirector struct {
	*TrafficRedirectionSpec
	executor process.ProcessExecutor
}

// TrafficRedirectorConfig defines the options for creating a TrafficRedirector
type TrafficRedirectorConfig struct {
	Executor process.ProcessExecutor
}

// Creating instances passing a TrafficRedirectorConfig
func newTrafficRedirectorWithConfig(
	tr *TrafficRedirectionSpec,
	config TrafficRedirectorConfig,
) (TrafficRedirector, error) {
	if tr.DestinationPort == 0 || tr.RedirectPort == 0 {
		return nil, fmt.Errorf("the DestinationPort and RedirectPort must be specified")
	}

	if tr.DestinationPort == tr.RedirectPort {
		return nil, fmt.Errorf("the DestinationPort and RedirectPort must be different")
	}

	if tr.Iface == "" {
		return nil, fmt.Errorf("the Iface must be specified")
	}

	return &redirector{
		TrafficRedirectionSpec: tr,
		executor:               config.Executor,
	}, nil
}

// NewTrafficRedirector creates an instance of a TrafficRedirector with default configuration
func NewTrafficRedirector(tf *TrafficRedirectionSpec) (TrafficRedirector, error) {
	config := TrafficRedirectorConfig{
		Executor: process.DefaultProcessExecutor(),
	}
	return newTrafficRedirectorWithConfig(tf, config)
}

// buildRedirectCmd builds a command for adding or removing a transparent proxy using iptables
func (tr *redirector) execRedirectCmd(a action) error {
	cmd := fmt.Sprintf(
		redirectCommand,
		string(a),
		tr.Iface,
		tr.DestinationPort,
		tr.RedirectPort,
	)

	out, err := tr.executor.Exec("iptables", strings.Split(cmd, " ")...)
	if err != nil {
		return fmt.Errorf("error executing iptables command: %s", string(out))
	}

	return nil
}

func (tr *redirector) execResetCmd(a action, port uint) error {
	cmd := fmt.Sprintf(
		resetCommand,
		string(a),
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
	_ = tr.execResetCmd(DELETE, tr.RedirectPort)
	err := tr.execRedirectCmd(ADD)
	if err != nil {
		return err
	}
	return tr.execResetCmd(ADD, tr.DestinationPort)
}

// Stops removes the TrafficRedirect
func (tr *redirector) Stop() error {
	err := tr.execRedirectCmd(DELETE)
	if err != nil {
		return err
	}

	err = tr.execResetCmd(ADD, tr.RedirectPort)
	if err != nil {
		return err
	}

	return tr.execResetCmd(DELETE, tr.DestinationPort)
}
