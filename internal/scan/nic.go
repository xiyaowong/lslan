//go:build windows

package scan

import (
	"cmp"
	"fmt"
	"net/netip"
	"slices"
	"syscall"
	"unsafe"

	"lslan/internal/model"
)

// Field layout mirrors IP_ADAPTER_ADDRESSES through FirstGatewayAddress
// (Win Vista+ 64-bit ABI). The first 8 bytes are a union holding
// Length+IfIndex (as two ulongs on 64-bit); only the fields below are
// read, so the rest of the record is left undefined.
type ipAdapterAddresses struct {
	Union                  [8]byte // Length, IfIndex
	Next                   *ipAdapterAddresses
	AdapterName            *byte
	FirstUnicastAddress    *ipAdapterUnicastAddress
	FirstAnycastAddress    uintptr
	FirstMulticastAddress  uintptr
	FirstDnsServerAddress  uintptr
	DnsSuffix              *uint16
	Description            *uint16
	FriendlyName           *uint16
	PhysicalAddress        [maxAdapterAddressLength]byte
	PhysicalAddressLength  uint32
	Flags                  uint32
	Mtu                    uint32
	IfType                 uint32
	OperStatus             uint32
	Ipv6IfIndex            uint32
	ZoneIndices            [16]uint32
	FirstPrefix            uintptr
	TransmitLinkSpeed      uint64
	ReceiveLinkSpeed       uint64
	FirstWinsServerAddress uintptr
	FirstGatewayAddress    *ipAdapterGatewayAddress
}

// Mirror of IP_ADAPTER_UNICAST_ADDRESS_LH: the first 8 bytes are a
// union holding Length+Flags.
type ipAdapterUnicastAddress struct {
	Union              [8]byte // Length, Flags
	Next               *ipAdapterUnicastAddress
	Address            socketAddress
	PrefixOrigin       uint32
	SuffixOrigin       uint32
	DadState           uint32
	ValidLifetime      uint32
	PreferredLifetime  uint32
	LeaseLifetime      uint32
	OnLinkPrefixLength uint8
	_                  [7]byte
}

// Mirror of IP_ADAPTER_GATEWAY_ADDRESS_LH: the first 8 bytes are a
// union holding Length+Reserved.
type ipAdapterGatewayAddress struct {
	Union   [8]byte // Length, Reserved
	Next    *ipAdapterGatewayAddress
	Address socketAddress
}

// ListNICs returns the machine's usable IPv4 adapters (loopback
// excluded, gateway resolved when known), ordered by interface index.
func ListNICs() ([]model.NIC, error) {
	flags := uintptr(gaaIncludePrefix | gaaIncludeGateways)
	var size uint32
	buf := []byte{}
	for range 5 {
		var bufPtr uintptr
		if len(buf) > 0 {
			bufPtr = uintptr(unsafe.Pointer(&buf[0]))
		}
		sizePtr := uintptr(unsafe.Pointer(&size))
		ret, _, _ := procGetAdaptersAddresses.Call(afInet, flags, 0, bufPtr, sizePtr)
		if ret == 0 {
			return parseAdapters(buf), nil
		}
		// 206 (ERROR_FILENAME_EXCED_RANGE) is what the probe returns when
		// the reported size was not honored; 0 means the API gave no size
		// hint, so fall back to the documented 15KB starting buffer.
		if ret != errBufferOverflow && ret != errFilenameExcedRange {
			return nil, fmt.Errorf("scan: GetAdaptersAddresses: %w", syscall.Errno(ret))
		}
		if size == 0 {
			size = 15000
		}
		buf = make([]byte, size)
	}
	return nil, fmt.Errorf("scan: GetAdaptersAddresses: buffer kept resizing")
}

func parseAdapters(buf []byte) []model.NIC {
	var nics []model.NIC
	if len(buf) == 0 {
		return nics
	}
	for aa := (*ipAdapterAddresses)(unsafe.Pointer(&buf[0])); aa != nil; aa = aa.Next {
		if aa.OperStatus != ifOperUp || aa.IfType == ifTypeLoop {
			continue
		}
		ip, plen, ok := firstIPv4Unicast(aa.FirstUnicastAddress)
		if !ok || plen == 0 {
			// A /0 "subnet" is never a real LAN; skip such virtual adapters.
			continue
		}
		nic := model.NIC{
			Index:     ifIndex(aa),
			Name:      utf16Str(aa.FriendlyName),
			IP:        ip,
			PrefixLen: plen,
			Virtual:   isVirtualIfType(aa.IfType),
		}
		n := min(int(aa.PhysicalAddressLength), maxAdapterAddressLength)
		nic.MAC = formatMAC(aa.PhysicalAddress[:n])
		if aa.FirstGatewayAddress != nil {
			if g, ok := sockaddrIPv4(aa.FirstGatewayAddress.Address); ok {
				nic.Gateway = g
			}
		}
		nics = append(nics, nic)
	}
	// Physical adapters first so the UI's default pick is the real LAN.
	slices.SortStableFunc(nics, func(a, b model.NIC) int {
		if a.Virtual != b.Virtual {
			if a.Virtual {
				return 1
			}
			return -1
		}
		return cmp.Compare(a.Index, b.Index)
	})
	return nics
}

// ifIndex reads the adapter's IPv4 interface index out of the union that
// opens IP_ADAPTER_ADDRESSES: on this ABI it holds Length then IfIndex,
// both as 4-byte fields.
func ifIndex(aa *ipAdapterAddresses) int {
	var idx uint32
	copy((*[4]byte)(unsafe.Pointer(&idx))[:], aa.Union[4:8])
	return int(idx)
}

// isVirtualIfType reports whether an IANA interface type is a virtual
// adapter (TUN/TAP, tunnel, loopback-like) rather than physical LAN.
func isVirtualIfType(t uint32) bool {
	switch t {
	case 53, // propVirtual
		131, // tunnel
		24,  // softwareLoopback
		999: // (defensive; some drivers report 999 for TUN)
		return true
	}
	return false
}

func firstIPv4Unicast(u *ipAdapterUnicastAddress) (ip netip.Addr, plen int, ok bool) {
	for ; u != nil; u = u.Next {
		a, is4 := sockaddrIPv4(u.Address)
		if !is4 {
			continue
		}
		return a, int(u.OnLinkPrefixLength), true
	}
	return netip.Addr{}, 0, false
}
