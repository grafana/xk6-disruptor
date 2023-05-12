package helpers

import (
	"bytes"
	"context"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/testutils/assertions"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
	corev1 "k8s.io/api/core/v1"
	networking "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"
)

func buildEndpointsWithNotReadyAddresses() *corev1.Endpoints {
	return &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "EndPoints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{
			{
				NotReadyAddresses: []corev1.EndpointAddress{
					{
						IP: "1.1.1.1",
					},
				},
			},
		},
	}
}

func Test_WaitServiceReady(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		test        string
		delay       time.Duration
		endpoints   *corev1.Endpoints
		updated     *corev1.Endpoints
		expectError bool
		timeout     time.Duration
	}

	testCases := []TestCase{
		{
			test:        "endpoint not created",
			endpoints:   nil,
			updated:     nil,
			delay:       time.Second * 0,
			expectError: true,
			timeout:     time.Second * 5,
		},
		{
			test: "endpoint ready",
			endpoints: builders.NewEndPointsBuilder("service").
				WithSubset(
					[]corev1.EndpointPort{},
					[]string{"pod1"},
				).
				Build(),
			updated:     nil,
			delay:       time.Second * 0,
			expectError: false,
			timeout:     time.Second * 5,
		},
		{
			test:      "wait for endpoint to be ready",
			endpoints: builders.NewEndPointsBuilder("service").Build(),
			updated: builders.NewEndPointsBuilder("service").
				WithSubset(
					[]corev1.EndpointPort{},
					[]string{"pod1"},
				).
				Build(),
			delay:       time.Second * 2,
			expectError: false,
			timeout:     time.Second * 5,
		},
		{
			test:        "not ready addresses",
			endpoints:   builders.NewEndPointsBuilder("service").Build(),
			updated:     buildEndpointsWithNotReadyAddresses(),
			delay:       time.Second * 2,
			expectError: true,
			timeout:     time.Second * 5,
		},
		{
			test:      "timeout waiting for addresses",
			endpoints: builders.NewEndPointsBuilder("service").Build(),
			updated: builders.NewEndPointsBuilder("service").
				WithSubset(
					[]corev1.EndpointPort{},
					[]string{"pod1"},
				).
				Build(),
			delay:       time.Second * 10,
			expectError: true,
			timeout:     time.Second * 5,
		},
		{
			test: "other endpoint ready",
			endpoints: builders.NewEndPointsBuilder("another-service").
				WithSubset(
					[]corev1.EndpointPort{},
					[]string{"pod1"},
				).
				Build(),
			updated:     nil,
			delay:       time.Second * 10,
			expectError: true,
			timeout:     time.Second * 5,
		},
	}
	for _, tc := range testCases {
		tc := tc

		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			client := fake.NewSimpleClientset()
			if tc.endpoints != nil {
				_, err := client.CoreV1().Endpoints("default").Create(context.TODO(), tc.endpoints, metav1.CreateOptions{})
				if err != nil {
					t.Errorf("error updating endpoint: %v", err)
				}
			}

			go func(tc TestCase) {
				if tc.updated == nil {
					return
				}
				time.Sleep(tc.delay)
				_, err := client.CoreV1().Endpoints("default").Update(context.TODO(), tc.updated, metav1.UpdateOptions{})
				if err != nil {
					t.Errorf("error updating endpoint: %v", err)
				}
			}(tc)

			h := NewServiceHelper(client, nil, "default")

			err := h.WaitServiceReady(context.TODO(), "service", tc.timeout)
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tc.expectError && err == nil {
				t.Error("expected an error but none returned")
				return
			}
			// error expected and returned, it is ok
			if tc.expectError && err != nil {
				return
			}
		})
	}
}

