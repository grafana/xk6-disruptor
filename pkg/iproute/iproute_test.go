package iproute_test

import (
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/grafana/xk6-disruptor/pkg/iproute"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

func Test_IPRoute(t *testing.T) {
	t.Parallel()

	for _, tc := range []struct {
		name        string
		run         func(ip iproute.IPRoute) error
		expectedCmd []string
	}{
		{
			name: "Adds an IP address",
			run: func(ip iproute.IPRoute) error {
				return ip.Add("10.0.0.1/24", "eth0")
			},
			expectedCmd: []string{"ip addr add 10.0.0.1/24 dev eth0"},
		},
		{
			name: "Removes an IP address",
			run: func(ip iproute.IPRoute) error {
				return ip.Delete("10.0.0.1/24", "eth0")
			},
			expectedCmd: []string{"ip addr del 10.0.0.1/24 dev eth0"},
		},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			fakeExec := runtime.NewFakeExecutor(nil, nil)
			ip := iproute.New(fakeExec)
			err := tc.run(ip)
			if err != nil {
				t.Fatalf("returned error: %v", err)
			}

			if diff := cmp.Diff(tc.expectedCmd, fakeExec.CmdHistory()); diff != "" {
				t.Errorf("commands ran do not match expectations:\n%s", diff)
			}
		})
	}
}

func Test_IPRoutePropagatesErrors(t *testing.T) {
	t.Parallel()

	expectedErr := errors.New("propagated errors")

	fakeExec := runtime.NewFakeExecutor([]byte("propagated error"), expectedErr)
	ip := iproute.New(fakeExec)
	err := ip.Add("something", "something")
	if !errors.Is(err, expectedErr) {
		t.Fatalf("returned error %q, expected %q", err, expectedErr)
	}
}
