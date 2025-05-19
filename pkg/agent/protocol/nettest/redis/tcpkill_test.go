package redis_test

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol/nettest/util"
)

func Test_Redis_TCPKill(t *testing.T) {
	t.Parallel()

	// if os.Getenv("NETTEST") == "" {
	// 	t.Skip("Skipping network protocol test as NETTEST is not set")
	// }

	ctx := context.TODO()

	redis, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
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

	// TODO: Calling terminate with a log attached makes the test hang.
	// See: https://github.com/testcontainers/testcontainers-go/issues/1669
	// t.Cleanup(func() {
	//  	_ = redisGo.Terminate(ctx)
	// })

	redisGo.FollowOutput(util.Mirror{T: t, Name: "redis-go"})
	err = redisGo.StartLogProducer(ctx)
	if err != nil {
		t.Fatal(err)
	}
	// TODO: See above.
	// t.Cleanup(func() {
	// 	redisGo.StopLogProducer()
	// })

	redisGoStatus, err := redisGo.State(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !redisGoStatus.Running {
		t.Fatalf("Redis client container failed")
	}

	time.Sleep(3 * time.Second)

	tcpkill, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Dockerfile: "Dockerfile",
				Context:    filepath.Join("..", "containers", "tcpkill"),
			},
			NetworkMode: container.NetworkMode("container:" + redis.GetContainerID()),
			Cmd:         []string{"/bin/sh", "-c", "tcpkill -i any -5 port 6379"},
			Privileged:  true,
			WaitingFor:  wait.ForLog("tcpkill: listening"),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("failed to create tcpkill container %v", err)
	}

	t.Cleanup(func() {
		_ = tcpkill.Terminate(ctx)
	})

	tcpkill.FollowOutput(util.Mirror{T: t, Name: "tcpkill"})
	err = tcpkill.StartLogProducer(ctx)
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() {
		tcpkill.StopLogProducer()
	})

	time.Sleep(7 * time.Second)

	redisGoStatus, err = redisGo.State(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !redisGoStatus.Running {
		t.Fatalf("Redis client container failed")
	}
}
