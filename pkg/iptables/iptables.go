// Package iptables implements helpers for manipulating the iptables.
// Requires the iptables command to be installed.
// Requires 'NET_ADMIN' capabilities for manipulating the iptables.
package iptables

import (
	"errors"
	"fmt"
	"strings"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
	"github.com/grafana/xk6-disruptor/pkg/iproute"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

// redirectRule is a netfilter rule that redirects traffic destined to the application's port to the proxy.
// As per https://unix.stackexchange.com/a/112232/39698, traffic coming from non-local addresses cannot be redirected
// to local addresses, so the proxy is required to listen on, in addition to localhost, the same address where the
// application also listens on (typically `eth0`).
// As per https://upload.wikimedia.org/wikipedia/commons/3/37/Netfilter-packet-flow.svg, locally originated traffic,
// such as port-forwarded or traffic generated from inside the container, does not traverse PREROUTING. Likewise,
// traffic coming from the outside of the pod does not traverse OUTPUT, so this rule has to be instantiated for both
// OUTPUT and PREROUTING chains.
//
// As a technicality, the PREROUTING rule does not actually need `! -s %s/32`, as traffic from the proxy to upstream
// does not traverse PREROUTING but OUTPUT. We keep it anyway for consistency.
const redirectRule = "%s %s -t nat " + // For traffic traversing the nat table
	"-p tcp --dport %s " + // Sent to the upstream application's port
	"! -s %s/32 " + // Unless it is sent from the proxy IP address
	"-j REDIRECT --to-port %s" // Forward it to the proxy address

// redirectChains are the chains in which the redirectRule should be added. see redirectRule for details.
//
//nolint:gochecknoglobals // Read-only constant-ish.
var redirectChains = []string{"OUTPUT", "PREROUTING"}

// resetCommand creates an iptables rule that closes existing and new connections to the upstream application's port,
// forcing clients to re-establish connections.
const resetCommand = "%s INPUT " + // For traffic traversing the INPUT chain
	"! -s %s/32 " + // Not coming from the proxy address
	"-p tcp --dport %s " + // Directed to the upstream application's port
	"-m state --state ESTABLISHED " + // That are already ESTABLISHED, i.e. not before they are redirected
	"-j REJECT --reject-with tcp-reset" // Reject it

// cidrSuffix is the CIDR prefix length suffix appended to LocalAddress before adding it.
const cidrSuffix = "/32"

// TrafficRedirectionSpec specifies the redirection of traffic to a destination
type TrafficRedirectionSpec struct {
	// LocalAddress is the IP address the proxy will use to send requests, which is excluded from redirection.
	// This address is added to the interface specified below at the start of the disruption, and removed at the end.
	// LocalAddress must be specified in CIDR notation.
	LocalAddress string
	// Interface is the interface where LocalAddress is added.
	Interface string
	// ProxyPort is the port where the proxy is listening at.
	ProxyPort string
	// TargetPort is the port of for the upstream application.
	TargetPort string
}

// trafficRedirect defines an instance of a TrafficRedirector
type redirector struct {
	*TrafficRedirectionSpec
	ip       iproute.IPRoute
	executor runtime.Executor
}

// NewTrafficRedirector creates instances of an iptables traffic redirector
func NewTrafficRedirector(
	tr *TrafficRedirectionSpec,
	executor runtime.Executor,
) (protocol.TrafficRedirector, error) {
	if tr.TargetPort == "" || tr.ProxyPort == "" {
		return nil, fmt.Errorf("TargetPort and ProxyPort must be specified")
	}

	if tr.TargetPort == tr.ProxyPort {
		return nil, fmt.Errorf("TargetPort (%s) and ProxyPort (%s) must be different", tr.TargetPort, tr.ProxyPort)
	}

	if tr.LocalAddress == "" || tr.Interface == "" {
		return nil, fmt.Errorf("local address and interface must be specified")
	}

	return &redirector{
		TrafficRedirectionSpec: tr,
		executor:               executor,
		ip:                     iproute.New(executor),
	}, nil
}

// delete iptables rules for redirection
// Delete commands are all executed regardless of whether the previous failed.
func (tr *redirector) deleteRedirectRules() error {
	errs := make([]error, 0, len(redirectChains))
	for _, chain := range redirectChains {
		errs = append(errs, tr.execRedirectCmd("-D", chain))
	}

	return errorsJoin(errs...)
}

// add iptables rules for redirection
func (tr *redirector) addRedirectRules() error {
	for _, chain := range redirectChains {
		if err := tr.execRedirectCmd("-A", chain); err != nil {
			return err
		}
	}

	return nil
}

// add iptables rules for reset connections to port
func (tr *redirector) addResetRules() error {
	return tr.execResetCmd("-A")
}

// delete iptables rules for reset connections to port
func (tr *redirector) deleteResetRules() error {
	return tr.execResetCmd("-D")
}

// buildRedirectCmd builds a command for adding or removing a transparent proxy using iptables
func (tr *redirector) execRedirectCmd(action, chain string) error {
	cmd := fmt.Sprintf(
		redirectRule,
		action,
		chain,
		tr.TargetPort,
		tr.LocalAddress,
		tr.ProxyPort,
	)

	out, err := tr.executor.Exec("iptables", strings.Split(cmd, " ")...)
	if err != nil {
		return fmt.Errorf("error executing iptables command %q: %w %s", cmd, err, string(out))
	}

	return nil
}

func (tr *redirector) execResetCmd(action string) error {
	cmd := fmt.Sprintf(
		resetCommand,
		action,
		tr.LocalAddress,
		tr.TargetPort,
	)

	out, err := tr.executor.Exec("iptables", strings.Split(cmd, " ")...)
	if err != nil {
		return fmt.Errorf("error executing iptables command %q: %w %s", cmd, err, string(out))
	}

	return nil
}

// Start applies the TrafficRedirect
func (tr *redirector) Start() error {
	cidrAddr := tr.LocalAddress + cidrSuffix
	if err := tr.ip.Add(cidrAddr, tr.Interface); err != nil {
		return fmt.Errorf("adding ip address: %w", err)
	}

	if err := tr.addRedirectRules(); err != nil {
		return err
	}

	return tr.addResetRules()
}

// Stop stops the TrafficRedirect
func (tr *redirector) Stop() error {
	cidrAddr := tr.LocalAddress + cidrSuffix

	// Cleanup steps are all performed regardless of intermediate errors, as they are idempotent.
	redirErr := tr.deleteRedirectRules()
	resetErr := tr.deleteResetRules()
	addrDelErr := tr.ip.Delete(cidrAddr, tr.Interface)

	return errorsJoin(redirErr, resetErr, addrDelErr)
}

// errorsJoin is a hacky implementation of errors.Join, which does not wrap errors.
// TODO: Replace this homemade error aggregation with errors.Join when we upgrade from Go 1.19 to 1.20.
func errorsJoin(errs ...error) error {
	var composite string

	for _, err := range errs {
		if err != nil {
			composite = fmt.Sprintf("%s%v; ", composite, err)
		}
	}

	if composite == "" {
		return nil
	}

	return errors.New(strings.TrimSuffix(composite, "; "))
}
