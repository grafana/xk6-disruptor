package network

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/iptables"
)

// Test_rules checks that the queue returns the correct rules for a given config and disruption.
func Test_DisruptorRules(t *testing.T) {
	t.Parallel()

	d := Disruptor{
		Filter: Filter{
			Port:     6666,
			Protocol: "tcp",
		},
	}

	actual := d.rules()
	expected := []iptables.Rule{
		{
			Table: "filter", Chain: "INPUT",
			Args: "-p tcp --dport 6666 -j DROP",
		},
	}

	if diff := cmp.Diff(actual, expected); diff != "" {
		t.Fatalf("Generated rules do not match expected:\n%s", diff)
	}
}
