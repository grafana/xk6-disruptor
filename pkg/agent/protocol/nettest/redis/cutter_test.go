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

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol/nettest/util"
)

func Test_Redis_Cutter(t *testing.T) {
	t.Parallel()

	if os.Getenv("NETTEST") == "" {
		t.Skip("Skipping network protocol test as NETTEST is not set")
	}

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

	cutter, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			FromDockerfile: testcontainers.FromDockerfile{
				Dockerfile: "Dockerfile",
				Context:    filepath.Join("..", "containers", "cutter"),
			},
			NetworkMode: container.NetworkMode("container:" + redis.GetContainerID()),
			Cmd:         []string{"/bin/sh", "-c", "tcp-cutter 127.0.0.1 6379"},
			Privileged:  true,
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("failed to create cutter container %v", err)
	}

	// t.Cleanup(func() {
	// 	_ = cutter.Terminate(ctx)
	// })

	cutter.FollowOutput(util.Mirror{T: t, Name: "cutter"})
	err = cutter.StartLogProducer(ctx)
	if err != nil {
		t.Fatal(err)
	}

	//  t.Cleanup(func() {
	//  	cutter.StopLogProducer()
	//  })

	time.Sleep(2 * time.Second)

	redisGoStatus, err = redisGo.State(ctx)
	if err != nil {
		t.Fatal(err)
	}

	if !redisGoStatus.Running {
		t.Fatalf("Redis client container failed")
	}
}
