package disruptors

import (
	"context"
	"sort"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/kubernetes"
	"github.com/grafana/xk6-disruptor/pkg/testutils/kubernetes/builders"

	corev1 "k8s.io/api/core/v1"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

func Test_NewPodDisruptor(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		title       string
		pods        []corev1.Pod
		selector    PodSelector
		expectError bool
		expected    []string
	}{
		{
			title: "matching pods",
			pods: []corev1.Pod{
				builders.NewPodBuilder("pod-1").
					WithNamespace("test-ns").
					WithLabel("app", "test").
					WithIP("192.0.2.6").
					Build(),
			},
			selector: PodSelector{
				Namespace: "test-ns",
				Select: PodAttributes{Labels: map[string]string{
					"app": "test",
				}},
			},
			expectError: false,
			expected:    []string{"pod-1"},
		},
		{
			title: "no matching pods",
			pods:  []corev1.Pod{},
			selector: PodSelector{
				Namespace: "test-ns",
				Select: PodAttributes{Labels: map[string]string{
					"app": "test",
				}},
			},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		tc := tc

		t.Run(tc.title, func(t *testing.T) {
			t.Parallel()

			var objs []k8sruntime.Object
			for p := range tc.pods {
				objs = append(objs, &tc.pods[p])
			}

			client := fake.NewSimpleClientset(objs...)
			k, _ := kubernetes.NewFakeKubernetes(client)

			d, err := NewPodDisruptor(
				context.TODO(),
				k,
				tc.selector,
				PodDisruptorOptions{InjectTimeout: -1}, // Disable waiting for injected container to become Running.
			)

			if tc.expectError && err != nil {
				return
			}

			if !tc.expectError && err != nil {
				t.Errorf("unexpected error creating pod disruptor: %v", err)
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

func Test_PodSelectorString(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name     string
		selector PodSelector
		expected string
	}{
		{
			name:     "Empty selector",
			expected: `all pods in ns "default"`,
		},
		{
			name: "Only inclusions",
			selector: PodSelector{
				Namespace: "testns",
				Select:    PodAttributes{map[string]string{"foo": "bar"}},
			},
			expected: `pods including(foo=bar) in ns "testns"`,
		},
		{
			name: "Only exclusions",
			selector: PodSelector{
				Namespace: "testns",
				Exclude:   PodAttributes{map[string]string{"foo": "bar"}},
			},
			expected: `pods excluding(foo=bar) in ns "testns"`,
		},
		{
			name: "Both inclusions and exclusions",
			selector: PodSelector{
				Namespace: "testns",
				Select:    PodAttributes{map[string]string{"foo": "bar"}},
				Exclude:   PodAttributes{map[string]string{"boo": "baa"}},
			},
			expected: `pods including(foo=bar), excluding(boo=baa) in ns "testns"`,
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			output := tc.selector.String()
			if tc.expected != output {
				t.Errorf("expected string does not match output string:\n%s\n%s", tc.expected, output)
			}
		})
	}
}
