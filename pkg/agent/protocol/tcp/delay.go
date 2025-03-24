package tcp

import (
	"fmt"
	"io"
	"math/rand"
	"time"
)

type DelayHandler struct {
	delay     time.Duration
	variation float64

	lastDelayed time.Time
}

func DelayHandlerBuilder(delay time.Duration, variation float64) HandlerBuilder {
	return func(ConnMeta) Handler {
		return &DelayHandler{
			delay:     delay,
			variation: variation,
		}
	}
}

func (d *DelayHandler) HandleUpward(client io.Reader, server io.Writer) error {
	// Buffer of 2048 bytes for reading a TCP segment. 2048 is the smallest power of two able to hold the most common
	// TCP MSS, 1500 - (20 + 20) = 1460 bytes.
	buf := make([]byte, 2048)

	for {
		// Wait until there is one byte available.
		_, err := client.Read(buf[:1])
		if err != nil {
			return fmt.Errorf("reading from downstream: %w", err)
		}

		// After the first byte reception, introduce delay if we need to.
		d.waitIfNeeded()

		// Read the rest of the TCP frame *up to* len(buf).
		// Stream-based implementations of io.Reader return early if there is no more data
		n, err := client.Read(buf[1:])
		if err != nil {
			return fmt.Errorf("reading from downstream: %w", err)
		}

		// Write the amount read plus the first byte.
		_, err = server.Write(buf[:n+1])
		if err != nil {
			return fmt.Errorf("writing to upstream: %w", err)
		}
	}
}

func (d *DelayHandler) HandleDownward(server io.Reader, client io.Writer) error {
	_, err := io.Copy(client, server)
	// Copy dos not return EOF.
	return fmt.Errorf("relaying data downstream: %w", err)
}

func (d *DelayHandler) waitIfNeeded() {
	if time.Since(d.lastDelayed) < time.Second {
		return
	}

	d.lastDelayed = time.Now()
	plusMinus := rand.Float64()*d.variation*2 - d.variation
	delay := time.Duration(float64(d.delay) * (1 + plusMinus))

	time.Sleep(delay)
}
