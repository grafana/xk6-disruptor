package iptables

import (
	"fmt"
	"strings"

	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

// Iptables adds and removes iptables rules by executing the `iptables` binary.
// Add()ed rules are remembered and are automatically removed when Remove is called.
type Iptables struct {
	// Executor is the runtime.Executor used to run the iptables binary.
	executor runtime.Executor

	rules []Rule
}

// New returns a new Iptables ready to use.
func New(executor runtime.Executor) *Iptables {
	return &Iptables{
		executor: executor,
	}
}

// Add adds a rule. Added rule will be remembered and removed later when Remove is called.
func (i *Iptables) Add(r Rule) error {
	err := i.exec(r.add())
	if err != nil {
		return err
	}

	i.rules = append(i.rules, r)

	return nil
}

// Remove removes all added rules. If an error occurs, Remove continues to try and remove remaining rules.
func (i *Iptables) Remove() error {
	var errors []error

	var remaining []Rule
	for _, rule := range i.rules {
		err := i.exec(rule.remove())
		if err != nil {
			errors = append(errors, err)
			remaining = append(remaining, rule)
		}
	}

	i.rules = remaining

	// TODO: Return all errors with errors.Join when we move to go 1.21.
	if len(errors) > 0 {
		return errors[0]
	}

	return nil
}

func (i *Iptables) exec(args string) error {
	out, err := i.executor.Exec("iptables", strings.Split(args, " ")...)
	if err != nil {
		return fmt.Errorf("%w: %q", err, out)
	}

	return nil
}

// Rule is a netfilter/iptables rule.
type Rule struct {
	// Table is the netfilter table to which this rule belongs. It is usually "filter".
	Table string
	// Chain is the netfilter chain to which this rule belongs. Usual values are "INPUT", "OUTPUT".
	Chain string
	// Args is the rest of the netfilter rule.
	Args string
}

func (r Rule) add() string {
	return fmt.Sprintf("-t %s -A %s %s", r.Table, r.Chain, r.Args)
}

func (r Rule) remove() string {
	return fmt.Sprintf("-t %s -D %s %s", r.Table, r.Chain, r.Args)
}
