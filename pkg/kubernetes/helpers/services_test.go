package helpers

import (
	"bytes"
	"context"
	"net/http"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

func buildEndpointsWithoutAddresses() *corev1.Endpoints {
	return &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "EndPoints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "service",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{},
	}
}

func buildEndpointsWithAddresses() *corev1.Endpoints {
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
				Addresses: []corev1.EndpointAddress{
					{
						IP: "1.1.1.1",
					},
				},
			},
		},
	}
}

func buildOtherEndpointsWithAddresses() *corev1.Endpoints {
	return &corev1.Endpoints{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1",
			Kind:       "EndPoints",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "otherservice",
			Namespace: "default",
		},
		Subsets: []corev1.EndpointSubset{
			{
				Addresses: []corev1.EndpointAddress{
					{
						IP: "1.1.1.1",
					},
				},
			},
		},
	}
}

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
			test:        "endpoint ready",
			endpoints:   buildEndpointsWithAddresses(),
			updated:     nil,
			delay:       time.Second * 0,
			expectError: false,
			timeout:     time.Second * 5,
		},
		{
			test:        "wait for endpoint to be ready",
			endpoints:   buildEndpointsWithoutAddresses(),
			updated:     buildEndpointsWithAddresses(),
			delay:       time.Second * 2,
			expectError: false,
			timeout:     time.Second * 5,
		},
		{
			test:        "not ready addresses",
			endpoints:   buildEndpointsWithoutAddresses(),
			updated:     buildEndpointsWithNotReadyAddresses(),
			delay:       time.Second * 2,
			expectError: true,
			timeout:     time.Second * 5,
		},
		{
			test:        "timeout waiting for addresses",
			endpoints:   buildEndpointsWithoutAddresses(),
			updated:     buildEndpointsWithAddresses(),
			delay:       time.Second * 10,
			expectError: true,
			timeout:     time.Second * 5,
		},
		{
			test:        "other endpoint ready",
			endpoints:   buildOtherEndpointsWithAddresses(),
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

			h := NewHelper(client, nil, "default")

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
