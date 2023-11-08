package iptables_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/iptables"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

func Test_RulesetAddsRemovesRules(t *testing.T) {
	t.Parallel()

	exec := runtime.NewFakeExecutor(nil, nil)
	ruleset := iptables.NewRuleSet(iptables.New(exec))

	// Add two rules
	err := ruleset.Add(iptables.Rule{
		Table: "table1", Chain: "CHAIN1", Args: "--foo foo --bar bar",
	})
	if err != nil {
		t.Fatalf("error adding rule: %v", err)
	}

	err = ruleset.Add(iptables.Rule{
		Table: "table2", Chain: "CHAIN2", Args: "--boo boo --baz baz",
	})
	if err != nil {
		t.Fatalf("error adding rule: %v", err)
	}

	// Check we have run the expected commands.
	expectedAddCmds := []string{
		"iptables -t table1 -A CHAIN1 --foo foo --bar bar",
		"iptables -t table2 -A CHAIN2 --boo boo --baz baz",
	}

	if diff := cmp.Diff(exec.CmdHistory(), expectedAddCmds); diff != "" {
		t.Fatalf("Executed commands to add rules do not match expected:\n%s", diff)
	}

	exec.Reset()

	// Remove the rules.
	err = ruleset.Remove()
	if err != nil {
		t.Fatalf("error removing rules: %v", err)
	}

	// Check we have run the expected commands.
	expectedRemoveCmds := []string{
		"iptables -t table1 -D CHAIN1 --foo foo --bar bar",
		"iptables -t table2 -D CHAIN2 --boo boo --baz baz",
	}

	if diff := cmp.Diff(exec.CmdHistory(), expectedRemoveCmds); diff != "" {
		t.Fatalf("Executed commands to remove rules do not match expected:\n%s", diff)
	}

	exec.Reset()

	// After removing the rules, add a new one.
	err = ruleset.Add(iptables.Rule{
		Table: "table3", Chain: "CHAIN3", Args: "--zoo zoo --zap zap",
	})
	if err != nil {
		t.Fatalf("error adding rule: %v", err)
	}

	// Check we run the expected add command.
	expectedAddCmds = []string{
		"iptables -t table3 -A CHAIN3 --zoo zoo --zap zap",
	}
	if diff := cmp.Diff(exec.CmdHistory(), expectedAddCmds); diff != "" {
		t.Fatalf("Executed commands to add rules do not match expected:\n%s", diff)
	}

	exec.Reset()

	// Remove the rule.
	err = ruleset.Remove()
	if err != nil {
		t.Fatalf("error removing rules: %v", err)
	}

	// Check we have run the expected command.
	expectedRemoveCmds = []string{
		"iptables -t table3 -D CHAIN3 --zoo zoo --zap zap",
	}

	if diff := cmp.Diff(exec.CmdHistory(), expectedRemoveCmds); diff != "" {
		t.Fatalf("Executed commands to remove rules do not match expected:\n%s", diff)
	}
}
