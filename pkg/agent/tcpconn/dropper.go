package tcpconn

import (
	"encoding/binary"
	"hash/crc32"

	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
)

// Dropper is an interface implemented by objects that decide whether a packet should be dropped (rejected).
type Dropper interface {
	Drop(packetBytes []byte) bool
}

// TCPConnectionDropper is a dropper that drops a defined percentage of TCP the connections it sees.
type TCPConnectionDropper struct {
	DropRate float64
}

// Drop decides whether a packet should be dropped by taking the modulus of hash of the connection it belongs to and
// comparing it to a threshold derived from DropRate.
func (tcd TCPConnectionDropper) Drop(packetBytes []byte) bool {
	packet := gopacket.NewPacket(packetBytes, layers.LayerTypeIPv4, gopacket.Default)

	ipLayer := packet.Layer(layers.LayerTypeIPv4)
	if ipLayer == nil {
		return false
	}
	ip, _ := ipLayer.(*layers.IPv4)

	tcpLayer := packet.Layer(layers.LayerTypeTCP)
	if tcpLayer == nil {
		return false
	}
	tcp, _ := tcpLayer.(*layers.TCP)

	ftBuf := make([]byte, 36)
	fourTuple(ftBuf, ip, tcp)

	hash := crc32.NewIEEE()
	_, _ = hash.Write(ftBuf)
	checksum := hash.Sum32()

	return (checksum % 100) < uint32(100*tcd.DropRate)
}

// 4 tuple writes the 4-tuple of a given packet in to dst, given its ip and tcp layers.
// dst must be at least 36 bytes long.
func fourTuple(dst []byte, ip *layers.IPv4, tcp *layers.TCP) {
	offset := 0

	// Go's ip.IP representation can always be 16 bytes, even for v4 addresses.
	copy(dst[offset:], ip.SrcIP)
	offset += 16

	copy(dst[offset:], ip.DstIP)
	offset += 16

	binary.LittleEndian.PutUint16(dst[offset:], uint16(tcp.SrcPort))
	offset += 2

	binary.LittleEndian.PutUint16(dst[offset:], uint16(tcp.DstPort))
}
