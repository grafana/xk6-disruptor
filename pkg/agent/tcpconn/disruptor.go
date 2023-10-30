// Package tcpconn contains a TCP connection disruptor.
package tcpconn

import (
	"context"
	"errors"
	"fmt"
	"time"
)

// Disruptor applies TCP Connection disruptions.
type Disruptor struct {
	Queue      Queue
	Disruption Disruption
	NFQConfig  NFQConfig
}

// Disruption holds the parameters that describe a TCP connection disruption.
type Disruption struct {
	// Port is the target port to match which connections will be intercepted.
	Port uint
	// DropRate is the rate in [0, 1] range of connections that should be dropped.
	DropRate float64
	// DropPeriod defines how often connections will be terminated according to DropRate.
	// If set to zero, connections will only be terminated once. If it is set to a non-zero value, connections will be
	// re-evaluated for termination when this period passes.
	DropPeriod time.Duration
}

// ErrDurationTooShort is returned when the supplied duration is smaller than 1s.
var ErrDurationTooShort = errors.New("duration must be at least 1 second")

// Apply executes the configured disruption for the specified duration.
func (d Disruptor) Apply(ctx context.Context, duration time.Duration) error {
	if duration < time.Second {
		return ErrDurationTooShort
	}

	// Set context timeout only if duration is not 0 (forever).
	ctx, cancel := context.WithTimeout(ctx, duration)
	defer cancel()

	packets := make(chan Packet, 2)
	defer close(packets)

	dropper := TCPConnectionDropper{
		DropRate: d.Disruption.DropRate,
	}

	go func() {
		for p := range packets {
			if dropper.Drop(p.Bytes()) {
				p.Reject()
				continue
			}

			p.Accept()
		}
	}()

	err := d.Queue.Start(ctx, packets)
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("packet handler: %w", err)
	}

	return nil
}
