//go:build integration
// +build integration

package agent

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/grafana/xk6-disruptor/pkg/testutils/grpc/dynamic"
	testcontainers "github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
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

func injectGrpcFault(rate float64, status int, upstream string) []string {
	return []string{
		"xk6-disruptor-agent",
		"grpc",
		"--duration",
		"300s",
		"--rate",
		fmt.Sprintf("%.2f", rate),
		"--status",
		fmt.Sprintf("%d", status),
		"--message",
		"Internal error",
		"--port",
		"4000",
		"--target",
		"9000",
		"-x",
		// exclude reflection service otherwise the dynamic client will not work
		"grpc.reflection.v1alpha.ServerReflection,grpc.reflection.v1.ServerReflection",
		"--upstream-host",
		upstream,
	}
}

func checkHTTPRequest(url string, expected int) error {
	request, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request %w", err)
	}

	resp, err := http.DefaultClient.Do(request)
	if err != nil {
		return fmt.Errorf("failed request to %q: %v", url, err)
	}

	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != expected {
		return fmt.Errorf("expected status code %d but %d received", expected, resp.StatusCode)
	}

	return nil
}

func checkGrpcRequest(host string, service string, method string, request []byte, expected int32) error {
	client, err := dynamic.NewClientWithDialOptions(
		host,
		service,
		grpc.WithInsecure(),
	)
	if err != nil {
		return fmt.Errorf("creating client for service %s: %w", service, err)
	}

	err = client.Connect(context.TODO())
	if err != nil {
		return fmt.Errorf("connecting to service %s: %w", service, err)
	}

	input := [][]byte{}
	input = append(input, request)

	_, err = client.Invoke(context.TODO(), method, input)
	// got an error but it is not due to the grpc status
	s, ok := status.FromError(err)
	if !ok {
		return fmt.Errorf("unexpected error: %w", err)
	}

	if int32(s.Code()) != expected {
		return fmt.Errorf("expected status code %d got %d", expected, int32(s.Code()))
	}

	return nil
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
					WaitingFor:  wait.ForExec([]string{"pgrep", "xk6-disruptor-agent"}),
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

			err = checkHTTPRequest(url, tc.expect)
			if err != nil {
				t.Fatalf("failed %v", err)
			}
		})
	}
}

func Test_GrpcFaultInjection(t *testing.T) {
	t.Parallel()

	testCases := []struct {
		test   string
		rate   float64
		status int
		expect int32
	}{
		{
			test:   "inject internal error",
			rate:   1.0,
			status: 14,
			expect: 14,
		},
		{
			test:   "inject no error",
			rate:   0.0,
			status: 14,
			expect: 0,
		},
	}

	for _, tc := range testCases {
		tc := tc
		t.Run(tc.test, func(t *testing.T) {
			t.Parallel()

			ctx := context.Background()

			exposed, _ := nat.NewPort("tcp", "9000")
			gcr := testcontainers.GenericContainerRequest{
				ProviderType: testcontainers.ProviderDocker,
				ContainerRequest: testcontainers.ContainerRequest{
					Image: "moul/grpcbin",
					ExposedPorts: []string{
						"9000",
					},
					WaitingFor: wait.ForListeningPort(exposed),
				},
				Started: true,
			}
			grpcbin, err := testcontainers.GenericContainer(ctx, gcr)
			if err != nil {
				t.Fatalf("failed to create grpcbin container: %v", err)
			}

			t.Cleanup(func() {
				_ = grpcbin.Terminate(ctx)
			})

			grpcbinIP, err := grpcbin.ContainerIP(ctx)
			if err != nil {
				t.Fatalf("failed to get grpcbin IP:\n%v", err)
			}

			// make the agent run using the same stack than the grpcbin container
			grpcbinNetwork := container.NetworkMode("container:" + grpcbin.GetContainerID())
			gcr = testcontainers.GenericContainerRequest{
				ProviderType: testcontainers.ProviderDocker,
				ContainerRequest: testcontainers.ContainerRequest{
					Image:       "ghcr.io/grafana/xk6-disruptor-agent",
					NetworkMode: grpcbinNetwork,
					Cmd:         injectGrpcFault(tc.rate, tc.status, grpcbinIP),
					Privileged:  true,
					WaitingFor:  wait.ForExec([]string{"pgrep", "xk6-disruptor-agent"}),
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

			grpcPort, err := grpcbin.MappedPort(ctx, "9000")
			if err != nil {
				t.Fatalf("failed to get httpbin port:\n%v", err)
			}

			// access grpcbin
			url := fmt.Sprintf("localhost:%s", grpcPort.Port())

			err = checkGrpcRequest(url, "grpcbin.GRPCBin", "Empty", []byte("{}"), tc.expect)
			if err != nil {
				t.Fatalf("failed %v", err)
			}
		})
	}
}
