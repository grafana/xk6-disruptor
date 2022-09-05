package cluster

import (
	"testing"
)

const defaultConfig = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane`

const configWithNodePort = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 32080
    hostPort: 8080
    listenAddress: "0.0.0.0"
    protocol: tcp`

const configWithWorkers = `
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
- role: worker
- role: worker`

func Test_CreateConfig(t *testing.T) {
	testCases := []struct {
		title          string
		options        ClusterOptions
		expectError    bool
		expectedConfig string
	}{
		{
			title:          "Default config",
			options:        ClusterOptions{},
			expectError:    false,
			expectedConfig: defaultConfig,
		},
		{
			title: "Node Ports",
			options: ClusterOptions{
				NodePorts: []NodePort{
					{
						NodePort: 32080,
						HostPort: 8080,
					},
				},
			},
			expectError:    false,
			expectedConfig: configWithNodePort,
		},
		{
			title: "Worker nodes",
			options: ClusterOptions{
				Workers: 2,
			},
			expectError:    false,
			expectedConfig: configWithWorkers,
		},
		{
			title: "Custom config",
			options: ClusterOptions{
				Config: baseConfig,
			},
			expectError:    false,
			expectedConfig: baseConfig,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.title, func(t *testing.T) {
			config, err := buildClusterConfig(tc.options)

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
			}

			if config != tc.expectedConfig {
				t.Errorf("Actual config is not the expected\nActual:\n%s\nExpected:\n%s\n", config, tc.expectedConfig)
			}
		})
	}
}
