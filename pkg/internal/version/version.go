// Package version provide information about the build version
package version

import (
	"runtime/debug"
)

// DisruptorVersion returns the version of the currently executed disruptor
func DisruptorVersion() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		panic("could not read runtime debug info. Package version could not be defined")
	}

	return bi.Main.Version
}

// AgentImage returns the name of the agent image that corresponds to
// this version of the extension.
func AgentImage() string {
	tag := "latest"

	// if a specific version of the disruptor was built, use it for agent's tag
	// (go test sets version to "")
	dv := DisruptorVersion()
	if dv != "" && dv != "(devel)" {
		tag = dv
	}

	return "ghcr.io/grafana/xk6-disruptor-agent:" + tag
}
