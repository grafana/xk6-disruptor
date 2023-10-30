package tcpconn

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/iptables"
)

// Test_rules checks that the queue returns the correct rules for a given config and disruption.
func Test_NFQueueRules(t *testing.T) {
	t.Parallel()

	q := NFQueue{
		NFQConfig: NFQConfig{
			QueueID:    1,
			RejectMark: 2,
		},
		Disruption: Disruption{
			Port: 6666,
		},
	}

	actual := q.rules()
	expected := []iptables.Rule{
		{
			Table: "filter", Chain: "INPUT",
			Args: "-p tcp --dport 6666 -m mark --mark 2 -j REJECT --reject-with tcp-reset",
		},
		{
			Table: "filter", Chain: "INPUT",
			Args: "-p tcp --dport 6666 -j NFQUEUE --queue-num 1 --queue-bypass",
		},
	}

	if diff := cmp.Diff(actual, expected); diff != "" {
		t.Fatalf("Generated rules do not match expected:\n%s", diff)
	}
}
