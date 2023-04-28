package helpers

import (
	"io"
	"net/http"
	"strings"
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

// fakeNamespaceHelper is an fake instance of a NamespaceHelper. It can delegate to the actual
// helper the execution of actions or override them when needed
type fakeNamespaceHelper struct {
	NamespaceHelper
}

// NewFakeNamespaceHelper creates a set of a NamespaceHelper on the default namespace
func NewFakeNamespaceHelper(client kubernetes.Interface) NamespaceHelper {
	h := NewNamespaceHelper(client)
	return &fakeNamespaceHelper{
		h,
	}
}

// FakeHTTPClient implement a fake HTTPClient that returns a fixed response.
// When invoked, it records the request it received
type FakeHTTPClient struct {
	Request  *http.Request
	Response *http.Response
	Err      error
}

// newFakeHTTPClient creates a FakeHTTPClient that returns a fixed response from a status and an content body
func newFakeHTTPClient(status int, body []byte) *FakeHTTPClient {
	response := &http.Response{
		Proto:         "HTTP/1.1",
		ProtoMajor:    1,
		ProtoMinor:    1,
		StatusCode:    status,
		Status:        http.StatusText(status),
		Body:          io.NopCloser(strings.NewReader(string(body))),
		ContentLength: int64(len(body)),
	}

	return &FakeHTTPClient{
		Response: response,
		Err:      nil,
	}
}

// Do implements HTTPClient's Do method
func (f *FakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	f.Request = req
	return f.Response, f.Err
}
