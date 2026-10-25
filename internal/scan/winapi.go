//go:build windows

package scan

import (
	"encoding/binary"
	"net/netip"
	"syscall"
	"unsafe"
)

// The Windows API layer is built directly on iphlpapi.dll through the
// stdlib syscall package: golang.org/x/sys is not in the module graph,
// and the zero-pollution, unprivileged requirements rule out raw
// sockets. Structure layouts below must match the 64-bit ABI.

var (
	iphlpapi                 = syscall.NewLazyDLL("iphlpapi.dll")
	procIcmpCreateFile       = iphlpapi.NewProc("IcmpCreateFile")
	procIcmpCloseHandle      = iphlpapi.NewProc("IcmpCloseHandle")
	procIcmpSendEcho         = iphlpapi.NewProc("IcmpSendEcho")
	procGetAdaptersAddresses = iphlpapi.NewProc("GetAdaptersAddresses")
	procGetIpNetTable        = iphlpapi.NewProc("GetIpNetTable")
)

const (
	afInet        = 2
	ifOperUp      = 1
	ifTypeLoop    = 24
	arpTypeDyn    = 3
	arpTypeStatic = 4

	gaaIncludePrefix   = 0x10
	gaaIncludeGateways = 0x40

	errBufferOverflow       = 111 // ERROR_BUFFER_OVERFLOW
	errInsufficientBuffer   = 122 // ERROR_INSUFFICIENT_BUFFER
	errFilenameExcedRange   = 206 // ERROR_FILENAME_EXCED_RANGE
	maxAdapterAddressLength = 8
)

// sockaddrIPv4 reads an IPv4 address out of a SOCKET_ADDRESS field.
// Windows stores IPAddr values so that the octets appear in memory in
// network order; on little-endian that means reading the raw bytes.
func sockaddrIPv4(sa socketAddress) (netip.Addr, bool) {
	if sa.Sockaddr == nil || sa.Len < 4 {
		return netip.Addr{}, false
	}
	if sa.Sockaddr.Family != afInet {
		return netip.Addr{}, false
	}
	return netip.AddrFrom4(sa.Sockaddr.Addr), true
}

type sockaddrIn struct {
	Family uint16
	Port   uint16
	Addr   [4]byte
	Zero   [8]byte
}

type socketAddress struct {
	Sockaddr *sockaddrIn
	Len      int32
	_        uint32
}

// utf16Str reads a NUL-terminated UTF-16 string pointer.
func utf16Str(p *uint16) string {
	if p == nil {
		return ""
	}
	n := 0
	for *(*uint16)(unsafe.Add(unsafe.Pointer(p), uintptr(2*n))) != 0 {
		n++
	}
	return syscall.UTF16ToString(unsafe.Slice(p, n))
}

// formatMAC renders a hardware address as "aa:bb:cc:dd:ee:ff".
func formatMAC(b []byte) string {
	const hexd = "0123456789abcdef"
	out := make([]byte, 0, len(b)*3)
	for i, x := range b {
		if i > 0 {
			out = append(out, ':')
		}
		out = append(out, hexd[x>>4], hexd[x&0xf])
	}
	return string(out)
}

// ipAddrValue converts an IPv4 address to the IPAddr DWORD the iphlpapi
// functions expect: octets in memory in network order.
func ipAddrValue(a netip.Addr) uint32 {
	b := a.As4()
	return binary.LittleEndian.Uint32(b[:])
}
