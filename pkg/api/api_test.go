package api

import (
	"context"
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/grafana/xk6-disruptor/pkg/disruptors"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
	"go.k6.io/k6/js/common"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// test environment
type testEnv struct {
	rt  *goja.Runtime
	k8s kubernetes.Kubernetes
}

// a function that constructs an object
type constructor func(*testEnv, goja.ConstructorCall) (*goja.Object, error)

// registers a constructor with a name in the environment's runtime
func (env *testEnv) registerConstructor(name string, constructor constructor) error {
	var object *goja.Object
	var err error
	err = env.rt.Set(name, func(c goja.ConstructorCall) *goja.Object {
		object, err = constructor(env, c)
		if err != nil {
			common.Throw(env.rt, fmt.Errorf("error creating %s: %w", name, err))
		}
		return object
	})
	return err
}

func testSetup() (*testEnv, error) {
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	k8s, err := kubernetes.NewFakeKubernetes(fake.NewSimpleClientset())
	if err != nil {
		return nil, err
	}
	return &testEnv{
		rt:  rt,
		k8s: k8s,
	}, nil
}

func Test_PodDisruptorConstructor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		script      string
		expectError bool
	}{
		{
			description: "valid constructor",
			script: `
			const selector = {
				namespace: "namespace",
				select: {
					labels: {
						app: "app"
					}
				}
			}
			const opts = {
				injectTimeout: 0
			}
			new PodDisruptor(selector, opts)
			`,
			expectError: false,
		},
		{
			description: "valid constructor without options",
			script: `
			const selector = {
				namespace: "namespace",
				select: {
					labels: {
						app: "app"
					}
				}
			}
			new PodDisruptor(selector)
			`,
			expectError: false,
		},
		{
			description: "invalid constructor without selector",
			script: `
			new PodDisruptor()
			`,
			expectError: true,
		},
		{
			description: "invalid constructor with malformed selector",
			script: `
			const selector = {
				namespace: "namespace",
				labels: {
					app: "app"
				}
			}
			new PodDisruptor(selector)
			`,
			expectError: true,
		},
		{
			description: "invalid constructor with malformed options",
			script: `
			const selector = {
				namespace: "namespace",
				select: {
					labels: {
						app: "app"
					}
				}
			}
			const opts = {
				timeout: 0
			}
			new PodDisruptor(selector, opts)
			`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			env, err := testSetup()
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			err = env.registerConstructor("PodDisruptor", func(e *testEnv, c goja.ConstructorCall) (*goja.Object, error) {
				return NewPodDisruptor(e.rt, c, e.k8s)
			})
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			_, err = env.rt.RunString(tc.script)

			if !tc.expectError && err != nil {
				t.Errorf("failed %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}
		})
	}
}

const setupPodDisruptor = `
	const selector = {
	namespace: "namespace",
	select: {
		labels: {
			app: "app"
		}
	}
}
const opts = {
	injectTimeout: 0
}
const d = new PodDisruptor(selector, opts)
`

func Test_JsPodDisruptor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		script      string
		expectError bool
	}{
		{
			description: "get targets",
			script: `
			d.targets()
			`,
			expectError: false,
		},
		{
			description: "inject HTTP Fault with full arguments",
			script: `
			const fault = {
				errorRate: 1.0,
				errorCode: 500,
				averageDelay: 100,
				delayVariation: 10,
				errorBody: '',
				exclude: "",
				port: 80
			}

			const faultOpts = {
				proxyPort: 8080,
				iface: "eth0"
			}

			d.injectHTTPFaults(fault, 1, faultOpts)
			`,
			expectError: false,
		},
		{
			description: "inject HTTP Fault without options",
			script: `
			const fault = {
				errorRate: 1.0,
				errorCode: 500,
				averageDelay: 100,
				delayVariation: 10,
				errorBody: '',
				exclude: "",
				port: 80
			}

			d.injectHTTPFaults(fault, 1)
			`,
			expectError: false,
		},
		{
			description: "inject HTTP Fault without duration",
			script: `
			const fault = {
				errorRate: 1.0,
				errorCode: 500,
				averageDelay: 100,
				delayVariation: 10,
				errorBody: '',
				exclude: "",
				port: 80
			}

			d.injectHTTPFaults(fault)
			`,
			expectError: true,
		},
		{
			description: "inject HTTP Fault with malformed fault (misspelled field)",
			script: `
			const fault = {
				errorRate: 1.0,
				error: 500,       // this is should be 'errorCode'
			}
			
			d.injectHTTPFaults(fault, 1)
			`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()

			env, err := testSetup()
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			err = env.registerConstructor("PodDisruptor", func(e *testEnv, c goja.ConstructorCall) (*goja.Object, error) {
				return NewPodDisruptor(e.rt, c, e.k8s)
			})
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			_, err = env.rt.RunString(setupPodDisruptor)
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			_, err = env.rt.RunString(tc.script)

			if !tc.expectError && err != nil {
				t.Errorf("failed %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}
		})
	}
}

func Test_ServiceDisruptorConstructor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		description string
		script      string
		expectError bool
	}{
		{
			description: "valid constructor",
			script: `
			const opts = {
				injectTimeout: 0
			}
			new ServiceDisruptor("service", "default", opts)
			`,
			expectError: false,
		},
		{
			description: "valid constructor without options",
			script: `
			new ServiceDisruptor("service", "default")
			`,
			expectError: false,
		},
		{
			description: "invalid constructor without namespace",
			script: `
			new ServiceDisruptor("service")
			`,
			expectError: true,
		},
		{
			description: "invalid constructor service name is not string",
			script: `
			new ServiceDisruptor(1, "default")
			`,
			expectError: true,
		},
		{
			description: "invalid constructor namespace is not string",
			script: `
			new ServiceDisruptor("service", {})
			`,
			expectError: true,
		},
		{
			description: "invalid constructor without arguments",
			script: `
			new ServiceDisruptor()
			`,
			expectError: true,
		},
		{
			description: "valid constructor malformed options",
			script: `
			const opts = {
				timeout: 0
			}
			new ServiceDisruptor("service", "default", opts)
			`,
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.description, func(t *testing.T) {
			t.Parallel()
			env, err := testSetup()
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			err = env.registerConstructor("ServiceDisruptor", func(e *testEnv, c goja.ConstructorCall) (*goja.Object, error) {
				return NewServiceDisruptor(e.rt, c, e.k8s)
			})
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			// create a service because the ServiceDisruptor's constructor expects it to exist
			svc := builders.NewServiceBuilder("service").Build()
			_, _ = env.k8s.CoreV1().Services("default").Create(context.TODO(), svc, v1.CreateOptions{})

			value, err := env.rt.RunString(tc.script)

			if !tc.expectError && err != nil {
				t.Errorf("failed %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			// failed and it was expected, it is ok
			if tc.expectError && err != nil {
				return
			}

			var sd disruptors.ServiceDisruptor
			err = env.rt.ExportTo(value, &sd)
			if err != nil {
				t.Errorf("returned valued cannot be converted to ServiceDisruptor: %v", err)
				return
			}
		})
	}
}