func Test_WaitIngressReady(t *testing.T) {
	t.Parallel()

	type TestCase struct {
		test        string
		delay       time.Duration
		ingress     *networking.Ingress
		expectError bool
		timeout     time.Duration
	}

	testCases := []TestCase{
		{
			test:        "ingress not created",
			ingress:     nil,
			delay:       time.Second * 0,
			expectError: true,
			timeout:     time.Second * 5,
		},
		{
			test: "ingress ready",
			ingress: builders.NewIngressBuilder("ingress", intstr.FromInt(80)).
				WithAddress("loadbalancer").
				Build(),
			delay:       time.Second * 0,
			expectError: false,
			timeout:     time.Second * 5,
		},
		{
			test:        "wait for ingress to be ready",
			ingress:     builders.NewIngressBuilder("ingress", intstr.FromInt(80)).Build(),
			delay:       time.Second * 2,
			expectError: false,
			timeout:     time.Second * 5,
		},
		{
			test:        "timeout waiting",
			ingress:     builders.NewIngressBuilder("ingress", intstr.FromInt(80)).Build(),
			delay:       time.Second * 10,
			expectError: true,
			timeout:     time.Second * 5,
		},
	}
	for _, tc := range testCases {
		tc := tc

		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()
			client := fake.NewSimpleClientset()

			if tc.ingress != nil {
				_, err := client.NetworkingV1().
					Ingresses("default").
					Create(context.TODO(), tc.ingress, metav1.CreateOptions{})
				if err != nil {
					t.Errorf("error updating ingress: %v", err)
				}
			}

			// update ingress with address
			go func(tc TestCase) {
				if tc.ingress == nil {
					return
				}
				time.Sleep(tc.delay)
				updated := tc.ingress.DeepCopy()
				updated.Status.LoadBalancer.Ingress = []networking.IngressLoadBalancerIngress{
					{
						Hostname: "loadbalancer",
					},
				}
				_, err := client.NetworkingV1().
					Ingresses("default").
					Update(context.TODO(), updated, metav1.UpdateOptions{})
				if err != nil {
					t.Errorf("error updating ingress: %v", err)
				}
			}(tc)

			h := NewServiceHelper(client, nil, "default")

			err := h.WaitIngressReady(context.TODO(), "ingress", tc.timeout)
			if !tc.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if tc.expectError && err == nil {
				t.Error("expected an error but none returned")
				return
			}
			// error expected and returned, it is ok
			if tc.expectError && err != nil {
				return
			}
		})
	}
}

func Test_ServiceClient(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		name        string
		host        string
		service     string
		port        int
		namespace   string
		method      string
		path        string
		reqBody     []byte
		headers     map[string]string
		respBody    []byte
		respStatus  int
		expectError bool
		expectURL   string
	}{
		{
			name:        "simple get request",
			host:        "http://localhost:8001",
			service:     "my-service",
			port:        80,
			namespace:   "default",
			method:      "GET",
			path:        "/path/to/request",
			reqBody:     []byte{},
			headers:     map[string]string{},
			respBody:    []byte{},
			respStatus:  200,
			expectError: false,
			expectURL:   "http://localhost:8001/api/v1/namespaces/default/services/my-service:80/proxy/path/to/request",
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			testRequest, err := http.NewRequest(tc.method, tc.path, bytes.NewReader(tc.reqBody))
			if err != nil {
				t.Errorf("failed creating test request %v", err)
				return
			}

			fakeClient := newFakeHTTPClient(tc.respStatus, tc.respBody)
			serviceClient := newServiceProxy(
				fakeClient,
				tc.host,
				tc.namespace,
				tc.service,
				tc.port,
			)

			_, err = serviceClient.Do(testRequest)

			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should have failed")
				return
			}

			if tc.expectError && err != nil {
				return
			}

			if fakeClient.Request.URL.String() != tc.expectURL {
				t.Errorf("invalid request url. Expected: %s received: %s", tc.expectURL, fakeClient.Request.URL.String())
				return
			}

			if fakeClient.Request.Method != tc.method {
				t.Errorf("invalid request method. Expected: %s received: %s", tc.method, fakeClient.Request.Method)
				return
			}
		})
	}
}

