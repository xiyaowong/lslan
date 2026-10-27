package ui

import (
	"encoding/binary"
	"net/netip"
	"strconv"
)

// fillRange fills the sweep range from the NIC, trimmed to usable hosts.
func (a *app) fillRange() {
	nic := a.selectedNIC()
	if nic == nil {
		return
	}
	start, end := a.rangeForNIC(*nic)
	start, end = trimRangeEdges(start, end)
	a.setRange(start, end)
}

func (a *app) setRange(start, end netip.Addr) {
	if !start.IsValid() || !end.IsValid() {
		return
	}
	sb, eb := start.As4(), end.As4()
	for i := range 4 {
		a.ipCells[i], a.ipCells[i+4] = strconv.Itoa(int(sb[i])), strconv.Itoa(int(eb[i]))
	}
	if _, _, _, ok := splitRange(start, end); ok {
		a.cellLock = true
	} else {
		a.cellLock = false
	}
	a.lastCell = -1
}

// unlockRange frees the network octets so another subnet can be typed.
// The lock exists so a typo cannot quietly point a sweep at the wrong
// network; a household with a second subnet needs it off.
func (a *app) unlockRange() { a.cellLock = false }

// cellLocked tells whether cell i shows a locked network octet: in a
// locked control, every cell but the host cell of each address.
func (a *app) cellLocked(i int) bool {
	return a.cellLock && i%4 != 3
}

func (a *app) octet(i int) int {
	n, err := strconv.Atoi(a.ipCells[i])
	if err != nil {
		return -1
	}
	return n
}

// cellAddr builds the address of the first (base 0) or second (base 4)
// cell group; ok is false while any of its cells is not a full octet.
func (a *app) cellAddr(base int) (netip.Addr, bool) {
	var b [4]byte
	for i := range 4 {
		n := a.octet(base + i)
		if n < 0 {
			return netip.Addr{}, false
		}
		b[i] = byte(n)
	}
	return netip.AddrFrom4(b), true
}

// fullRange reads the segmented control as an IPv4 span; it fails while
// the user is mid-edit and an address is incomplete.
func (a *app) fullRange() (start, end netip.Addr, ok bool) {
	start, ok1 := a.cellAddr(0)
	end, ok2 := a.cellAddr(4)
	if !ok1 || !ok2 {
		return start, end, false
	}
	if end.Compare(start) < 0 {
		start, end = end, start
	}
	return start, end, true
}

// parseRange reads the cells as a valid ordered span; startScan goes
// through it.
func (a *app) parseRange() (start, end netip.Addr, ok bool) {
	start, end, ok = a.fullRange()
	return start, end, ok
}

func (a *app) narrowRange() {
	start, _, ok := a.fullRange()
	if !ok {
		return
	}
	b := start.As4()
	a.setRange(
		netip.AddrFrom4([4]byte{b[0], b[1], b[2], 1}),
		netip.AddrFrom4([4]byte{b[0], b[1], b[2], 254}),
	)
}

func (a *app) rangeTotal() (uint64, bool) {
	start, end, ok := a.fullRange()
	if !ok {
		return 0, false
	}
	return rangeCount(start, end)
}

// trimRangeEdges drops network/broadcast addresses of a subnet-sized range.
func trimRangeEdges(start, end netip.Addr) (netip.Addr, netip.Addr) {
	n, ok := rangeCount(start, end)
	if !ok || n > 256 {
		return start, end
	}
	sb, eb := start.As4(), end.As4()
	if sb[3] == 0 {
		start = netip.AddrFrom4([4]byte{sb[0], sb[1], sb[2], 1})
	}
	if eb[3] == 255 {
		end = netip.AddrFrom4([4]byte{eb[0], eb[1], eb[2], 254})
	}
	return start, end
}

func rangeCount(start, end netip.Addr) (uint64, bool) {
	if !start.IsValid() || !end.IsValid() || !start.Is4() || !end.Is4() || end.Compare(start) < 0 {
		return 0, false
	}
	s, e := binary.BigEndian.Uint32(start.AsSlice()), binary.BigEndian.Uint32(end.AsSlice())
	return uint64(e) - uint64(s) + 1, true
}

// Segmented IP input guard: the range is eight octet cells, `x.x.x.x –
// x.x.x.x`, digits only. The three network octets are locked when both
// ends share a /24 prefix, so only host numbers can change.

// digitsOnly keeps just the digits of s: an illegal character never
// reaches the cell.
func digitsOnly(s string) string {
	b := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= '0' && c <= '9' {
			b = append(b, c)
		}
	}
	return string(b)
}

// clampOctet reduces a cell to digits, at most three of them, worth at
// most 255: whatever an IME or paste slipped in, what stands always
// parses as an octet, an over-cap value losing its last digit.
func clampOctet(s string) string {
	s = digitsOnly(s)
	if len(s) > 3 {
		s = s[:3]
	}
	if n, err := strconv.Atoi(s); err == nil && n > 255 {
		s = s[:len(s)-1]
	}
	return s
}

// splitRange breaks two addresses into their shared first three octets
// and the host numbers. ok is false unless both parse and share the
// prefix, in which case the guard locks the network cells.
func splitRange(start, end netip.Addr) (prefix [3]byte, sHost, eHost int, ok bool) {
	if !start.IsValid() || !end.IsValid() || !start.Is4() || !end.Is4() {
		return
	}
	sb, eb := start.As4(), end.As4()
	if sb[0] != eb[0] || sb[1] != eb[1] || sb[2] != eb[2] {
		return
	}
	copy(prefix[:], sb[:3])
	return prefix, int(sb[3]), int(eb[3]), true
}
