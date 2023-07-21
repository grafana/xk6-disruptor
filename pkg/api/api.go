// Package api implements a layer between javascript code (via goja)) and the disruptors
// allowing for validations and type conversions when needed
//
// The implementation of the JS API follows the design described in
// https://github.com/grafana/xk6-disruptor/blob/fix-context-usage/docs/01-development/design-docs/002-js-api-implementation.md
package api

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/dop251/goja"
	"github.com/grafana/xk6-disruptor/pkg/disruptors"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"go.k6.io/k6/js/common"
)

// TODO: call directly Convert from API methods
func convertValue(_ *goja.Runtime, value goja.Value, target interface{}) error {
	return Convert(value.Export(), target)
}

// buildObject returns the value as a
func buildObject(rt *goja.Runtime, value interface{}) (*goja.Object, error) {
	obj := rt.NewObject()

	t := reflect.TypeOf(value)
	v := reflect.ValueOf(value)
	for i := 0; i < t.NumMethod(); i++ {
		name := t.Method(i).Name
		f := v.MethodByName(name)
		err := obj.Set(toCamelCase(name), f.Interface())
		if err != nil {
			return nil, err
		}
	}

	return obj, nil
}

// jsDisruptor implements the JS interface for Disruptor
type jsDisruptor struct {
	ctx context.Context // this context controls the object's lifecycle
	rt  *goja.Runtime
	disruptors.Disruptor
}

// Targets is a proxy method. Validates parameters and delegates to the PodDisruptor method
func (p *jsDisruptor) Targets() goja.Value {
	targets, err := p.Disruptor.Targets(p.ctx)
	if err != nil {
		common.Throw(p.rt, fmt.Errorf("error getting kubernetes config path: %w", err))
	}

	return p.rt.ToValue(targets)
}

// jsProtocolFaultInjector implements the JS interface for jsProtocolFaultInjector
type jsProtocolFaultInjector struct {
	ctx context.Context // this context controls the object's lifecycle
	rt  *goja.Runtime
	disruptors.ProtocolFaultInjector
}

// injectHTTPFaults is a proxy method. Validates parameters and delegates to the Protocol Disruptor method
func (p *jsProtocolFaultInjector) InjectHTTPFaults(args ...goja.Value) {
	if len(args) < 2 {
		common.Throw(p.rt, fmt.Errorf("HTTPFault and duration are required"))
	}

	fault := disruptors.HTTPFault{}
	err := convertValue(p.rt, args[0], &fault)
	if err != nil {
		common.Throw(p.rt, fmt.Errorf("invalid fault argument: %w", err))
	}

	var duration time.Duration
	err = convertValue(p.rt, args[1], &duration)
	if err != nil {
		common.Throw(p.rt, fmt.Errorf("invalid duration argument: %w", err))
	}

	opts := disruptors.HTTPDisruptionOptions{}
	if len(args) > 2 {
		err = convertValue(p.rt, args[2], &opts)
		if err != nil {
			common.Throw(p.rt, fmt.Errorf("invalid options argument: %w", err))
		}
	}

	err = p.ProtocolFaultInjector.InjectHTTPFaults(p.ctx, fault, duration, opts)
	if err != nil {
		common.Throw(p.rt, fmt.Errorf("error injecting fault: %w", err))
	}
}

// InjectGrpcFaults is a proxy method. Validates parameters and delegates to the PodDisruptor method
func (p *jsProtocolFaultInjector) InjectGrpcFaults(args ...goja.Value) {
	if len(args) < 2 {
		common.Throw(p.rt, fmt.Errorf("GrpcFault and duration are required"))
	}

	fault := disruptors.GrpcFault{}
	err := convertValue(p.rt, args[0], &fault)
	if err != nil {
		common.Throw(p.rt, fmt.Errorf("invalid fault argument: %w", err))
	}

	var duration time.Duration
	err = convertValue(p.rt, args[1], &duration)
	if err != nil {
		common.Throw(p.rt, fmt.Errorf("invalid duration argument: %w", err))
	}

	opts := disruptors.GrpcDisruptionOptions{}
	if len(args) > 2 {
		err = convertValue(p.rt, args[2], &opts)
		if err != nil {
			common.Throw(p.rt, fmt.Errorf("invalid options argument: %w", err))
		}
	}

	err = p.ProtocolFaultInjector.InjectGrpcFaults(p.ctx, fault, duration, opts)
	if err != nil {
		common.Throw(p.rt, fmt.Errorf("error injecting fault: %w", err))
	}
}

