//go:build integration
// +build integration

package network_test

import (
	"context"
	"fmt"
	"math/rand"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/docker/docker/api/types/container"
	"github.com/grafana/xk6-disruptor/pkg/testutils/echotester"

	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const echoServerPort = "6666"

func networkDisruption(duration time.Duration) []string {
	return []string{
		"xk6-disruptor-agent", "network-drop", "-d", fmt.Sprint(duration),
	}
}

func Test_DropsNetworkTraffic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	echoserver, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			ExposedPorts: []string{echoServerPort},
			FromDockerfile: testcontainers.FromDockerfile{
				Dockerfile: "Dockerfile",
				Context:    filepath.Join("..", "..", "..", "testcontainers", "echoserver"),
			},
			WaitingFor: wait.ForExposedPort(),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("creating echoserver container: %v", err)
	}

	t.Cleanup(func() { require.NoError(t, echoserver.Terminate(ctx)) })

	port, err := echoserver.MappedPort(ctx, nat.Port(echoServerPort))
	if err != nil {
		t.Fatalf("getting echoserver mapped port: %v", err)
	}

	agentSidecar, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:       "ghcr.io/grafana/xk6-disruptor-agent:latest",
			Entrypoint:  networkDisruption(time.Hour),
			Privileged:  true,
			WaitingFor:  wait.ForExec([]string{"pgrep", "xk6-disruptor-agent"}),
			NetworkMode: container.NetworkMode("container:" + echoserver.GetContainerID()),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("creating agent container: %v", err)
	}

	t.Cleanup(func() { require.NoError(t, agentSidecar.Terminate(ctx)) })

	time.Sleep(1 * time.Second)

	errors := make(chan error)

	const nTests = 100
	for range nTests {
		go func() {
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
			echoTester, err := echotester.NewTester(net.JoinHostPort("localhost", port.Port()))
			if err != nil {
				errors <- err
				return
			}

			errors <- echoTester.Echo()
		}()
	}

	nErrors := 0
	for range nTests {
		if err := <-errors; err != nil {
			nErrors++
		}
	}

	if nErrors != nTests {
		t.Errorf("got %d errors, expected %d", nErrors, nTests)
	}

	t.Logf("Got %d errors", nErrors)
}

func Test_StopsDroppingNetworkTraffic(t *testing.T) {
	t.Parallel()

	ctx := context.Background()

	echoserver, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			ExposedPorts: []string{echoServerPort},
			FromDockerfile: testcontainers.FromDockerfile{
				Dockerfile: "Dockerfile",
				Context:    filepath.Join("..", "..", "..", "testcontainers", "echoserver"),
			},
			WaitingFor: wait.ForExposedPort(),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("creating echoserver container: %v", err)
	}

	t.Cleanup(func() { require.NoError(t, echoserver.Terminate(ctx)) })

	port, err := echoserver.MappedPort(ctx, nat.Port(echoServerPort))
	if err != nil {
		t.Fatalf("getting echoserver mapped port: %v", err)
	}

	agentSidecar, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:       "ghcr.io/grafana/xk6-disruptor-agent:latest",
			Entrypoint:  networkDisruption(3 * time.Second),
			Privileged:  true,
			WaitingFor:  wait.ForExec([]string{"pgrep", "xk6-disruptor-agent"}),
			NetworkMode: container.NetworkMode("container:" + echoserver.GetContainerID()),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("creating agent container: %v", err)
	}

	t.Cleanup(func() { require.NoError(t, agentSidecar.Terminate(ctx)) })

	// Wait until the disruption has ended.
	time.Sleep(5 * time.Second)

	errors := make(chan error)

	const nTests = 5
	for range nTests {
		go func() {
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
			echoTester, err := echotester.NewTester(net.JoinHostPort("localhost", port.Port()))
			if err != nil {
				errors <- err
				return
			}

			errors <- echoTester.Echo()
		}()
	}

	for range nTests {
		if err := <-errors; err != nil {
			t.Errorf("Error connecting to echoserver: %v", err)
		}
	}
}
