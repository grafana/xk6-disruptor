package helpers

import (
	"sync"

	"k8s.io/client-go/kubernetes"
)

// Command records the execution of a command in a Pod
type Command struct {
	Pod       string
	Container string
	Command   []string
	Stdin     []byte
}

// FakePodCommandExecutor mocks the execution of a command in a pod
// recording the command history and returning a predefined stdout, stderr, and error
type FakePodCommandExecutor struct {
	mutex   sync.Mutex
	history []Command
	stdout  []byte
	stderr  []byte
	err     error
}

// Exec records the execution of a command and returns the pre-defined
func (f *FakePodCommandExecutor) Exec(
	pod string,
	container string,
	cmd []string,
	stdin []byte,
) ([]byte, []byte, error) {
	f.mutex.Lock()
	f.history = append(f.history, Command{
		Pod:       pod,
		Container: container,
		Command:   cmd,
		Stdin:     stdin,
	})
	f.mutex.Unlock()

	return f.stdout, f.stderr, f.err
}

// SetResult sets the results to be returned for each invocation to the FakePodCommandExecutor
func (f *FakePodCommandExecutor) SetResult(stdout []byte, stderr []byte, err error) {
	f.stdout = stdout
	f.stderr = stderr
	f.err = err
}

// GetHistory returns the history of commands executed by the FakePodCommandExecutor
func (f *FakePodCommandExecutor) GetHistory() []Command {
	return f.history
}

// NewFakePodCommandExecutor creates a new instance of FakePodCommandExecutor
// with default attributes
func NewFakePodCommandExecutor() *FakePodCommandExecutor {
	return &FakePodCommandExecutor{}
}

// fakePodHelper is an fake instance of a PodHelpers. It can delegate to the actual
// helper the execution of actions or override them when needed
type fakePodHelper struct {
	PodHelper
	executor *FakePodCommandExecutor
}

// NewFakePodHelper creates a set of a FakePodHelper on the default namespace
func NewFakePodHelper(client kubernetes.Interface, namespace string, executor *FakePodCommandExecutor) PodHelper {
	h := NewPodHelper(client, nil, namespace)
	return &fakePodHelper{
		h,
		executor,
	}
}

// Fakes the execution of a command in a pod
func (f *fakePodHelper) Exec(pod string, container string, command []string, stdin []byte) ([]byte, []byte, error) {
	return f.executor.Exec(pod, container, command, stdin)
}

// fakePodHelper is an fake instance of a PodHelpers. It can delegate to the actual
// helper the execution of actions or override them when needed
type fakeServiceHelper struct {
	ServiceHelper
	executor *FakePodCommandExecutor
}

// NewFakeServiceHelper creates a set of a FakeServiceHelper on the default namespace
func NewFakeServiceHelper(
	client kubernetes.Interface,
	namespace string,
	executor *FakePodCommandExecutor,
) ServiceHelper {
	h := NewServiceHelper(client, nil, namespace)
	return &fakeServiceHelper{
		h,
		executor,
	}
}
