package tcpconn

import (
	"encoding/hex"
	"net"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/google/gopacket/layers"
)

func Test_DropperFourTuple(t *testing.T) {
	t.Parallel()

	ipLayer := layers.IPv4{
		SrcIP: net.IPv4(0xaa, 0xbb, 0xcc, 0xdd),
		DstIP: net.IPv4(0xee, 0xff, 0x11, 0x22),
	}

	tcpLayer := layers.TCP{
		SrcPort: 0x1234,
		DstPort: 0x5678,
	}

	hash := make([]byte, 36)
	fourTuple(hash, &ipLayer, &tcpLayer)

	expected := "00000000000000000000ffffaabbccdd00000000000000000000ffffeeff112234127856"
	if diff := cmp.Diff(hex.EncodeToString(hash), expected); diff != "" {
		t.Fatalf("output hash does not match expected:\n%s", diff)
	}
}
