//go:build integration
// +build integration

package agent

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/docker/docker/api/types/container"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func injectHTTPError(rate float64, code int, upstream string) []string {
	return []string{
		"xk6-disruptor-agent",
		"http",
		"--duration",
		"300s",
		"--rate",
		fmt.Sprintf("%.2f", rate),
		"--error",
		fmt.Sprintf("%d", code),
		"--port",
		"8080",
		"--target",
		"80",
		"--upstream-host",
		upstream,
	}
}

func Test_HTTPFaultInjection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		test    string
		rate    float64
		code    int
		request int
		expect  int
	}{
		{
			test:    "inject 418 error",
			rate:    1.0,
			code:    418,
			request: 200,
			expect:  418,
		},
		{
			test:    "inject no error",
			rate:    0.0,
			code:    0,
			request: 200,
			expect:  200,
		},
		{
			test:    "handle upstream error",
			rate:    0.0,
			code:    0,
			request: 500,
			expect:  500,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			gcr := testcontainers.GenericContainerRequest{
				ProviderType: testcontainers.ProviderDocker,
				ContainerRequest: testcontainers.ContainerRequest{
					Image: "kennethreitz/httpbin",
					ExposedPorts: []string{
						"80",
					},
					WaitingFor: wait.ForExposedPort(),
				},
				Started: true,
			}
			httpbin, err := testcontainers.GenericContainer(ctx, gcr)
			if err != nil {
				t.Fatalf("failed to create httpbin container %v", err)
			}

			t.Cleanup(func() {
				_ = httpbin.Terminate(ctx)
			})

			httpbinIP, err := httpbin.ContainerIP(ctx)
			if err != nil {
				t.Fatalf("failed to get httpbin IP:\n%v", err)
			}

			// make the agent run using the same stack than the httpbin container
			httpbinNetwork := container.NetworkMode("container:" + httpbin.GetContainerID())
			gcr = testcontainers.GenericContainerRequest{
				ProviderType: testcontainers.ProviderDocker,
				ContainerRequest: testcontainers.ContainerRequest{
					Image:       "ghcr.io/grafana/xk6-disruptor-agent",
					NetworkMode: httpbinNetwork,
					Cmd:         injectHTTPError(tc.rate, tc.code, httpbinIP),
					Privileged:  true,
					// TODO: find a better way for checking the agent is ready
					WaitingFor: wait.ForExec([]string{"pgrep", "xk6-disruptor-agent"}),
				},
				Started: true,
			}

			agent, err := testcontainers.GenericContainer(ctx, gcr)
			if err != nil {
				t.Fatalf("failed to create agent container %v", err)
			}

			t.Cleanup(func() {
				_ = agent.Terminate(ctx)
			})

			httpPort, err := httpbin.MappedPort(ctx, "80")
			if err != nil {
				t.Fatalf("failed to get httpbin port:\n%v", err)
			}

			// access httpbin
			url := fmt.Sprintf("http://localhost:%s/status/%d", httpPort.Port(), tc.request)

			request, err := http.NewRequest("GET", url, nil)
			if err != nil {
				t.Fatalf("failed to create request %v", err)
			}

			resp, err := http.DefaultClient.Do(request)
			if err != nil {
				t.Fatalf("failed request to %q: %v", url, err)
			}

			defer func() {
				_ = resp.Body.Close()
			}()

			if resp.StatusCode != tc.expect {
				t.Fatalf("expected status code %d but %d received", tc.expect, resp.StatusCode)
			}
		})
	}
}
