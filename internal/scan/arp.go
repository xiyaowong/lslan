//go:build windows

package scan

import (
	"encoding/binary"
	"net/netip"
	"sync"
	"time"
	"unsafe"
)

type mibIpNetRow struct {
	Index       uint32
	PhysAddrLen uint32
	PhysAddr    [maxAdapterAddressLength]byte
	Addr        uint32
	Type        uint32
}

type mibIpNetTable struct {
	NumEntries uint32
	Rows       [1]mibIpNetRow
}

// readARPTable returns the system ARP cache as ip -> "aa:bb:cc:dd:ee:ff",
// keeping dynamic and static entries. Entries whose hardware address is
// incomplete are skipped. It returns nil if the API call fails; callers
// treat that as an empty cache.
func readARPTable() map[netip.Addr]string {
	var size uint32
	ret, _, _ := procGetIpNetTable.Call(0, uintptr(unsafe.Pointer(&size)), 0)
	if ret != 0 && ret != errInsufficientBuffer {
		return nil
	}
	if size == 0 {
		return nil
	}
	buf := make([]byte, size)
	ret, _, _ = procGetIpNetTable.Call(uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)), 0)
	if ret != 0 {
		return nil
	}
	t := (*mibIpNetTable)(unsafe.Pointer(&buf[0]))
	n := int(t.NumEntries)
	out := make(map[netip.Addr]string, n)
	for _, row := range unsafe.Slice(&t.Rows[0], n) {
		if row.Type != arpTypeDyn && row.Type != arpTypeStatic {
			continue
		}
		if row.PhysAddrLen == 0 || row.PhysAddrLen > maxAdapterAddressLength {
			continue
		}
		var ipb [4]byte
		binary.LittleEndian.PutUint32(ipb[:], row.Addr)
		out[netip.AddrFrom4(ipb)] = formatMAC(row.PhysAddr[:row.PhysAddrLen])
	}
	return out
}

// arpCache is a time-windowed snapshot of the system ARP cache shared
// by all sweep workers: the table is re-read at most once per window
// no matter how many workers ask, keeping the GetIpNetTable call rate
// flat instead of one call per probe.
type arpCache struct {
	read   func() map[netip.Addr]string
	window time.Duration
	now    func() time.Time

	mu      sync.Mutex
	table   map[netip.Addr]string
	expires time.Time
}

func newARPCache(window time.Duration) *arpCache {
	return &arpCache{read: readARPTable, window: window, now: time.Now}
}

// mac returns the hardware address for ip, refreshing the snapshot
// when the window has lapsed. ok is false when the snapshot holds no
// entry for ip; a failed table read is cached as empty for the window.
func (c *arpCache) mac(ip netip.Addr) (string, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.table == nil || !c.expires.After(c.now()) {
		c.table = c.read()
		if c.table == nil {
			c.table = map[netip.Addr]string{}
		}
		c.expires = c.now().Add(c.window)
	}
	mac, ok := c.table[ip]
	return mac, ok
}
