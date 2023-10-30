//go:build integration
// +build integration

package tcpconn_test

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"net"
	"path/filepath"
	"testing"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/go-connections/nat"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

const echoServerPort = "6666"

func tcpDisruption(port string, duration time.Duration, rate float64) []string {
	return []string{
		"xk6-disruptor-agent", "tcp-drop", "-d", fmt.Sprint(duration), "--port", port, "--rate", fmt.Sprint(rate),
	}
}

const disruptWithRate = "xk6-disruptor-agent tcp-drop -d 1h --port 6666 --rate %f"

// Test_DropsConnectionsAccordingToRate tests that a number of connections to a target server fail according to rate.
func Test_DropsConnectionsAccordingToRate(t *testing.T) {
	t.Parallel()

	const rate = 0.5

	ctx := context.TODO()

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

	t.Cleanup(func() {
		echoserver.Terminate(ctx)
	})

	port, err := echoserver.MappedPort(ctx, nat.Port(echoServerPort))
	if err != nil {
		t.Fatalf("getting echoserver mapped port: %v", err)
	}

	agentSidecar, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:       "ghcr.io/grafana/xk6-disruptor-agent",
			Entrypoint:  tcpDisruption(echoServerPort, time.Hour, rate),
			Privileged:  true,
			WaitingFor:  wait.ForExec([]string{"pgrep", "xk6-disruptor-agent"}),
			NetworkMode: container.NetworkMode("container:" + echoserver.GetContainerID()),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("creating agent container: %v", err)
	}

	t.Cleanup(func() {
		agentSidecar.Terminate(ctx)
	})

	// TODO: Find a better way to wait for disruption to start.
	time.Sleep(time.Second)

	errors := make(chan error)

	const nTests = 500
	for i := 0; i < nTests; i++ {
		go func() {
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
			echoTester, err := newTester(net.JoinHostPort("localhost", port.Port()))
			if err != nil {
				errors <- err
				return
			}

			errors <- echoTester.echo()
		}()
	}

	nErrs := 0.0
	for i := 0; i < nTests; i++ {
		if err := <-errors; err != nil {
			nErrs++
		}
	}

	// We expect nTests * rate errors, but we will accept +-15%.
	min := nTests * rate * 0.85
	max := nTests * rate * 1.15

	if nErrs < min || nErrs > max {
		t.Fatalf("got %f errors, expected %f<%f<%f", nErrs, min, nTests*rate, max)
	}

	t.Logf("Got %f errors", nErrs)
}

// Test_StopsDroppingConnections tests that after the disruption ends, no connections are terminated.
func Test_StopsDroppingConnections(t *testing.T) {
	t.Parallel()

	const rate = 1

	ctx := context.TODO()

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

	t.Cleanup(func() {
		echoserver.Terminate(ctx)
	})

	port, err := echoserver.MappedPort(ctx, nat.Port(echoServerPort))
	if err != nil {
		t.Fatalf("getting echoserver mapped port: %v", err)
	}

	agentSidecar, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:       "ghcr.io/grafana/xk6-disruptor-agent",
			Entrypoint:  tcpDisruption(echoServerPort, 3*time.Second, rate),
			Privileged:  true,
			WaitingFor:  wait.ForExec([]string{"pgrep", "xk6-disruptor-agent"}),
			NetworkMode: container.NetworkMode("container:" + echoserver.GetContainerID()),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("creating agent container: %v", err)
	}

	t.Cleanup(func() {
		agentSidecar.Terminate(ctx)
	})

	// Wait until the disruption has ended.
	time.Sleep(5 * time.Second)

	errors := make(chan error)

	const nTests = 5
	for i := 0; i < nTests; i++ {
		go func() {
			time.Sleep(time.Duration(rand.Intn(500)) * time.Millisecond)
			echoTester, err := newTester(net.JoinHostPort("localhost", port.Port()))
			if err != nil {
				errors <- err
				return
			}

			errors <- echoTester.echo()
		}()
	}

	for i := 0; i < nTests; i++ {
		if err := <-errors; err != nil {
			t.Errorf("Error connecting to echoserver: %v", err)
		}
	}
}

func Test_DropsExistingConnections(t *testing.T) {
	t.Parallel()

	const rate = 1

	ctx := context.TODO()

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

	t.Cleanup(func() {
		echoserver.Terminate(ctx)
	})

	port, err := echoserver.MappedPort(ctx, nat.Port(echoServerPort))
	if err != nil {
		t.Fatalf("getting echoserver mapped port: %v", err)
	}

	echoTester, err := newTester(net.JoinHostPort("localhost", port.Port()))
	if err != nil {
		t.Fatalf("connecting to echo server before disruption: %v", err)
	}

	err = echoTester.echo()
	if err != nil {
		t.Fatalf("performing echo test before disruption: %v", err)
	}

	agentSidecar, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ProviderType: testcontainers.ProviderDocker,
		ContainerRequest: testcontainers.ContainerRequest{
			Image:       "ghcr.io/grafana/xk6-disruptor-agent",
			Entrypoint:  tcpDisruption(echoServerPort, 3*time.Second, rate),
			Privileged:  true,
			WaitingFor:  wait.ForExec([]string{"pgrep", "xk6-disruptor-agent"}),
			NetworkMode: container.NetworkMode("container:" + echoserver.GetContainerID()),
		},
		Started: true,
	})
	if err != nil {
		t.Fatalf("creating agent container: %v", err)
	}

	t.Cleanup(func() {
		agentSidecar.Terminate(ctx)
	})

	// TODO: Find a better way to ensure the disruption is in place.
	time.Sleep(1 * time.Second)

	err = echoTester.echo()
	if err == nil {
		t.Fatalf("Connection was still alive after disruption kicked in")
	}
}

// tester is a struct that can be used to test an echoserver is behaving as expected. Each tester uses and keeps
// a connection to an echoserver.
type tester struct {
	conn   net.Conn
	reader *bufio.Reader
}

// newTester opens a connection to the specified address and returns a tester that uses it.
func newTester(address string) (*tester, error) {
	t := &tester{}

	var err error
	t.conn, err = net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("connecting to %s: %w", address, err)
	}

	t.reader = bufio.NewReader(t.conn)

	return t, nil
}

// echo sends a message to the echoserver and checks the same message is received back.
func (t *tester) echo() error {
	const line = "I am a test!\n"

	_, err := t.conn.Write([]byte(line))
	if err != nil {
		return fmt.Errorf("writing string: %w", err)
	}

	echoed, err := t.reader.ReadString('\n')
	if err != nil {
		return fmt.Errorf("reading back: %w", err)
	}

	if echoed != line {
		return fmt.Errorf("echoed string %q does not match sent %q", echoed, line)
	}

	return nil
}

// close closes the connection to the echoserver and tests the receiving end of the connection gets closed as well.
func (t *tester) close() error {
	err := t.conn.Close()
	if err != nil {
		return fmt.Errorf("closing connection: %w", err)
	}

	str, err := t.reader.ReadString('\n')
	if !errors.Is(err, io.EOF) {
		return fmt.Errorf("expected EOF after closing, got %v and read %q instead", err, str)
	}

	return nil
}
