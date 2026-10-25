//go:build windows

package scan

import (
	"net"
	"net/netip"
	"strconv"
	"sync"
	"time"
	"unsafe"
)

const icmpPayload = "lslan-icmp-probe0000000000000000" // 32 bytes

type ipOptionInformation struct {
	TTL         uint8
	Tos         uint8
	Flags       uint8
	OptionsSize uint8
	_           uint32
	OptionsData uintptr
}

type icmpEchoReply struct {
	Address       uint32
	Status        uint32
	RoundTripTime uint32
	DataSize      uint16
	Reserved      uint16
	Data          uintptr
	Options       ipOptionInformation
}

var (
	icmpOnce sync.Once
	icmpOK   bool
)

// icmpAvailable reports whether the iphlpapi ICMP device can be opened.
// It needs no privileges; a failure means the TCP fallback is used.
func icmpAvailable() bool {
	icmpOnce.Do(func() {
		h, _, _ := procIcmpCreateFile.Call()
		if h != 0 && h != ^uintptr(0) {
			icmpOK = true
			procIcmpCloseHandle.Call(h)
		}
	})
	return icmpOK
}

// icmpPing sends one echo request and reports the measured round-trip
// time and the reply's TTL. ok is false on timeout or any error status.
func icmpPing(ip netip.Addr, timeout time.Duration) (rtt time.Duration, ttl int, ok bool) {
	if !ip.Is4() {
		return 0, 0, false
	}
	h, _, _ := procIcmpCreateFile.Call()
	if h == 0 || h == ^uintptr(0) {
		return 0, 0, false
	}
	defer procIcmpCloseHandle.Call(h)

	req := []byte(icmpPayload)
	reply := make([]byte, unsafe.Sizeof(icmpEchoReply{})+uintptr(len(req))+8)
	start := time.Now()
	n, _, _ := procIcmpSendEcho.Call(
		h,
		uintptr(ipAddrValue(ip)),
		uintptr(unsafe.Pointer(&req[0])),
		uintptr(len(req)),
		0,
		uintptr(unsafe.Pointer(&reply[0])),
		uintptr(len(reply)),
		uintptr(timeout.Milliseconds()),
	)
	rtt = time.Since(start)
	if n == 0 {
		return 0, 0, false
	}
	r := (*icmpEchoReply)(unsafe.Pointer(&reply[0]))
	if r.Status != 0 {
		return 0, 0, false
	}
	return rtt, int(r.Options.TTL), true
}

var tcpFallbackPorts = []int{80, 443, 445}

// tcpAlive is the liveness fallback when ICMP is unavailable.
func tcpAlive(ip netip.Addr, perPort time.Duration) bool {
	for _, p := range tcpFallbackPorts {
		c, err := net.DialTimeout("tcp", net.JoinHostPort(ip.String(), strconv.Itoa(p)), perPort)
		if err == nil {
			c.Close()
			return true
		}
	}
	return false
}
