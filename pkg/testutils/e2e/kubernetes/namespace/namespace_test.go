package namespace

import (
	"context"
	"testing"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_CreateNamespace(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		options     []TestNamespaceOption
		expectError bool
		check       func(kubernetes.Interface, string) error
	}{
		{
			title:       "default options",
			options:     []TestNamespaceOption{},
			expectError: false,
			check: func(k8s kubernetes.Interface, ns string) error {
				return nil
			},
		},
		{
			title:       "random namespace",
			options:     []TestNamespaceOption{WithPrefix("prefix-")},
			expectError: false,
			check: func(k8s kubernetes.Interface, ns string) error {
				return nil
			},
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			client := fake.NewSimpleClientset()
			ns, err := CreateTestNamespace(context.TODO(), t, client, tc.options...)
			if err != nil && !tc.expectError {
				t.Errorf("unexpected error %v", err)
				return
			}

			if err == nil && tc.expectError {
				t.Errorf("should had failed")
				return
			}

			if err != nil && tc.expectError {
				return
			}

			err = tc.check(client, ns)
			if err != nil {
				t.Errorf("%v", err)
				return
			}
		})
	}
}
