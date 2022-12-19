// Package disruptor implement the k6 extension interface for calling disruptors from js scripts
// running in the goya runtime
package disruptor

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"

	"github.com/dop251/goja"

	"github.com/grafana/xk6-disruptor/pkg/api"
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
	kubeConfigPath, err := getKubernetesConfigPath()
	if err != nil {
		common.Throw(vu.Runtime(), fmt.Errorf("error getting kubernetes config path: %w", err))
	}
	cfg := kubernetes.Config{
		Context:    vu.Context(),
		Kubeconfig: kubeConfigPath,
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

	disruptor, err := api.NewPodDisruptor(rt, c, m.k8s)
	if err != nil {
		common.Throw(rt, fmt.Errorf("error creating PodDisruptor: %w", err))
	}
	return disruptor
}

// creates an instance of a ServiceDisruptor
func (m *ModuleInstance) newServiceDisruptor(c goja.ConstructorCall) *goja.Object {
	rt := m.vu.Runtime()

	disruptor, err := api.NewServiceDisruptor(rt, c, m.k8s)
	if err != nil {
		common.Throw(rt, fmt.Errorf("error creating ServiceDisruptor: %w", err))
	}

	return disruptor
}

// Copied from ahmetb/kubectx source code:
// https://github.com/ahmetb/kubectx/blob/29850e1a75cb5cad8d93f74a4114311eb9feba9f/internal/kubeconfig/kubeconfigloader.go#L59
func getKubernetesConfigPath() (string, error) {
	// KUBECONFIG env var
	if v := os.Getenv("KUBECONFIG"); v != "" {
		list := filepath.SplitList(v)
		if len(list) > 1 {
			// TODO KUBECONFIG=file1:file2 currently not supported
			return "", errors.New("multiple files in KUBECONFIG are currently not supported")
		}
		return v, nil
	}

	// default path
	home := os.Getenv("HOME")
	if home == "" {
		home = os.Getenv("USERPROFILE") // windows
	}
	if home == "" {
		return "", errors.New("HOME or USERPROFILE environment variable not set")
	}
	return filepath.Join(home, ".kube", "config"), nil
}