func Test_ServicePortMapping(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		serviceName string
		namespace   string
		service     *corev1.Service
		endpoints   *corev1.Endpoints
		port        uint
		expectError bool
		targets     map[string]uint
	}{
		{
			title:       "invalid Port option",
			serviceName: "test-svc",
			namespace:   "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelector(map[string]string{
					"app": "test",
				}).
				WithPorts(
					[]corev1.ServicePort{
						{
							Name:       "http",
							Port:       8080,
							TargetPort: intstr.FromInt(8080),
						},
					},
				).Build(),
			endpoints: builders.NewEndPointsBuilder("test-svc").
				WithNamespace("test-ns").
				WithSubset(
					[]corev1.EndpointPort{
						{
							Name: "http",
							Port: 80,
						},
					},
					[]string{"pod-1"},
				).Build(),
			port:        80,
			targets:     map[string]uint{},
			expectError: true,
		},
		{
			title:       "Numeric target port specified",
			serviceName: "test-svc",
			namespace:   "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelector(map[string]string{
					"app": "test",
				}).
				WithPorts(
					[]corev1.ServicePort{
						{
							Name:       "http",
							Port:       8080,
							TargetPort: intstr.FromInt(80),
						},
					},
				).Build(),
			endpoints: builders.NewEndPointsBuilder("test-svc").
				WithNamespace("test-ns").
				WithSubset(
					[]corev1.EndpointPort{
						{
							Name: "http",
							Port: 80,
						},
					},
					[]string{"pod-1"},
				).Build(),
			port:        8080,
			expectError: false,
			targets: map[string]uint{
				"pod-1": 80,
			},
		},
		{
			title:       "named target port",
			serviceName: "test-svc",
			namespace:   "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelector(map[string]string{
					"app": "test",
				}).
				WithPorts(
					[]corev1.ServicePort{
						{
							Name:       "http",
							Port:       8080,
							TargetPort: intstr.FromString("http"),
						},
					},
				).Build(),
			endpoints: builders.NewEndPointsBuilder("test-svc").
				WithNamespace("test-ns").
				WithSubset(
					[]corev1.EndpointPort{
						{
							Name: "http",
							Port: 80,
						},
					},
					[]string{"pod-1"},
				).Build(),
			port:        8080,
			expectError: false,
			targets: map[string]uint{
				"pod-1": 80,
			},
		},
		{
			title:       "Multiple target ports",
			serviceName: "test-svc",
			namespace:   "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelector(map[string]string{
					"app": "test",
				}).
				WithPorts(
					[]corev1.ServicePort{
						{
							Name:       "http",
							Port:       80,
							TargetPort: intstr.FromString("http"),
						},
					},
				).Build(),
			endpoints: builders.NewEndPointsBuilder("test-svc").
				WithNamespace("test-ns").
				WithSubset(
					[]corev1.EndpointPort{
						{
							Name: "http",
							Port: 80,
						},
					},
					[]string{"pod-1"},
				).
				WithSubset(
					[]corev1.EndpointPort{
						{
							Name: "http",
							Port: 8080,
						},
					},
					[]string{"pod-2"},
				).
				Build(),
			port:        80,
			expectError: false,
			targets: map[string]uint{
				"pod-1": 80,
				"pod-2": 8080,
			},
		},
		{
			title:       "Default port mapping",
			serviceName: "test-svc",
			namespace:   "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelector(map[string]string{
					"app": "test",
				}).
				WithPorts(
					[]corev1.ServicePort{
						{
							Name:       "http",
							Port:       8080,
							TargetPort: intstr.FromInt(80),
						},
					},
				).Build(),
			endpoints: builders.NewEndPointsBuilder("test-svc").
				WithNamespace("test-ns").
				WithSubset(
					[]corev1.EndpointPort{
						{
							Name: "http",
							Port: 80,
						},
					},
					[]string{"pod-1"},
				).Build(),
			port: 0,
			targets: map[string]uint{
				"pod-1": 80,
			},
			expectError: false,
		},
		{
			title:       "No target for mapping",
			serviceName: "test-svc",
			namespace:   "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelector(map[string]string{
					"app": "test",
				}).
				WithPorts(
					[]corev1.ServicePort{
						{
							Port:       8080,
							TargetPort: intstr.FromInt(80),
						},
					},
				).Build(),
			endpoints: builders.NewEndPointsBuilder("test-svc").
				WithNamespace("test-ns").
				WithSubset(
					[]corev1.EndpointPort{
						{
							Port: 8080,
						},
					},
					[]string{"pod-1"},
				).Build(),
			port:        8080,
			expectError: false,
			targets:     map[string]uint{},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			client := fake.NewSimpleClientset()
			_, err := client.CoreV1().Services(tc.namespace).Create(context.TODO(), tc.service, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("error creating service: %v", err)
			}
			_, err = client.CoreV1().Endpoints(tc.namespace).Create(context.TODO(), tc.endpoints, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("error creating endpoint: %v", err)
			}

			helper := NewServiceHelper(client, nil, tc.namespace)

			targets, err := helper.MapPort(context.TODO(), tc.serviceName, tc.port)
			if !tc.expectError && err != nil {
				t.Errorf(" failed: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed")
				return
			}

			if tc.expectError && err != nil {
				return
			}

			if !reflect.DeepEqual(tc.targets, targets) {
				t.Errorf("expected %v got %v", tc.targets, targets)
				return
			}
		})
	}
}

