package disruptors

import (
	"context"
	"sort"
	"testing"

	corev1 "k8s.io/api/core/v1"

	runtime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"
)

// TODO: Refactor tests so they include the generated command.
// Currently we do not have tests covering command generation logic for ServiceDisruptor.
func Test_NewServiceDisruptor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		name        string
		namespace   string
		service     *corev1.Service
		pods        []corev1.Pod
		options     ServiceDisruptorOptions
		expectError bool
		expected    []string
	}{
		{
			title:     "one endpoint",
			name:      "test-svc",
			namespace: "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelectorLabel("app", "test").
				WithPort("http", 80, intstr.FromInt(80)).
				BuildAsPtr(),
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-1").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					WithIP("192.0.2.6").
					Build(),
			},
			options: ServiceDisruptorOptions{
				InjectTimeout: -1,
			},
			expectError: false,
			expected:    []string{"pod-1"},
		},
		{
			title:     "multiple endpoints",
			name:      "test-svc",
			namespace: "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelectorLabel("app", "test").
				WithPort("http", 80, intstr.FromInt(80)).
				BuildAsPtr(),
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-1").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					WithIP("192.0.2.6").
					Build(),
				builders.NewPodBuilder("pod-2").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					WithIP("192.0.2.7").
					Build(),
			},
			options: ServiceDisruptorOptions{
				InjectTimeout: -1,
			},
			expectError: false,
			expected:    []string{"pod-1", "pod-2"},
		},
		{
			title:     "no endpoints",
			name:      "test-svc",
			namespace: "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelectorLabel("app", "test").
				WithPort("http", 80, intstr.FromInt(80)).
				BuildAsPtr(),
			pods:        []corev1.Pod{},
			options:     ServiceDisruptorOptions{},
			expectError: false,
			expected:    []string{},
		},
		{
			title:       "service does not exist",
			name:        "test-svc",
			namespace:   "test-ns",
			service:     nil,
			pods:        []corev1.Pod{},
			options:     ServiceDisruptorOptions{},
			expectError: true,
		},
		{
			title:     "empty service name",
			name:      "",
			namespace: "test-ns",
			service: builders.NewServiceBuilder("test-svc").
				WithNamespace("test-ns").
				WithSelectorLabel("app", "test").
				WithPort("http", 80, intstr.FromInt(80)).
				BuildAsPtr(),
			pods:        []corev1.Pod{},
			options:     ServiceDisruptorOptions{},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			objs := []runtime.Object{}
			if tc.service != nil {
				objs = append(objs, tc.service)
			}
			for p := range tc.pods {
				objs = append(objs, &tc.pods[p])
			}

			client := fake.NewSimpleClientset(objs...)
			k, _ := kubernetes.NewFakeKubernetes(client)

			d, err := NewServiceDisruptor(
				context.TODO(),
				k,
				tc.name,
				tc.namespace,
				tc.options,
			)

			if tc.expectError && err != nil {
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

			targets, _ := d.Targets(context.TODO())
			sort.Strings(targets)
			if diff := cmp.Diff(targets, tc.expected); diff != "" {
				t.Errorf("expected targets dot not match returned\n%s", diff)
				return
			}
		})
	}
}
