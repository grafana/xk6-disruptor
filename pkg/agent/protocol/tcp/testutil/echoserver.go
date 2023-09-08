package testutil

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"time"
)

// EchoServer is a helper function for testing that accepts a single connection from the given listener, and pushes
// each received line to lineCh. When the connection is closed, it also closes lineCh.
func EchoServer(l net.Listener, lineCh chan string) error {
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
