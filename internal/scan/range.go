package scan

import (
	"encoding/binary"
	"net/netip"

	"lslan/internal/model"
)

// RangeForNIC returns the default sweep range for a NIC: the full IPv4
// subnet the NIC's own address belongs to (a /24 is capped at 254
// hosts by the caller).
func RangeForNIC(nic model.NIC) (start, end netip.Addr) {
	if !nic.IP.IsValid() || !nic.IP.Is4() {
		return netip.Addr{}, netip.Addr{}
	}
	plen := nic.PrefixLen
	if plen < 0 || plen > 32 {
		plen = 32
	}
	ip := be32(nic.IP)
	var mask uint32
	if plen > 0 {
		mask = ^uint32(0) << (32 - plen)
	}
	network := ip & mask
	broadcast := network | ^mask
	if plen >= 31 {
		// No reserved network/broadcast address: every address is usable.
		return addrFromUint32(network), addrFromUint32(broadcast)
	}
	return addrFromUint32(network + 1), addrFromUint32(broadcast - 1)
}

// be32 returns the IPv4 address as its host-order numeric value, the
// index space sweeps iterate over.
func be32(a netip.Addr) uint32 {
	b := a.As4()
	return binary.BigEndian.Uint32(b[:])
}

func addrFromUint32(v uint32) netip.Addr {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], v)
	return netip.AddrFrom4(b)
}
