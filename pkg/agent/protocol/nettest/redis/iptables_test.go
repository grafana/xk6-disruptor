package redis_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const iptablesRule = "INPUT -p tcp --dport 6379 -j REJECT --reject-with tcp-reset"

func Test_Redis(t *testing.T) {
	t.Parallel()

	if os.Getenv("NETTEST") == "" {
		t.Skip("Skipping network protocol test as NETTEST is not set")
	}

	ctx := context.TODO()

	redis, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			Networks:     []string{},
			Image:        "redis",
			ExposedPorts: []string{"6379/tcp"},
			WaitingFor:   wait.ForExposedPort(),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("failed to create redis container %v", err)
	}

	t.Cleanup(func() {
		_ = redis.Terminate(ctx)
	})

	iptables, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Dockerfile: "Dockerfile",
				Context:    filepath.Join("..", "containers", "iptables"),
			},
			NetworkMode: container.NetworkMode("container:" + redis.GetContainerID()),
			Cmd:         []string{"/bin/sh", "-c", "echo ready && sleep infinity"},
			Privileged:  true,
			WaitingFor:  wait.ForLog("ready"),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("failed to create agent container %v", err)
	}

	t.Cleanup(func() {
		_ = iptables.Terminate(ctx)
	})

	redisGo, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Dockerfile: "Dockerfile",
				Context:    filepath.Join("..", "containers", "redis-go"),
			},
			Cmd:         []string{"localhost:6379"},
			NetworkMode: container.NetworkMode("container:" + redis.GetContainerID()),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("failed to create agent container %v", err)
	}

	t.Cleanup(func() {
		_ = redisGo.Terminate(ctx)
	})

	// TODO:Follow container under test logs. Currently doing so hangs out the test forever.

	redisGoStatus, err := redisGo.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !redisGoStatus.Running {
		t.Fatalf("Redis client container failed")
	}

	//nolint:errcheck,gosec // Error checking elided for brevity. TODO: Wrap this in a helper function.
	iptables.Exec(context.TODO(), []string{"/bin/sh", "-c", "iptables -I " + iptablesRule})

	time.Sleep(2 * time.Second)

	//nolint:errcheck,gosec // Error checking elided for brevity. TODO: Wrap this in a helper function.
	iptables.Exec(context.TODO(), []string{"/bin/sh", "-c", "iptables -D " + iptablesRule})

	redisGoStatus, err = redisGo.State(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !redisGoStatus.Running {
		t.Fatalf("Redis client container failed")
	}
}
