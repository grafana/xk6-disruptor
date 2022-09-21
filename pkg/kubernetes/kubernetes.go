// Package kubernetes implements helper functions for manipulating resources in a
// Kubernetes cluster.
package kubernetes

import (
	"context"

	"github.com/grafana/xk6-disruptor/pkg/kubernetes/helpers"

	apimeta "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/yaml"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
)

// Defines an interface that extends kubernetes interface[k8s.io/client-go/kubernetes.Interface] adding
// generic functions that operate on any kind of object
type Kubernetes interface {
	kubernetes.Interface
	Context() context.Context
	Create(manifest string) error
	Get(kind string, name string, namespace string, obj runtime.Object) error
	Delete(kind string, name string, namespace string) error
	Helpers() helpers.Helpers
	NamespacedHelpers(namespace string) helpers.Helpers
}

// KubernetesConfig defines the configuration for creating a Kubernetes instance
type KubernetesConfig struct {
	// Context for executing kubernetes operations
	Context context.Context
	// Path to Kubernetes access configuration
	Kubeconfig string
}

// k8s Holds the reference to the helpers for interacting with kubernetes
type k8s struct {
	config *rest.Config
	kubernetes.Interface
	ctx        context.Context
	dynamic    dynamic.Interface
	serializer runtime.Serializer
	mapper     apimeta.RESTMapper
}

// getRestMapper returns a mapper that allows mapping object types to api resources
func getRestMapper(client kubernetes.Interface, config *rest.Config) (apimeta.RESTMapper, error) {
	gr, err := restmapper.GetAPIGroupResources(client.Discovery())
	if err != nil {
		return nil, err
	}

	mapper := restmapper.NewDiscoveryRESTMapper(gr)
	return mapper, nil
}

// NewFromKubeconfig returns a Kubernetes instance configured with the kubeconfig pointed by the given path
func NewFromKubeconfig(kubeconfig string) (Kubernetes, error) {
	return NewFromConfig(KubernetesConfig{
		Kubeconfig: kubeconfig,
	})
}

// NewFromConfig returns a Kubernetes instance
func NewFromConfig(c KubernetesConfig) (Kubernetes, error) {
	config, err := clientcmd.BuildConfigFromFlags("", c.Kubeconfig)
	if err != nil {
		return nil, err
	}

	dynamic, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	mapper, err := getRestMapper(client, config)
	if err != nil {
		return nil, err
	}

	ctx := c.Context
	if ctx == nil {
		ctx = context.TODO()
	}

	return &k8s{
		config:     config,
		Interface:  client,
		ctx:        ctx,
		dynamic:    dynamic,
		serializer: yaml.NewDecodingSerializer(unstructured.UnstructuredJSONScheme),
		mapper:     mapper,
	}, nil
}

// Returns the context for executing k8s actions
func (k *k8s)Context() context.Context {
	return k.ctx
}

// Create creates a resource in a kubernetes cluster from a yaml manifest
func (k *k8s) Create(manifest string) error {
	uObj := &unstructured.Unstructured{}
	_, gvk, err := k.serializer.Decode([]byte(manifest), nil, uObj)
	if err != nil {
		return err
	}

	namespace := uObj.GetNamespace()
	if namespace == "" {
		namespace = "default"
	}
	mapping, err := k.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return err
	}

	_, err = k.dynamic.Resource(mapping.Resource).
		Namespace(namespace).
		Create(
			k.ctx,
			uObj,
			metav1.CreateOptions{},
		)

	return err
}

// Get returns an object given its kind, name and namespace. The object is returned in the runtime
// object passed as parameter.
// Example:
//    pod := corev1.Pod{}
//    err := k8s.Get("Pod", "podname", "namespace", &pod)
func (k *k8s) Get(kind string, name string, namespace string, obj runtime.Object) error {

	gvk := schema.GroupKind{Kind: kind}

	mapping, err := k.mapper.RESTMapping(gvk)
	if err != nil {
		return err
	}

	resp, err := k.dynamic.
		Resource(mapping.Resource).
		Namespace(namespace).
		Get(k.ctx, name, metav1.GetOptions{})

	if err != nil {
		return err
	}

	//convert the unstructured object to a runtime object
	uObj := resp.UnstructuredContent()
	err = runtime.DefaultUnstructuredConverter.FromUnstructured(uObj, obj)

	return err
}

// Delete deletes an object given its kind, name and namespace
func (k *k8s) Delete(kind string, name string, namespace string) error {

	gvk := schema.GroupKind{Kind: kind}

	mapping, err := k.mapper.RESTMapping(gvk)
	if err != nil {
		return err
	}

	err = k.dynamic.
		Resource(mapping.Resource).
		Namespace(namespace).
		Delete(k.ctx, name, metav1.DeleteOptions{})

	return err
}

// Helpers returns Helpers for the default namespace
func (k *k8s) Helpers() helpers.Helpers {
	return helpers.NewHelper(
		k.Interface,
		k.config,
		k.ctx,
		"default",
	)
}

// NamespacedHelpers returns helpers for the given namespace
func (k *k8s) NamespacedHelpers(namespace string) helpers.Helpers {
	return helpers.NewHelper(
		k.Interface,
		k.config,
		k.ctx,
		namespace,
	)
}
