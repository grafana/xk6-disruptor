package tcp_test

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"testing"
	"time"

	"github.com/grafana/xk6-disruptor/pkg/agent/protocol/tcp"
)

const localv4 = "127.0.0.1:0"

// Test_Proxy_Forwards tests the tcp.Proxy using tcp.ForwardHandler, ensuring messages are forwarded to and from the
// proxy.
func Test_Proxy_Forwards(t *testing.T) {
	t.Parallel()

	upstreamL, err := net.Listen("tcp", localv4)
	if err != nil {
		t.Fatalf("creating upstream listener: %v", err)
	}

	serverCh := make(chan string)
	serverErr := make(chan error)
	go func() {
		serverErr <- echoServer(upstreamL, serverCh)
	}()

	proxyL, err := net.Listen("tcp", localv4)
	if err != nil {
		t.Fatalf("creating proxy listener: %v", err)
	}

	proxy := tcp.NewProxy(proxyL, upstreamL.Addr(), tcp.ForwardHandler{})
	go func() {
		err := proxy.Start()
		if err != nil {
			// t.Fatal cannot be used inside a goroutine.
			t.Errorf("couldn't start poxy: %v", err)
		}
	}()

	proxyConn, err := net.Dial("tcp", proxyL.Addr().String())
	if err != nil {
		t.Fatalf("dialing proxy address: %v", err)
	}

	bufReader := bufio.NewReader(proxyConn)

	// Write a first line.
	_, err = fmt.Fprintln(proxyConn, "a line")
	if err != nil {
		t.Fatalf("writing to proxy conn: %v", err)
	}

	// Check the server received the line
	select {
	case <-time.After(time.Second):
		t.Fatalf("upstream did not receive the line before the deadline")
	case serverLine := <-serverCh:
		if serverLine != "a line\n" {
			t.Fatalf("upstream received unexpected data %q", serverLine)
		}
	}

	// Check we received the echoed data
	clientLine, err := bufReader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading upstream response from proxyconn: %v", err)
	}
	if clientLine != "a line\n" {
		t.Fatalf("downstream received unexpected data %q", clientLine)
	}

	// Write a second line.
	_, err = fmt.Fprintln(proxyConn, "another line")
	if err != nil {
		t.Fatalf("writing to proxy conn: %v", err)
	}

	// Check the server received the line
	select {
	case <-time.After(time.Second):
		t.Fatalf("upstream did not receive the line before the deadline")
	case serverLine := <-serverCh:
		if serverLine != "another line\n" {
			t.Fatalf("upstream received unexpected data %q", serverLine)
		}
	}

	// Check we received the echoed data
	clientLine, err = bufReader.ReadString('\n')
	if err != nil {
		t.Fatalf("reading upstream response from proxyconn: %v", err)
	}
	if clientLine != "another line\n" {
		t.Fatalf("downstream received unexpected data %q", clientLine)
	}

	// Close the connection to the proxy.
	_ = proxyConn.Close()

	select {
	case <-time.After(time.Second):
		t.Fatalf("upstream connection was not closed")
	case line, ok := <-serverCh:
		if ok {
			t.Fatalf("upstream receive unexpected data: %q", line)
		}
	}

	select {
	case <-time.After(time.Second):
		t.Fatalf("server did not terminate")
	case err = <-serverErr:
		if err != nil {
			t.Fatalf("server returned an error: %v", err)
		}
	}
}

// Test_Proxy_Forwards tests the tcp.Proxy using tcp.RejectHandler, ensuring both client and server connections are
// closed properly and cleanly when handlers return errors.
func Test_Proxy_Rejects(t *testing.T) {
	t.Parallel()

	upstreamL, err := net.Listen("tcp", localv4)
	if err != nil {
		t.Fatalf("creating upstream listener: %v", err)
	}

	serverCh := make(chan string)
	serverErr := make(chan error)
	go func() {
		serverErr <- echoServer(upstreamL, serverCh)
	}()

	proxyL, err := net.Listen("tcp", localv4)
	if err != nil {
		t.Fatalf("creating proxy listener: %v", err)
	}

	proxy := tcp.NewProxy(proxyL, upstreamL.Addr(), tcp.RejectHandler{})
	go func() {
		err := proxy.Start()
		if err != nil {
			// t.Fatal cannot be used inside a goroutine.
			t.Errorf("couldn't start poxy: %v", err)
		}
	}()

	proxyConn, err := net.Dial("tcp", proxyL.Addr().String())
	if err != nil {
		t.Fatalf("dialing proxy address: %v", err)
	}

	// Attempt to write a first line.
	_, err = fmt.Fprintln(proxyConn, "a line")
	if err != nil {
		t.Fatalf("error writing data: %v", err)
	}

	singleByte := make([]byte, 1)
	_, err = proxyConn.Read(singleByte)
	if err == nil {
		t.Fatalf("expected connection to be closed by rejectHandler: %v", err)
	}

	select {
	case <-time.After(time.Second):
		t.Fatalf("upstream connection was not closed")
	case line, ok := <-serverCh:
		if ok {
			t.Fatalf("upstream receive unexpected data: %q", line)
		}
	}

	select {
	case <-time.After(time.Second):
		t.Fatalf("server did not terminate")
	case err = <-serverErr:
		if err != nil {
			t.Fatalf("server returned an error: %v", err)
		}
	}
}

// echoServer is a helper function for testing that accepts a single connection from the given listener, and pushes
// each received line to lineCh. When the connection is closed, it also closes lineCh.
func echoServer(l net.Listener, lineCh chan string) error {
	defer close(lineCh)

	conn, err := l.Accept()
	if err != nil {
		return fmt.Errorf("accepting conn: %w", err)
	}

	reader := bufio.NewReader(conn)
	for {
		line, err := reader.ReadString('\n')
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("reading from conn: %w", err)
		}

		_, err = conn.Write([]byte(line))
		if err != nil {
			return fmt.Errorf("echoing back to conn: %w", err)
		}

		select {
		case lineCh <- line:
			continue
		case <-time.After(time.Second):
			return fmt.Errorf("reader did not consume line %q", line)
		}
	}
}
