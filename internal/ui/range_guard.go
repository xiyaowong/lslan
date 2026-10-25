package ui

import (
	"net/netip"
	"strconv"
)

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
