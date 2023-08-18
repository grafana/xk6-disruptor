package protocol_test

import (
	"testing"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol"
)

func TestMetricMap(t *testing.T) {
	t.Parallel()

	t.Run("increases counters", func(t *testing.T) {
		t.Parallel()

		const name = "foo_metric"

		mm := protocol.MetricMap{}
		if current := mm.Map(); current[name] != 0 {
			t.Fatalf("map should start containing zero")
		}

		mm.Inc(name)
		if updated := mm.Map(); updated[name] != 1 {
			t.Fatalf("metric was not incremented")
		}
	})
}
