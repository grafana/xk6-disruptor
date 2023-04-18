// Package api implements a layer between javascript code (via goja)) and the disruptors
// allowing for validations and type conversions when needed
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

// converts a goja.Value into a object. target is expected to be a pointer
func convertValue(rt *goja.Runtime, value goja.Value, target interface{}) error {
	// get the value pointed to by the target and check for compatibility
	err := IsCompatible(value.Export(), reflect.ValueOf(target).Elem().Interface())
	if err != nil {
		return err
	}

	err = rt.ExportTo(value, target)
	return err
}

// converts a goja Value to a duration
func convertDuration(rt *goja.Runtime, value goja.Value, duration *time.Duration) error {
	durationString := ""

	err := IsCompatible(value, durationString)
	if err != nil {
		return err
	}

	err = rt.ExportTo(value, &durationString)
	if err != nil {
		return err
	}

	*duration, err = time.ParseDuration(durationString)
	return err
}

// JsPodDisruptor implements the JS interface for PodDisruptor
type JsPodDisruptor struct {
	rt        *goja.Runtime
	disruptor disruptors.PodDisruptor
}

// Targets is a proxy method. Validates parameters and delegates to the PodDisruptor method
func (p *JsPodDisruptor) Targets() goja.Value {
	targets, err := p.disruptor.Targets()
	if err != nil {
		common.Throw(p.rt, fmt.Errorf("error getting kubernetes config path: %w", err))
	}

	return p.rt.ToValue(targets)
}

// InjectHTTPFaults is a proxy method. Validates parameters and delegates to the PodDisruptor method
func (p *JsPodDisruptor) InjectHTTPFaults(args ...goja.Value) {
	if len(args) < 2 {
		common.Throw(p.rt, fmt.Errorf("HTTPFault and duration are required"))
	}

	fault := disruptors.HTTPFault{}
	err := convertValue(p.rt, args[0], &fault)
	if err != nil {
		common.Throw(p.rt, fmt.Errorf("invalid fault argument: %w", err))
	}

	var duration time.Duration
	err = convertDuration(p.rt, args[1], &duration)
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

	err = p.disruptor.InjectHTTPFaults(fault, duration, opts)
	if err != nil {
		common.Throw(p.rt, fmt.Errorf("error injecting fault: %w", err))
	}
}

// InjectGrpcFaults is a proxy method. Validates parameters and delegates to the PodDisruptor method
func (p *JsPodDisruptor) InjectGrpcFaults(args ...goja.Value) {
	if len(args) < 2 {
		common.Throw(p.rt, fmt.Errorf("GrpcFault and duration are required"))
	}

	fault := disruptors.GrpcFault{}
	err := convertValue(p.rt, args[0], &fault)
	if err != nil {
		common.Throw(p.rt, fmt.Errorf("invalid fault argument: %w", err))
	}

	var duration time.Duration
	err = convertDuration(p.rt, args[1], &duration)
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

	err = p.disruptor.InjectGrpcFaults(fault, duration, opts)
	if err != nil {
		common.Throw(p.rt, fmt.Errorf("error injecting fault: %w", err))
	}
}

func buildJsPodDisruptor(rt *goja.Runtime, disruptor disruptors.PodDisruptor) (*goja.Object, error) {
	jsDisruptor := JsPodDisruptor{
		rt:        rt,
		disruptor: disruptor,
	}

	obj := rt.NewObject()
	err := obj.Set("targets", jsDisruptor.Targets)
	if err != nil {
		return nil, err
	}

	err = obj.Set("injectHTTPFaults", jsDisruptor.InjectHTTPFaults)
	if err != nil {
		return nil, err
	}

	err = obj.Set("injectGrpcFaults", jsDisruptor.InjectGrpcFaults)
	if err != nil {
		return nil, err
	}

	return obj, nil
}

// NewPodDisruptor creates an instance of a PodDisruptor
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

	obj, err := buildJsPodDisruptor(rt, disruptor)
	if err != nil {
		return nil, fmt.Errorf("error creating PodDisruptor: %w", err)
	}

	return obj, nil
}

// NewServiceDisruptor creates an instance of a ServiceDisruptor
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

	// ServiceDisruptor is a wrapper to PodDisruptor, so we can use it for building a JsPodDisruptor.
	// Notice that when [1] is implemented, this will make even more sense because there will be only
	// a Disruptor interface.
	// [1] https://github.com/grafana/xk6-disruptor/issues/60
	obj, err := buildJsPodDisruptor(rt, disruptor)
	if err != nil {
		return nil, fmt.Errorf("error creating ServiceDisruptor: %w", err)
	}

	return obj, nil
}