type jsPodDisruptor struct {
	jsDisruptor
	jsProtocolFaultInjector
}

// buildJsPodDisruptor builds a goja object that implements the PodDisruptor API
func buildJsPodDisruptor(
	ctx context.Context,
	rt *goja.Runtime,
	disruptor disruptors.PodDisruptor,
) (*goja.Object, error) {
	d := &jsPodDisruptor{
		jsDisruptor: jsDisruptor{
			ctx:       ctx,
			rt:        rt,
			Disruptor: disruptor,
		},
		jsProtocolFaultInjector: jsProtocolFaultInjector{
			ctx:                   ctx,
			rt:                    rt,
			ProtocolFaultInjector: disruptor,
		},
	}

	return buildObject(rt, d)
}

type jsServiceDisruptor struct {
	jsDisruptor
	jsProtocolFaultInjector
}

// buildJsServiceDisruptor builds a goja object that implements the ServiceDisruptor API
func buildJsServiceDisruptor(
	ctx context.Context,
	rt *goja.Runtime,
	disruptor disruptors.ServiceDisruptor,
) (*goja.Object, error) {
	d := &jsServiceDisruptor{
		jsDisruptor: jsDisruptor{
			ctx:       ctx,
			rt:        rt,
			Disruptor: disruptor,
		},
		jsProtocolFaultInjector: jsProtocolFaultInjector{
			ctx:                   ctx,
			rt:                    rt,
			ProtocolFaultInjector: disruptor,
		},
	}

	return buildObject(rt, d)
}

// NewPodDisruptor creates an instance of a PodDisruptor
// The context passed to this constructor is expected to control the lifecycle of the PodDisruptor
func NewPodDisruptor(
	ctx context.Context,
	rt *goja.Runtime,
	c goja.ConstructorCall,
	k8s kubernetes.Kubernetes,
) (*goja.Object, error) {
	if c.Argument(0).Equals(goja.Null()) {
		return nil, fmt.Errorf("PodDisruptor constructor expects a non null PodSelector argument")
	}

	selector := disruptors.PodSelector{}
	err := convertValue(rt, c.Argument(0), &selector)
	if err != nil {
		return nil, fmt.Errorf("invalid PodSelector: %w", err)
	}

	options := disruptors.PodDisruptorOptions{}
	// options argument is optional
	if len(c.Arguments) > 1 {
		err = convertValue(rt, c.Argument(1), &options)
		if err != nil {
			return nil, fmt.Errorf("invalid PodDisruptorOptions: %w", err)
		}
	}

	disruptor, err := disruptors.NewPodDisruptor(ctx, k8s, selector, options)
	if err != nil {
		return nil, fmt.Errorf("error creating PodDisruptor: %w", err)
	}

	obj, err := buildJsPodDisruptor(ctx, rt, disruptor)
	if err != nil {
		return nil, fmt.Errorf("error creating PodDisruptor: %w", err)
	}

	return obj, nil
}

// NewServiceDisruptor creates an instance of a ServiceDisruptor and returns it as a goja object
// The context passed to this constructor is expected to control the lifecycle of the ServiceDisruptor
func NewServiceDisruptor(
	ctx context.Context,
	rt *goja.Runtime,
	c goja.ConstructorCall,
	k8s kubernetes.Kubernetes,
) (*goja.Object, error) {
	if len(c.Arguments) < 2 {
		return nil, fmt.Errorf("ServiceDisruptor constructor requires service and namespace parameters")
	}

	var service string
	err := convertValue(rt, c.Argument(0), &service)
	if err != nil {
		return nil, fmt.Errorf("invalid service name argument for ServiceDisruptor constructor: %w", err)
	}

	var namespace string
	err = convertValue(rt, c.Argument(1), &namespace)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace argument for ServiceDisruptor constructor: %w", err)
	}

	options := disruptors.ServiceDisruptorOptions{}
	// options argument is optional
	if len(c.Arguments) > 2 {
		err = convertValue(rt, c.Argument(2), &options)
		if err != nil {
			return nil, fmt.Errorf("invalid ServiceDisruptorOptions: %w", err)
		}
	}

	disruptor, err := disruptors.NewServiceDisruptor(ctx, k8s, service, namespace, options)
	if err != nil {
		return nil, fmt.Errorf("error creating ServiceDisruptor: %w", err)
	}

	obj, err := buildJsServiceDisruptor(ctx, rt, disruptor)
	if err != nil {
		return nil, fmt.Errorf("error creating ServiceDisruptor: %w", err)
	}

	return obj, nil
}
