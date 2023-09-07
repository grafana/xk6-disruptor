package tcp

import (
	"errors"
	"hash/crc64"
	"io"
	"net"
	"time"
)

// ConnMeta holds metadata about a TCP connection.
type ConnMeta struct {
	Opened        time.Time
	ClientAddress net.Addr
	ServerAddress net.Addr
}

// Hash returns a semi-unique number to every connection.
// The implementation of Hash is not guaranteed to be stable between updates of this package.
func (c ConnMeta) Hash() uint64 {
	// We use CRC64 as this hash does not need to be cryptographically secure, and it's easy to get an uint64 from it.
	hash := crc64.New(crc64.MakeTable(crc64.ISO))
	_, _ = hash.Write([]byte(c.Opened.String()))
	_, _ = hash.Write([]byte(c.ClientAddress.String()))
	_, _ = hash.Write([]byte(c.ServerAddress.String()))

	return hash.Sum64()
}

// Handler is an object capable of acting when TCP messages are either sent or received.
type Handler interface {
	// HandleUpward forwards data from the client to the server. Proxy will call HandleUpward once for every
	// connection, expecting it to keep consuming data until an error occurs, in which case the Proxy will close both
	// upstream and downstream connections. If ErrTerminate is returned, the connection is still closed but no error
	// message is logged.
	HandleUpward(client io.Reader, server io.Writer, meta ConnMeta) error
	// HandleDownward provides is the equivalent of HandleUpward for data sent from the server to the client.
	HandleDownward(server io.Reader, client io.Writer, meta ConnMeta) error
}

// ErrTerminate may be returned by Handler implementations that wish to willingly terminate a connection. Connection
// will be closed, but no error log will be generated.
var ErrTerminate = errors.New("connection terminated by proxy handler")

// ForwardHandler is a handler that forwards data between client and server without taking any actions.
type ForwardHandler struct{}

func (ForwardHandler) HandleUpward(client io.Reader, server io.Writer, _ ConnMeta) error {
	_, err := io.Copy(server, client)
	return err
}

func (ForwardHandler) HandleDownward(server io.Reader, client io.Writer, _ ConnMeta) error {
	_, err := io.Copy(client, server)
	return err
}

// RejectHandler is a handler that closes connections immediately after being opened.
type RejectHandler struct{}

func (RejectHandler) HandleUpward(client io.Reader, server io.Writer, _ ConnMeta) error {
	return ErrTerminate
}

func (RejectHandler) HandleDownward(server io.Reader, client io.Writer, _ ConnMeta) error {
	return ErrTerminate
}
