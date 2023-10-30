package tcpconn

import (
	"context"
	"fmt"
	"log"
	"math/rand"

	"github.com/florianl/go-nfqueue"
	"github.com/grafana/xk6-disruptor/pkg/iptables"
	"github.com/grafana/xk6-disruptor/pkg/runtime"
)

// NFQConfig contains netfilter queue IDs that iptables rules and the packet handler use to communicate.
type NFQConfig struct {
	// QueueID is an arbitrary integer used to identify a queue where a handler listens and a disruptor redirects target
	// packets.
	QueueID uint16
	// RejectMark is an arbitrary integer which the handler uses to mark packets that need to be dropped.
	RejectMark uint32
}

// RandomNFQConfig returns a NFQConfig with two random integers to be used as queue IDs and reject mark.
// To ensure the numbers are not zero they are ORed with 0b1, as adding 1 can actually result in the number overflowing
// and becoming zero.
func RandomNFQConfig() NFQConfig {
	return NFQConfig{
		QueueID:    uint16(rand.Int31()) | 0b1,
		RejectMark: uint32(rand.Int31()) | 0b1,
	}
}

// Queue is the interface implemented by objects that can write packets to a channel.
type Queue interface {
	// Start starts listening to packets, blocking until an error occurs or the context is cancelled.
	Start(ctx context.Context, packets chan<- Packet) error
}

// Packet is a packet received by a Queue. It can describe itself and be accepted or rejected.
type Packet interface {
	// Bytes returns the raw bytes that make the packet.
	Bytes() []byte
	// Accept marks the packet to be accepted.
	Accept()
	// Reject marks the packet to be rejected.
	Reject()
}

// FakeQueue is a fake implementation of queue. It stalls until the context is cancelled, never sending any packet.
type FakeQueue struct{}

// Start implements the Queue interface.
func (f FakeQueue) Start(ctx context.Context, _ chan<- Packet) error {
	<-ctx.Done()
	return ctx.Err()
}

// NFQueue implements the Queue interface using netfilter's nfqueue mechanism, reading packets sent to userspace by
// netfilter. NFQueue will process packets sent to the NFQUEUE chain with queue id specified in NFQConfig.QueueID.
// Accept()ed packets are immediately forwarded to the ACCEPT chain and do not traverse subsequent iptables rules.
// Reject()ed packets are requeued with the mark specified in NFQConfig.RejectMark.
// Iptables rules should take care to _not_ send again to the NFQUEUE target packets that have been marked with the
// RejectMark, and should direct those to the REJECT chain instead.
type NFQueue struct {
	Executor   runtime.Executor
	NFQConfig  NFQConfig
	Disruption Disruption
}

// Start sets up a nfqueue handler and starts handling packets sent to it. It blocks until an error occurs, or until
// the supplied context is canceled.
func (q NFQueue) Start(ctx context.Context, packetChan chan<- Packet) error {
	iptables := iptables.New(q.Executor)
	//nolint:errcheck // Nothing to do while we don't implement logging.
	defer iptables.Remove()

	for _, r := range q.rules() {
		err := iptables.Add(r)
		if err != nil {
			return err
		}
	}

	queue, err := nfqueue.Open(&nfqueue.Config{
		NfQueue:  q.NFQConfig.QueueID,
		Copymode: nfqueue.NfQnlCopyPacket, // Copymode must be set to NfQnlCopyPacket to be able to read the packet.

		// TODO: Refine this magic value. Larger values will cause nfqueue to error such as:
		// netlink receive: recvmsg: no buffer space available
		// Likely this means that we're trying to use too much memory for this queue.
		MaxQueueLen:  32,
		MaxPacketLen: 0xffff, // TODO: This can probably be smaller for IPv4 on top of ethernet (1500 mtu).
	})
	if err != nil {
		return fmt.Errorf("creating nfqueue: %w", err)
	}

	//nolint:errcheck
	defer queue.Close()

	// nfqueue processes packets in order: Until a veredict is emitted for a packet, the callback function will not
	// be invoked again.
	err = queue.RegisterWithErrorFunc(ctx,
		func(packet nfqueue.Attribute) int {
			packetChan <- NFPacket{
				packet:     packet,
				queue:      queue,
				rejectMark: q.NFQConfig.RejectMark,
			}
			return 0
		},
		func(err error) int {
			// TODO: Handle errors.
			log.Printf("nfq error: %v", err)
			return 0
		},
	)
	if err != nil {
		return fmt.Errorf("registering nqueue handlers: %w", err)
	}

	<-ctx.Done()
	return ctx.Err()
}

// rules returns the iptables rules that need to be set in place for the disruption to work.
// These rules are safe by default, meaning that if the handler is offline and rules are left over, no packet will be
// dropped.
func (q NFQueue) rules() []iptables.Rule {
	return []iptables.Rule{
		{
			// This rule rejects with tcp-reset traffic arriving to the disruption port if it has the RejectMark set by
			// the queue when Reject() is called on a packet. Packets that are Reject()ed are requeued and thus will
			// traverse this rule again even if they didn't the first time they arrived, when they weren't marked.
			Table: "filter", Chain: "INPUT", Args: fmt.Sprintf(
				"-p tcp --dport %d -m mark --mark %d -j REJECT --reject-with tcp-reset",
				q.Disruption.Port, q.NFQConfig.RejectMark,
			),
		},
		{
			// This rule sends other (non-marked) traffic to the queue, so it can make a decision over whether to
			// drop it or not.
			// --queue-bypass instruct netfilter to ACCEPT packets if nothing is listening on this queue.
			Table: "filter", Chain: "INPUT", Args: fmt.Sprintf(
				"-p tcp --dport %d -j NFQUEUE --queue-num %d --queue-bypass",
				q.Disruption.Port, q.NFQConfig.QueueID,
			),
		},
	}
}

// NFPacket wraps netfilter.NFPacket so it implements the Packet interface.
type NFPacket struct {
	packet     nfqueue.Attribute
	queue      *nfqueue.Nfqueue
	rejectMark uint32
}

// Bytes returns the payload of the packet.
func (n NFPacket) Bytes() []byte {
	return *n.packet.Payload
}

// Accept accepts the packet.
func (n NFPacket) Accept() {
	_ = n.queue.SetVerdict(*n.packet.PacketID, nfqueue.NfAccept)
}

// Reject requeues the packet and sets the configured reject mark on it.
func (n NFPacket) Reject() {
	_ = n.queue.SetVerdictWithMark(*n.packet.PacketID, nfqueue.NfRepeat, int(n.rejectMark))
}
