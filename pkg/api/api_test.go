package api

import (
	"errors"
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/grafana/xk6-disruptor/pkg/disruptors"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"go.k6.io/k6/js/common"
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

			var pd disruptors.PodDisruptor
			err = env.rt.ExportTo(value, &pd)
			if err != nil {
				t.Errorf("returned valued cannot be converted to PodDisruptor: %v", err)
				return
			}
		})
	}
}
