package stressors

import (
	"context"
	"testing"
	"time"
)

func Test_CPUStressor(t *testing.T) {
	t.Parallel()

	opts := ResourceStressOptions{
		Slice: 1 * time.Second,
	}
	s, err := NewResourceStressor(opts)
	if err != nil {
		t.Fatalf("creating stressor %v", err)
	}

	d := ResourceDisruption{
		CPUDisruption: CPUDisruption{
			Load: 80,
			CPUs: 1,
		},
	}
	err = s.Apply(context.TODO(), d, 1*time.Second)
	if err != nil {
		t.Fatalf("applying disruption %v", err)
	}
}
