package wire

import (
	"bytes"
	"testing"
	"time"
)

func TestAddrV2RejectsLegacyAllocationPayload(t *testing.T) {
	// The legacy addr2 encoding claimed a uint64-sized address in 12 bytes.
	payload := []byte{1, 1, 4, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}
	var msg MsgAddrV2
	if err := msg.BtcDecode(bytes.NewReader(payload), AddrV2Version); err == nil {
		t.Fatal("accepted malformed legacy address payload")
	}
}

func TestAddrV2PreservesGossipTimestamps(t *testing.T) {
	timestamp := time.Unix(1700000000, 0)
	for _, tc := range []struct {
		name string
		typ  NetAddressType
		addr []byte
	}{
		{"IPv4", IPv4Address, []byte{8, 8, 8, 8}},
		{"IPv6", IPv6Address, make([]byte, 16)},
		{"TorV3", TorV3Address, make([]byte, 32)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			msg := NewMsgAddrV2([]NetAddressV2{
				NewNetAddressV2(tc.typ, tc.addr, 9508, timestamp, SFNodeNetwork),
			})
			var encoded bytes.Buffer
			if err := msg.BtcEncode(&encoded, AddrV2Version); err != nil {
				t.Fatal(err)
			}
			var decoded MsgAddrV2
			if err := decoded.BtcDecode(&encoded, AddrV2Version); err != nil {
				t.Fatal(err)
			}
			if len(decoded.AddrList) != 1 || !decoded.AddrList[0].Timestamp.Equal(timestamp) {
				t.Fatalf("gossip timestamp was lost: %+v", decoded.AddrList)
			}
		})
	}
}