func Test_Targets(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title        string
		serviceName  string
		namespace    string
		service      *corev1.Service
		pods         []*corev1.Pod
		expectError  bool
		expectedPods []string
	}{
		{
			title:       "one endpoint",
			serviceName: "test-svc",
			namespace:   "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelector(map[string]string{
					"app": "test",
				}).
				WithPorts(
					[]corev1.ServicePort{
						{
							Name:       "http",
							Port:       8080,
							TargetPort: intstr.FromInt(80),
						},
					},
				).Build(),
			pods: []*corev1.Pod{
				builders.NewPodBuilder("pod-1").
					WithNamespace("test-ns").
					WithLabels(
						map[string]string{
							"app": "test",
						},
					).Build(),
			},
			expectError:  false,
			expectedPods: []string{"pod-1"},
		},
		{
			title:       "no targets",
			serviceName: "test-svc",
			namespace:   "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelector(map[string]string{
					"app": "test",
				}).
				WithPorts(
					[]corev1.ServicePort{
						{
							Name:       "http",
							Port:       8080,
							TargetPort: intstr.FromInt(80),
						},
					},
				).Build(),
			pods:         []*corev1.Pod{},
			expectError:  false,
			expectedPods: []string{},
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			client := fake.NewSimpleClientset()
			_, err := client.CoreV1().Services(tc.service.Namespace).Create(context.TODO(), tc.service, metav1.CreateOptions{})
			if err != nil {
				t.Errorf("error creating service: %v", err)
			}

			for _, pod := range tc.pods {
				_, err = client.CoreV1().Pods(tc.namespace).Create(context.TODO(), pod, metav1.CreateOptions{})
				if err != nil {
					t.Errorf("error creating endpoint: %v", err)
				}
			}

			helper := NewServiceHelper(client, nil, tc.namespace)
			targets, err := helper.GetTargets(context.TODO(), tc.serviceName)
			if !tc.expectError && err != nil {
				t.Errorf("failed: %v", err)
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf(" unexpected error creating service disruptor: %v", err)
				return
			}

			if tc.expectError && err == nil {
				t.Errorf("should had failed creating service disruptor")
				return
			}

			if tc.expectError && err != nil {
				return
			}

			if !assertions.CompareStringArrays(tc.expectedPods, targets) {
				t.Errorf("result does not match expected value. Expected: %s\nActual: %s\n", tc.expectedPods, targets)
				return
			}
		})
	}
}
