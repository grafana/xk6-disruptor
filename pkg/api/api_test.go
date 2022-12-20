package api

import (
	"fmt"
	"testing"

	"github.com/dop251/goja"
	"github.com/grafana/xk6-disruptor/pkg/disruptors"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"go.k6.io/k6/js/common"
	"k8s.io/client-go/kubernetes/fake"
)

func testRuntime() (*goja.Runtime, error) {
	rt := goja.New()
	rt.SetFieldNameMapper(common.FieldNameMapper{})

	k8s, err := kubernetes.NewFakeKubernetes(fake.NewSimpleClientset())
	if err != nil {
		return nil, err
	}

	var disruptor *goja.Object
	err = rt.Set("PodDisruptor", func(c goja.ConstructorCall) *goja.Object {
		disruptor, err = NewPodDisruptor(rt, c, k8s)
		if err != nil {
			common.Throw(rt, fmt.Errorf("error creating PodDisruptor: %w", err))
		}
		return disruptor
	})
	if err != nil {
		return nil, err
	}

	return rt, nil
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

			rt, err := testRuntime()
			if err != nil {
				t.Errorf("error in test setup %v", err)
				return
			}

			value, err := rt.RunString(tc.script)

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
			err = rt.ExportTo(value, &pd)
			if err != nil {
				t.Errorf("returned valued cannot be converted to PodDisruptor: %v", err)
				return
			}
		})
	}
}
