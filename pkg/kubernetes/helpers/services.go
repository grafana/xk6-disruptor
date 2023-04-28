package helpers

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// ServiceHelper implements functions for dealing with services
type ServiceHelper interface {
	// WaitServiceReady waits for the given service to have at least one endpoint available
	WaitServiceReady(ctx context.Context, service string, timeout time.Duration) error
	// GetServiceProxy returns a client for making HTTP requests to the service using api server's proxy
	GetServiceProxy(service string, port int) (ServiceClient, error)
	// MapPort return a map of pod, port pairs for a service port
	MapPort(ctx context.Context, service string, port uint) (map[string]uint, error)
	// GetTargets returns the list of pods that match the service selector criteria
	GetTargets(ctx context.Context, service string) ([]string, error)
}

// helpers struct holds the data required by the helpers
type serviceHelper struct {
	config    *rest.Config
	client    kubernetes.Interface
	namespace string
}

// NewServiceHelper returns a ServiceHelper
func NewServiceHelper(client kubernetes.Interface, config *rest.Config, namespace string) ServiceHelper {
	return &serviceHelper{
		client:    client,
		config:    config,
		namespace: namespace,
	}
}

func (h *serviceHelper) WaitServiceReady(ctx context.Context, service string, timeout time.Duration) error {
	return utils.Retry(timeout, time.Second, func() (bool, error) {
		ep, err := h.client.CoreV1().Endpoints(h.namespace).Get(ctx, service, metav1.GetOptions{})
		if err != nil {
			if errors.IsNotFound(err) {
				return false, nil
			}
			return false, fmt.Errorf("failed to access service: %w", err)
		}

		for _, subset := range ep.Subsets {
			if len(subset.Addresses) > 0 {
				return true, nil
			}
		}

		return false, nil
	})
}

// getTargetPort returns the service port that corresponds to the given port number
// if the given port is 0, it will return the default port or error if more than one port is defined
func getTargetPort(service *corev1.Service, port uint) (corev1.ServicePort, error) {
	ports := service.Spec.Ports
	if port != 0 {
		for _, p := range ports {
			if uint(p.Port) == port {
				return p, nil
			}
		}
		return corev1.ServicePort{}, fmt.Errorf("the service does not expose the given port: %d", port)
	}

	if len(ports) > 1 {
		return corev1.ServicePort{}, fmt.Errorf("service exposes multiple ports. Port option must be defined")
	}

	return ports[0], nil
}

func (h *serviceHelper) MapPort(ctx context.Context, name string, port uint) (map[string]uint, error) {
	service, err := h.client.CoreV1().Services(h.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve target service %s: %w", service, err)
	}

	targets := map[string]uint{}
	tp, err := getTargetPort(service, port)
	if err != nil {
		return nil, err
	}

	endpoints, err := h.client.CoreV1().Endpoints(h.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve endpoints for service %s: %w", service, err)
	}

	// iterate over sub-ranges looking for those that have the target port
	// and retrieve the name of the pods and the target por
	for _, subset := range endpoints.Subsets {
		for _, p := range subset.Ports {
			if (tp.TargetPort.StrVal != "" && tp.TargetPort.StrVal == p.Name) || tp.TargetPort.IntVal == p.Port {
				for _, addr := range subset.Addresses {
					if addr.TargetRef.Kind == "Pod" {
						targets[addr.TargetRef.Name] = uint(p.Port)
					}
				}
				break
			}
		}
	}

	return targets, nil
}

func (h *serviceHelper) GetTargets(ctx context.Context, name string) ([]string, error) {
	service, err := h.client.CoreV1().Services(h.namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to retrieve target service %s: %w", service, err)
	}

	listOptions := metav1.ListOptions{
		LabelSelector: labels.SelectorFromSet(service.Spec.Selector).String(),
	}
	pods, err := h.client.CoreV1().Pods(h.namespace).List(
		ctx,
		listOptions,
	)
	if err != nil {
		return nil, err
	}

	names := []string{}
	for _, p := range pods.Items {
		names = append(names, p.Name)
	}

	return names, nil
}

// ServiceClient is the minimal interface for executing HTTP requests
// Offers an interface similar to http.Client but only the Do method is supported
// It is used primarily to allow mocking the client in unit tests
type ServiceClient interface {
	// Do executes the request to the service and returns the response
	// From the request only the URL path method, headers and body are considered
	Do(request *http.Request) (*http.Response, error)
}

// ServiceProxy implements the HTTPClient interface for making HTTP request to a service
type ServiceProxy struct {
	service   string
	namespace string
	port      int
	baseURL   string
	client    ServiceClient
}

// newServiceProxy creates a ServiceProxy
func newServiceProxy(
	httpClient ServiceClient,
	host string,
	namespace string,
	service string,
	port int,
) *ServiceProxy {
	// build url to the service proxy
	baseURL := fmt.Sprintf("%s/api/v1/namespaces/%s/services/%s:%d/proxy", host, namespace, service, port)

	return &ServiceProxy{
		client:    httpClient,
		service:   service,
		namespace: namespace,
		baseURL:   baseURL,
		port:      port,
	}
}

func (h *serviceHelper) GetServiceProxy(service string, port int) (ServiceClient, error) {
	httpClient, err := rest.HTTPClientFor(h.config)
	if err != nil {
		return nil, err
	}

	return newServiceProxy(
		httpClient,
		h.config.Host,
		h.namespace,
		service,
		port,
	), nil
}

// Do implements the Do method from the ServiceClient interface
func (c *ServiceProxy) Do(request *http.Request) (*http.Response, error) {
	serviceURL := c.baseURL + request.URL.Path
	serviceRequest, err := http.NewRequest(request.Method, serviceURL, request.Body)
	if err != nil {
		return nil, err
	}

	serviceRequest.Header = request.Header

	return c.client.Do(serviceRequest)
}
