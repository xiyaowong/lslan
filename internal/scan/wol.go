//go:build windows

package scan

import (
	"errors"
	"net"
	"net/netip"

	"lslan/internal/model"
)

var errBadMAC = errors.New("scan: invalid MAC address")

// wakePorts are where a sleeping card listens: 9 is the convention, 7 the
// older one, and many cards take either.
var wakePorts = []int{9, 7}

// ParseMAC normalizes a hardware address in any common spelling
// ("aa:bb:cc:dd:ee:ff", "aa-bb-…", "aabbccddeeff") to the colon form, for
// showing it; ok is false unless it holds exactly twelve hex digits.
func ParseMAC(s string) (string, bool) {
	b, ok := parseMAC(s)
	if !ok {
		return "", false
	}
	return formatMAC(b[:]), true
}

// parseMAC reads the six octets of a hardware address, allowing the
// colon, dash and bare 12-digit spellings; anything else is rejected
// rather than guessed at.
func parseMAC(s string) ([6]byte, bool) {
	var b [6]byte
	n := 0
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ':' || c == '-' || c == '.' || c == ' ' {
			continue
		}
		var v byte
		switch {
		case c >= '0' && c <= '9':
			v = c - '0'
		case c >= 'a' && c <= 'f':
			v = c - 'a' + 10
		case c >= 'A' && c <= 'F':
			v = c - 'A' + 10
		default:
			return b, false
		}
		if n >= 12 {
			return b, false
		}
		b[n/2] |= v << (4 * (1 - n%2))
		n++
	}
	return b, n == 12
}

// magicPacket is the wake-up frame: six 0xFF bytes, then the hardware
// address sixteen times.
func magicPacket(mac [6]byte) []byte {
	p := make([]byte, 6+16*len(mac))
	for i := range 6 {
		p[i] = 0xff
	}
	for i := 6; i < len(p); i += len(mac) {
		copy(p[i:], mac[:])
	}
	return p
}

// SendWake sends the frame that powers a sleeping device on, three rounds
// to the limited broadcast and to the LAN's own: a switch may drop one of
// the two, and a lost datagram is silent otherwise.
func SendWake(nic model.NIC, mac string) error {
	m, ok := parseMAC(mac)
	if !ok {
		return errBadMAC
	}
	pkt := magicPacket(m)
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{})
	if err != nil {
		return err
	}
	defer conn.Close()
	sent := false
	for _, dst := range wakeTargets(nic) {
		for _, port := range wakePorts {
			addr := &net.UDPAddr{IP: dst, Port: port}
			for range 3 {
				if _, err := conn.WriteToUDP(pkt, addr); err == nil {
					sent = true
				}
			}
		}
	}
	if !sent {
		return errors.New("scan: the wake frame went nowhere")
	}
	return nil
}

// wakeTargets is where the frame goes: the limited broadcast always, and
// the LAN's own broadcast when the adapter names a subnet.
func wakeTargets(nic model.NIC) []net.IP {
	dsts := []net.IP{net.IPv4bcast}
	b, ok := subnetBroadcast(nic.IP, nic.PrefixLen)
	if !ok {
		return dsts
	}
	if ip := net.IP(b.AsSlice()); !ip.Equal(net.IPv4bcast) {
		dsts = append(dsts, ip)
	}
	return dsts
}

// subnetBroadcast is the all-ones address of ip's subnet.
func subnetBroadcast(ip netip.Addr, plen int) (netip.Addr, bool) {
	if !ip.Is4() || plen < 0 || plen > 32 {
		return netip.Addr{}, false
	}
	mask := ^uint32(0) << (32 - plen)
	return addrFromUint32(be32(ip) | ^mask), true
}

// MACFor returns the hardware address the system's ARP table has for ip,
// "" when it has none — the last word on a device that is off now.
func MACFor(ip netip.Addr) string { return readARPTable()[ip] }

// Probe reports whether ip answers now, by the sweep's own liveness
// check: an ICMP echo where available, else a TCP connect.
func Probe(ip netip.Addr) bool { return probeHost(ip) }
