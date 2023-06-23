// Package iproute houses the IPRoute object, which can be used to add and and remove IP addresses from interfaces
// by wrapping the ip(8) command.
package iproute

import (
	"fmt"

	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

// IPRoute implements adding and removing IP addresses from interfaces.
type IPRoute struct {
	exec runtime.Executor
}

// New returns a new IPRoute.
func New(executor runtime.Executor) IPRoute {
	return IPRoute{
		exec: executor,
	}
}

// Add adds the given IP address, in CIDR notation, to the given device.
func (ip IPRoute) Add(addrCidr, dev string) error {
	return ip.addr("add", addrCidr, dev)
}

// Delete removes the given IP address, in CIDR notation, from the given device.
func (ip IPRoute) Delete(addrCidr, dev string) error {
	return ip.addr("del", addrCidr, dev)
}

func (ip IPRoute) addr(operation, addrCidr, dev string) error {
	out, err := ip.exec.Exec("ip", "addr", operation, addrCidr, "dev", dev)
	if err != nil {
		return fmt.Errorf("running ip %s operation: %q: %w", operation, string(out), err)
	}

	return nil
}
