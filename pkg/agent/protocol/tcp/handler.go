package tcp

import (
	"errors"
	"hash/crc64"
	"io"
	"net"
	"time"
)

// HandlerBuilder is a function that returns a Handler. Proxy will use this function to create a new handler for each
// TCP connection. ConnMeta provides information about the TCP connection this handler will handle.
// Handler implementations that require additional parameters can have a builder that returns a HandlerBuilder, such as
type HandlerBuilder func(ConnMeta) Handler

// Handler is an object capable of acting when TCP messages are either sent or received.
type Handler interface {
	// HandleUpward forwards data from the client to the server. Proxy will call each method exactly once for the
	// single connection a Handler instance handles. Implementations should consume from client and write to server
	// until an error occurs.
	// When either HandleUpward or HandleDownward return an error, connections to both server and clients are closed.
	// If ErrTerminate is returned, the connection is still closed but no error message is logged.
	HandleUpward(client io.Reader, server io.Writer) error
	// HandleDownward provides is the equivalent of HandleUpward for data sent from the server to the client.
	HandleDownward(server io.Reader, client io.Writer) error
}

// ErrTerminate may be returned by Handler implementations that wish to willingly terminate a connection. Connection
// will be closed, but no error log will be generated.
var ErrTerminate = errors.New("connection terminated by proxy handler")

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

// ForwardHandler is a handler that forwards data between client and server without taking any actions.
type ForwardHandler struct{}

// ForwardHandlerBuilder returns a new instance of a ForwardHandler.
func ForwardHandlerBuilder(_ ConnMeta) Handler {
	return ForwardHandler{}
}

func (ForwardHandler) HandleUpward(client io.Reader, server io.Writer) error {
	_, err := io.Copy(server, client)
	return err
}

func (ForwardHandler) HandleDownward(server io.Reader, client io.Writer) error {
	_, err := io.Copy(client, server)
	return err
}

// RejectHandler is a handler that closes connections immediately after being opened.
type RejectHandler struct{}

// RejectHandlerBuilder returns a new instance of a ForwardHandler.
func RejectHandlerBuilder(_ ConnMeta) Handler {
	return RejectHandler{}
}

func (RejectHandler) HandleUpward(client io.Reader, server io.Writer) error {
	return ErrTerminate
}

func (RejectHandler) HandleDownward(server io.Reader, client io.Writer) error {
	return ErrTerminate
}
