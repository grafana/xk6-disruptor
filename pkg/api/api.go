// Package api implements a layer between javascript code (via goja)) and the disruptors
// allowing for validations and type conversions when needed
package api

import (
	"fmt"
	"reflect"

	"github.com/dop251/goja"
	"github.com/grafana/xk6-disruptor/pkg/disruptors"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
)

// NewPodDisruptor creates an instance of a PodDisruptor
func NewPodDisruptor(rt *goja.Runtime, c goja.ConstructorCall, k8s kubernetes.Kubernetes) (*goja.Object, error) {
	if c.Argument(0).Equals(goja.Null()) {
		return nil, fmt.Errorf("PodDisruptor constructor expects a non null PodSelector argument")
	}

	selector := disruptors.PodSelector{}
	err := IsCompatible(c.Argument(0).Export(), selector)
	if err != nil {
		return nil, fmt.Errorf("invalid PodSelector: %w", err)
	}
	err = rt.ExportTo(c.Argument(0), &selector)
	if err != nil {
		return nil, fmt.Errorf("invalid PodSelector: %w", err)
	}

	options := disruptors.PodDisruptorOptions{}
	// options argument is optional
	if len(c.Arguments) > 1 {
		err = IsCompatible(c.Argument(1).Export(), options)
		if err != nil {
			return nil, fmt.Errorf("invalid PodDisruptorOptions: %w", err)
		}
		err = rt.ExportTo(c.Argument(1), &options)
		if err != nil {
			return nil, fmt.Errorf("invalid PodDisruptorOptions: %w", err)
		}
	}

	disruptor, err := disruptors.NewPodDisruptor(k8s, selector, options)
	if err != nil {
		return nil, fmt.Errorf("error creating PodDisruptor: %w", err)
	}

	return rt.ToValue(disruptor).ToObject(rt), nil
}

// NewServiceDisruptor creates an instance of a ServiceDisruptor
func NewServiceDisruptor(rt *goja.Runtime, c goja.ConstructorCall, k8s kubernetes.Kubernetes) (*goja.Object, error) {
	if len(c.Arguments) < 2 {
		return nil, fmt.Errorf("ServiceDisruptor constructor requires service and namespace parameters")
	}

	if c.Argument(0).ExportType().Kind() != reflect.String {
		return nil, fmt.Errorf("ServiceDisruptor constructor expects service name to be a string")
	}
	var service string
	err := rt.ExportTo(c.Argument(0), &service)
	if err != nil {
		return nil, fmt.Errorf("invalid service name argument for ServiceDisruptor constructor: %w", err)
	}

	if c.Argument(1).ExportType().Kind() != reflect.String {
		return nil, fmt.Errorf("ServiceDisruptor constructor expects namespace to be a string")
	}
	var namespace string
	err = rt.ExportTo(c.Argument(1), &namespace)
	if err != nil {
		return nil, fmt.Errorf("invalid namespace argument for ServiceDisruptor constructor: %w", err)
	}

	options := disruptors.ServiceDisruptorOptions{}
	// options argument is optional
	if len(c.Arguments) > 2 {
		err = rt.ExportTo(c.Argument(2), &options)
		if err != nil {
			return nil, fmt.Errorf("invalid ServiceDisruptorOptions: %w", err)
		}
	}

	disruptor, err := disruptors.NewServiceDisruptor(k8s, service, namespace, options)
	if err != nil {
		return nil, fmt.Errorf("error creating ServiceDisruptor: %w", err)
	}

	return rt.ToValue(disruptor).ToObject(rt), nil
}
