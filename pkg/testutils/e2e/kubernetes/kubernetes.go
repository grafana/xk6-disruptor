// Package kubernetes implements helper functions for managing kubernetes resources
// in e2e tests
//
// This package borrows some code from https://github.com/grafana/xk6-kubernetes
package kubernetes

import (
	"context"
	"fmt"
	"path/filepath"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/discovery"
	memory "k8s.io/client-go/discovery/cached"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
)

// Client holds the state to access kubernetes
type Client struct {
	dynamic    dynamic.Interface
	mapper     meta.RESTMapper
	serializer runtime.Serializer
}

func getClientConfig(kubeconfig string) (*rest.Config, error) {
	if kubeconfig == "" {
		home := homedir.HomeDir()
		if home == "" {
			return nil, fmt.Errorf("home directory not found")
		}
		kubeconfig = filepath.Join(home, ".kube", "config")
	}
	return clientcmd.BuildConfigFromFlags("", kubeconfig)
}

// NewFromKubeconfig returns a new Client using the kubeconfig pointed by the path provided
func NewFromKubeconfig(ctx context.Context, kubeconfig string) (*Client, error) {
	config, err := getClientConfig(kubeconfig)
	if err != nil {
		return nil, err
	}

	return NewForConfig(ctx, config)
}


// NewForConfig returns a new Client using a rest.Config
func NewForConfig(ctx context.Context, config *rest.Config) (*Client, error) {
	dynamic, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	mapper := restmapper.NewDeferredDiscoveryRESTMapper(memory.NewMemCacheClient(discoveryClient))
	if err != nil {
		return nil, err
	}
	
	return &Client{
		mapper:     mapper,
		dynamic:    dynamic,
		serializer: yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme),
	}, nil
}


// getResource maps kinds to api resources
func (c *Client) getResource(kind string, namespace string, versions ...string) (dynamic.ResourceInterface, error) {
	gk := schema.ParseGroupKind(kind)
	if c.mapper == nil {
		return nil, fmt.Errorf("RESTMapper not initialized")
	}

	mapping, err := c.mapper.RESTMapping(gk, versions...)
	if err != nil {
		return nil, err
	}

	var resource dynamic.ResourceInterface
	if mapping.Scope.Name() == meta.RESTScopeNameNamespace {
		resource = c.dynamic.Resource(mapping.Resource).Namespace(namespace)
	} else {
		resource = c.dynamic.Resource(mapping.Resource)
	}

	return resource, nil
}

// Apply creates a resource in a kubernetes cluster from a YAML manifest
func (c *Client) Apply(ctx context.Context, manifest string) error {
	uObj := &unstructured.Unstructured{}
	_, gvk, err := c.serializer.Decode([]byte(manifest), nil, uObj)
	if err != nil {
		return fmt.Errorf("failed to decode manifest: %w", err)
	}

	name := uObj.GetName()
	namespace := uObj.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}

	resource, err := c.getResource(gvk.GroupKind().String(), namespace, gvk.Version)
	if err != nil {
		return fmt.Errorf("failed to get resource: %w", err)
	}

	_, err = resource.Apply(
		ctx,
		name,
		uObj,
		metav1.ApplyOptions{
			FieldManager: "xk6-disruptor",
		},
	)
	return err
}