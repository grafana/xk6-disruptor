// Package disruptor implement the k6 extension interface for calling disruptors from js scripts
// running in the goya runtime
package disruptor

import (
	"fmt"
	"os"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-disruptor/pkg/api/disruptors"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
)

func init() {
	modules.Register("k6/x/disruptor", new(RootModule))
}

// RootModule is the global module object type. It is instantiated once per test
// run and will be used to create `k6/x/disruptor` module instances for each VU.
type RootModule struct{}

// ModuleInstance represents an instance of the JS module.
type ModuleInstance struct {
	vu modules.VU
	// instance of a Kubernetes helper
	k8s kubernetes.Kubernetes
}

// Ensure the interfaces are implemented correctly.
var (
	_ modules.Module   = &RootModule{}
	_ modules.Instance = &ModuleInstance{}
)

// NewModuleInstance returns a new instance of the disruptor module for each VU.
func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	kubeconfig := os.Getenv("KUBECONFIG")
	cfg := kubernetes.Config{
		Context:    vu.Context(),
		Kubeconfig: kubeconfig,
	}
	k8s, err := kubernetes.NewFromConfig(cfg)
	if err != nil {
		common.Throw(vu.Runtime(), fmt.Errorf("error creating Kubernetes helper: %w", err))
	}
	return &ModuleInstance{
		vu:  vu,
		k8s: k8s,
	}
}

// Exports implements the modules.Instance interface and returns the exports
// of the JS module.
func (m *ModuleInstance) Exports() modules.Exports {
	return modules.Exports{
		Named: map[string]interface{}{
			"PodDisruptor":     m.newPodDisruptor,
			"ServiceDisruptor": m.newServiceDisruptor,
		},
	}
}

// creates an instance of a PodDisruptor
func (m *ModuleInstance) newPodDisruptor(c goja.ConstructorCall) *goja.Object {
	rt := m.vu.Runtime()

	selector := disruptors.PodSelector{}
	err := rt.ExportTo(c.Argument(0), &selector)
	if err != nil {
		common.Throw(rt,
			fmt.Errorf("PodDisruptor constructor expects PodSelector as argument: %w", err))
	}

	options := disruptors.PodDisruptorOptions{}
	err = rt.ExportTo(c.Argument(1), &options)
	if err != nil {
		common.Throw(rt,
			fmt.Errorf("PodDisruptor constructor expects PodDisruptorOptions as second argument: %w", err))
	}
	disruptor, err := disruptors.NewPodDisruptor(m.k8s, selector, options)
	if err != nil {
		common.Throw(rt, fmt.Errorf("error creating PodDisruptor: %w", err))
	}

	return rt.ToValue(disruptor).ToObject(rt)
}

// creates an instance of a ServiceDisruptor
func (m *ModuleInstance) newServiceDisruptor(c goja.ConstructorCall) *goja.Object {
	rt := m.vu.Runtime()

	var service string
	err := rt.ExportTo(c.Argument(0), &service)
	if err != nil {
		common.Throw(rt,
			fmt.Errorf("ServiceDisruptor constructor expects service name (string) as first argument: %w", err))
	}

	var namespace string
	err = rt.ExportTo(c.Argument(1), &namespace)
	if err != nil {
		common.Throw(rt,
			fmt.Errorf("ServiceDisruptor constructor expects namespace (string) as second argument: %w", err))
	}

	options := disruptors.ServiceDisruptorOptions{}
	// options argument is optional
	if len(c.Arguments) > 2 {
		err = rt.ExportTo(c.Argument(2), &options)
		if err != nil {
			common.Throw(rt,
				fmt.Errorf("PodDisruptor constructor expects PodDisruptorOptions as third argument: %w", err))
		}
	}

	disruptor, err := disruptors.NewServiceDisruptor(m.k8s, service, namespace, options)
	if err != nil {
		common.Throw(rt, fmt.Errorf("error creating ServiceDisruptor: %w", err))
	}

	return rt.ToValue(disruptor).ToObject(rt)
}
