// Package consts provide constants used by other packages to ensure consistency.
// Some of these constants are defined as variables to allow the value to be set at build time
package consts

// Version contains the current semantic version of k6.
// The value is set when building the binary
var Version = "" //nolint:gochecknoglobals

// AgentImage returns the name of the agent image that corresponds to
// this version of the extension.
func AgentImage() string {
	tag := "latest"
	if Version != "" {
		tag = Version
	}

	return "ghcr.io/grafana/xk6-disruptor-agent:" + tag
}
