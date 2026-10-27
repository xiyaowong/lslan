package scan

import (
	"testing"
	"unsafe"
)

// TestWindowsLayouts: the mirror structs are written by hand, and a wrong
// offset does not fail — it reads a different field and returns garbage
// addresses instead. These offsets are the 64-bit ABI's (checked against
// the SDK headers); if one drifts the test says so instead of the scanner
// quietly lying.
func TestWindowsLayouts(t *testing.T) {
	const ptr = 8 // amd64
	cases := []struct {
		name string
		got  uintptr
		want uintptr
	}{
		// IP_ADAPTER_ADDRESSES_LH.
		{"aa Union", unsafe.Offsetof(ipAdapterAddresses{}.Union), 0},
		{"aa Next", unsafe.Offsetof(ipAdapterAddresses{}.Next), 8},
		{"aa AdapterName", unsafe.Offsetof(ipAdapterAddresses{}.AdapterName), 16},
		{"aa FirstUnicastAddress", unsafe.Offsetof(ipAdapterAddresses{}.FirstUnicastAddress), 24},
		{"aa FriendlyName", unsafe.Offsetof(ipAdapterAddresses{}.FriendlyName), 72},
		{"aa PhysicalAddress", unsafe.Offsetof(ipAdapterAddresses{}.PhysicalAddress), 80},
		{"aa IfType", unsafe.Offsetof(ipAdapterAddresses{}.IfType), 100},
		{"aa ZoneIndices", unsafe.Offsetof(ipAdapterAddresses{}.ZoneIndices), 112},
		{"aa FirstGatewayAddress", unsafe.Offsetof(ipAdapterAddresses{}.FirstGatewayAddress), 208},
		{"aa size", unsafe.Sizeof(ipAdapterAddresses{}), 216},

		// IP_ADAPTER_UNICAST_ADDRESS_LH.
		{"ua Next", unsafe.Offsetof(ipAdapterUnicastAddress{}.Next), 8},
		{"ua Address", unsafe.Offsetof(ipAdapterUnicastAddress{}.Address), 16},
		{"ua OnLinkPrefixLength", unsafe.Offsetof(ipAdapterUnicastAddress{}.OnLinkPrefixLength), 56},
		{"ua size", unsafe.Sizeof(ipAdapterUnicastAddress{}), 64},

		// IP_ADAPTER_GATEWAY_ADDRESS_LH.
		{"ga Next", unsafe.Offsetof(ipAdapterGatewayAddress{}.Next), 8},
		{"ga Address", unsafe.Offsetof(ipAdapterGatewayAddress{}.Address), 16},
		{"ga size", unsafe.Sizeof(ipAdapterGatewayAddress{}), 32},

		// The ICMP reply the echo call fills.
		{"icmp Address", unsafe.Offsetof(icmpEchoReply{}.Address), 0},
		{"icmp Status", unsafe.Offsetof(icmpEchoReply{}.Status), 4},
		{"icmp RoundTripTime", unsafe.Offsetof(icmpEchoReply{}.RoundTripTime), 8},
		{"icmp DataSize", unsafe.Offsetof(icmpEchoReply{}.DataSize), 12},

		// The ARP table row.
		{"arp Index", unsafe.Offsetof(mibIpNetRow{}.Index), 0},
		{"arp PhysAddrLen", unsafe.Offsetof(mibIpNetRow{}.PhysAddrLen), 4},
		{"arp PhysAddr", unsafe.Offsetof(mibIpNetRow{}.PhysAddr), 8},
		{"arp Addr", unsafe.Offsetof(mibIpNetRow{}.Addr), 16},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s at %d, want %d", c.name, c.got, c.want)
		}
	}
	_ = ptr
}

// TestIfIndexOffset: the index lives at bytes 4..8 of the leading union,
// which is where ifIndex reads it.
func TestIfIndexOffset(t *testing.T) {
	if got := unsafe.Sizeof(ipAdapterAddresses{}.Union); got != 8 {
		t.Fatalf("the leading union is %d bytes, want 8", got)
	}
}
