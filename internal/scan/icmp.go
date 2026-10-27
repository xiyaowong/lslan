//go:build windows

package scan

import (
	"context"
	"errors"
	"net/netip"
	"sync"
	"syscall"
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

// icmpAvailable reports whether the iphlpapi ICMP device can be opened.
// It needs no privileges; a failure means the TCP fallback is used.
var icmpAvailable = sync.OnceValue(func() bool {
	h, _, _ := procIcmpCreateFile.Call()
	if h == 0 || h == ^uintptr(0) {
		return false
	}
	procIcmpCloseHandle.Call(h)
	return true
})

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

// tcpAliveFrom is the liveness fallback when ICMP is unavailable: a TCP
// connect from src, the adapter's address, so a VPN's TUN adapter cannot
// answer for a device that is not there. A refused connection counts as
// alive: the host answered, it just has nothing listening.
func tcpAliveFrom(src, ip netip.Addr, perPort time.Duration) bool {
	for _, p := range tcpFallbackPorts {
		c, err := dialFrom(context.Background(), src, ip, p, perPort)
		if err == nil {
			c.Close()
			return true
		}
		if refused(err) {
			return true
		}
	}
	return false
}

// refused reports whether the dial was rejected by the host itself, which
// is proof it is up. Windows reports WSAECONNREFUSED (10061), which does
// not compare equal to the Unix ECONNREFUSED through errors.Is.
func refused(err error) bool {
	var errno syscall.Errno
	if !errors.As(err, &errno) {
		return false
	}
	return errno == wsaConnRefused || errno == syscall.ECONNREFUSED
}

// wsaConnRefused is WSAECONNREFUSED, the Windows spelling of ECONNREFUSED.
const wsaConnRefused syscall.Errno = 10061
