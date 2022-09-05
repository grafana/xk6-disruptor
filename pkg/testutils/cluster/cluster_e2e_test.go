//go:build e2e
// +build e2e

package cluster

import (
	"testing"
	"time"
)

func Test_DefaultConfig(t *testing.T) {
	// create cluster with default configuration
	c, err := CreateCluster(
		"e2e-default-cluster",
		ClusterOptions{
			Wait: time.Second * 60,
		},
	)

	if err != nil {
		t.Errorf("failed to create cluster: %v", err)
		return
	}

	// delete cluster
	c.Delete()
}
