package kubernetes

import (
	"context"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"

	"k8s.io/client-go/kubernetes/fake"
)

// FakeKubernetes is a fake implementation of the Kubernetes interface
type FakeKubernetes struct {
	*fake.Clientset
	ctx      context.Context
	executor *helpers.FakePodCommandExecutor
}

// NewFakeKubernetes returns a new fake implementation of Kubernetes from fake Clientset
func NewFakeKubernetes(clientset *fake.Clientset) (*FakeKubernetes, error) {
	return &FakeKubernetes{
		Clientset: clientset,
		ctx:       context.TODO(),
		executor:  helpers.NewFakePodCommandExecutor(),
	}, nil
}

// Context returns the context for executing k8s actions
func (k *FakeKubernetes) Context() context.Context {
	return k.ctx
}

// Helpers return a instance of FakeHelper
func (f *FakeKubernetes) Helpers() helpers.Helpers {
	return helpers.NewFakeHelper(
		f.Clientset,
		"default",
		f.executor,
	)
}

// NamespacedHelpers return a instance of FakeHelper for a given namespace
func (f *FakeKubernetes) NamespacedHelpers(namespace string) helpers.Helpers {
	return helpers.NewFakeHelper(
		f.Clientset,
		namespace,
		f.executor,
	)
}

// GetFakeProcessExecutor returns the FakeProcessExecutor used by the helpers to mock
// the execution of commands in a Pod
func (f *FakeKubernetes) GetFakeProcessExecutor() *helpers.FakePodCommandExecutor {
	return f.executor
}
