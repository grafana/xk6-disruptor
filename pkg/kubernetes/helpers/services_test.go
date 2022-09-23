package helpers

import (
	"context"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

var endpointsWithoutAddresses = &corev1.Endpoints{
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

var endpointsWithAddresses = &corev1.Endpoints{
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

var otherEndpointsWithAddresses = &corev1.Endpoints{
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

var endpointsWithNotReadyAddresses = &corev1.Endpoints{
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

func Test_WaitServiceReady(t *testing.T) {
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
			endpoints:   endpointsWithAddresses,
			updated:     nil,
			delay:       time.Second * 0,
			expectError: false,
			timeout:     time.Second * 5,
		},
		{
			test:        "wait for endpoint to be ready",
			endpoints:   endpointsWithoutAddresses,
			updated:     endpointsWithAddresses,
			delay:       time.Second * 2,
			expectError: false,
			timeout:     time.Second * 5,
		},
		{
			test:        "not ready addresses",
			endpoints:   endpointsWithoutAddresses,
			updated:     endpointsWithNotReadyAddresses,
			delay:       time.Second * 2,
			expectError: true,
			timeout:     time.Second * 5,
		},
		{
			test:        "timeout waiting for addresses",
			endpoints:   endpointsWithoutAddresses,
			updated:     endpointsWithAddresses,
			delay:       time.Second * 10,
			expectError: true,
			timeout:     time.Second * 5,
		},
		{
			test:        "other endpoint ready",
			endpoints:   otherEndpointsWithAddresses,
			updated:     nil,
			delay:       time.Second * 10,
			expectError: true,
			timeout:     time.Second * 5,
		},
	}
	for _, tc := range testCases {
		t.Run(tc.test, func(t *testing.T) {
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

			err := h.WaitServiceReady("service", tc.timeout)
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
