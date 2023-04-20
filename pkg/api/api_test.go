package api

import (
	"context"
	"fmt"
	"testing"

	"github.com/dop251/goja"
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
				return NewPodDisruptor(context.TODO(), e.rt, c, e.k8s)
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

// This function tests covers both PodDisruptor and ServiceDisruptor because
// they are wrapped by the same methods in the API.
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
				averageDelay: "100ms",
				delayVariation: "10ms",
				errorBody: '',
				exclude: "",
				port: 80
			}

			const faultOpts = {
				proxyPort: 8080,
				iface: "eth0"
			}

			d.injectHTTPFaults(fault, "1s", faultOpts)
			`,
			expectError: false,
		},
		{
			description: "inject HTTP Fault without options",
			script: `
			const fault = {
				errorRate: 1.0,
				errorCode: 500,
				averageDelay: "100ms",
				delayVariation: "10ms",
				errorBody: '',
				exclude: "",
				port: 80
			}

			d.injectHTTPFaults(fault, "1s")
			`,
			expectError: false,
		},
		{
			description: "inject HTTP Fault without duration",
			script: `
			const fault = {
				errorRate: 1.0,
				errorCode: 500,
				averageDelay: "100ms",
				delayVariation: "10ms",
				errorBody: '',
				exclude: "",
				port: 80
			}

			d.injectHTTPFaults(fault)
			`,
			expectError: true,
		},
		{
			description: "inject HTTP Fault with invalid duration",
			script: `
			const fault = {
				errorRate: 1.0,
				errorCode: 500,
				averageDelay: "100ms",
				delayVariation: "10ms",
				errorBody: '',
				exclude: "",
				port: 80
			}

			d.injectHTTPFaults(fault, "1")  // missing duration unit
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

			d.injectHTTPFaults(fault, "1s")
			`,
			expectError: true,
		},
		{
			description: "inject Grpc Fault with full arguments",
			script: `
			const fault = {
				errorRate: 1.0,
				statusCode: 500,
				statusMessage: '',
				averageDelay: "100ms",
				delayVariation: "10ms",
				exclude: "",
				port: 80
			}

			const faultOpts = {
				proxyPort: 4000,
				iface: "eth0"
			}

			d.injectGrpcFaults(fault, "1s", faultOpts)
			`,
			expectError: false,
		},
		{
			description: "inject Grpc Fault without options",
			script: `
			const fault = {
				errorRate: 1.0,
				statusCode: 500,
				statusMessage: '',
				averageDelay: "100ms",
				delayVariation: "10ms",
				exclude: "",
				port: 80
			}

			d.injectGrpcFaults(fault, "1s")
			`,
			expectError: false,
		},
		{
			description: "inject Grpc Fault without duration",
			script: `
			const fault = {
				errorRate: 1.0,
				statusCode: 500,
				statusMessage: '',
				averageDelay: "100ms",
				delayVariation: "10ms",
				exclude: "",
				port: 80
			}

			const faultOpts = {
				proxyPort: 4000,
				iface: "eth0"
			}

			d.injectGrpcFaults(fault)
			`,
			expectError: true,
		},
		{
			description: "inject Grpc Fault without invalid duration",
			script: `
			const fault = {
				errorRate: 1.0,
				statusCode: 500,
				statusMessage: '',
				averageDelay: "100ms",
				delayVariation: "10ms",
				exclude: "",
				port: 80
			}

			const faultOpts = {
				proxyPort: 4000,
				iface: "eth0"
			}

			d.injectGrpcFaults(fault, "1")    // missing duration unit
			`,
			expectError: true,
		},
		{
			description: "inject Grpc Fault with malformed fault (misspelled field)",
			script: `
			const fault = {
				errorRate: 1.0,
				status: 500,       // this is should be 'statusCode'
			}

			d.injectGrpcFaults(fault, "1m")
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
				return NewPodDisruptor(context.TODO(), e.rt, c, e.k8s)
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
			// do not wait for fault injection
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
				return NewServiceDisruptor(context.TODO(), e.rt, c, e.k8s)
			})
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			// create a k8s resources because the ServiceDisruptor's constructor expects it to exist
			labels := map[string]string{
				"app": "test",
			}
			svc := builders.NewServiceBuilder("service").WithSelector(labels).Build()
			ep := builders.NewEndPointsBuilder("service").Build()

			_, _ = env.k8s.CoreV1().Services("default").Create(context.TODO(), svc, v1.CreateOptions{})
			_, _ = env.k8s.CoreV1().Endpoints("default").Create(context.TODO(), ep, v1.CreateOptions{})

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
