package tcp

import (
	"errors"
	"fmt"
	"log"
	"net"
	"time"
)

// Proxy implements a TCP transparent proxy between a client and a server.
type Proxy struct {
	l        net.Listener
	upstream net.Addr
	handler  Handler
}

func NewProxy(l net.Listener, upstream net.Addr, handler Handler) *Proxy {
	return &Proxy{
		l:        l,
		upstream: upstream,
		handler:  handler,
	}
}

func (p *Proxy) Start() error {
	for {
		conn, err := p.l.Accept()
		if err != nil {
			return err
		}

		go func() {
			err := p.handleConn(conn)
			// TODO: Better error handling
			log.Printf("handling connection: %v", err)
		}()
	}
}

func (p *Proxy) Stop() error {
	// TODO: Harvest open connections and close them.
	return nil
}

func (p *Proxy) handleConn(downstreamConn net.Conn) error {
	defer func() {
		_ = downstreamConn.Close()
	}()

	upstreamConn, err := net.Dial("tcp", p.upstream.String())
	if err != nil {
		return fmt.Errorf("opening upstream connection: %w", err)
	}

	defer func() {
		_ = upstreamConn.Close()
	}()

	metadata := ConnMeta{
		Opened:        time.Now(),
		ClientAddress: downstreamConn.RemoteAddr(),
		ServerAddress: upstreamConn.RemoteAddr(),
	}

	errChan := make(chan error, 2)
	go func() {
		errChan <- func() error {
			err := p.handler.HandleUpward(downstreamConn, upstreamConn, metadata)
			if err != nil && !errors.Is(err, ErrTerminate) {
				return err
			}

			return nil
		}()
	}()
	go func() {
		errChan <- func() error {
			err := p.handler.HandleDownward(upstreamConn, downstreamConn, metadata)
			if err != nil && !errors.Is(err, ErrTerminate) {
				return err
			}

			return nil
		}()
	}()

	err = <-errChan
	return fmt.Errorf("forwarding data: %w", err)
}
