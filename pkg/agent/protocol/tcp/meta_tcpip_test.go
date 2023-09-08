package tcp

import (
	"errors"
	"io"
	"net"
	"testing"
	"time"
)

// Test_TCPConnShortReads is a meta-test that proves that the TCP/IP stack behaves as we expect it to, where Read()
// returns immediately for a TCP segment.
func Test_TCPConnShortReads(t *testing.T) {
	server, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening server: %v", err)
	}

	client, err := net.Dial("tcp", server.Addr().String())
	if err != nil {
		t.Fatalf("dialing client: %v", err)
	}

	go func() {
		twoBytes := make([]byte, 2)
		_, cErr := client.Write(twoBytes)
		if cErr != nil {
			t.Errorf("writing twoBytes: %v", err)
		}

		time.Sleep(200 * time.Millisecond)

		_, cErr = client.Write(twoBytes)
		if cErr != nil {
			t.Errorf("writing twoBytes: %v", err)
		}

		_ = client.Close()
	}()

	conn, err := server.Accept()
	if err != nil {
		t.Fatalf("accepting server conn: %v", err)
	}

	buf := make([]byte, 4)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("reading from server conn: %v", err)
	}

	if n != 2 {
		t.Fatalf("expected to read 2 bytes")
	}

	n, err = conn.Read(buf)
	if err != nil {
		t.Fatalf("reading from server conn: %v", err)
	}

	if n != 2 {
		t.Fatalf("expected to read 2 bytes")
	}

	_, err = conn.Read(buf)
	if !errors.Is(err, io.EOF) {
		t.Fatalf("expected EOF: %v", err)
	}
}
