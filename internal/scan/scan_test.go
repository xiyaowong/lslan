package scan

import (
	"fmt"
	"net/netip"
	"testing"

	"lslan/internal/model"
)

func TestRangeForNIC(t *testing.T) {
	cases := []struct {
		nic   model.NIC
		start string
		end   string
	}{
		{model.NIC{IP: na("192.168.1.123"), PrefixLen: 24}, "192.168.1.1", "192.168.1.254"},
		{model.NIC{IP: na("10.0.0.5"), PrefixLen: 8}, "10.0.0.1", "10.255.255.254"},
		{model.NIC{IP: na("172.16.4.66"), PrefixLen: 16}, "172.16.0.1", "172.16.255.254"},
		{model.NIC{IP: na("192.168.9.2"), PrefixLen: 30}, "192.168.9.1", "192.168.9.2"},
		{model.NIC{IP: na("192.168.9.130"), PrefixLen: 25}, "192.168.9.129", "192.168.9.254"},
		// /31 and /32 have no reserved network/broadcast address.
		{model.NIC{IP: na("10.0.0.1"), PrefixLen: 31}, "10.0.0.0", "10.0.0.1"},
		{model.NIC{IP: na("10.0.0.7"), PrefixLen: 32}, "10.0.0.7", "10.0.0.7"},
	}
	for _, c := range cases {
		start, end := RangeForNIC(c.nic)
		if start.String() != c.start || end.String() != c.end {
			t.Errorf("RangeForNIC(%v, /%d) = %v..%v, want %v..%v",
				c.nic.IP, c.nic.PrefixLen, start, end, c.start, c.end)
		}
	}
}

func TestRangeForNICInvalid(t *testing.T) {
	start, end := RangeForNIC(model.NIC{})
	if start.IsValid() || end.IsValid() {
		t.Errorf("RangeForNIC(zero NIC) = %v..%v, want invalid addrs", start, end)
	}
}

// macStr renders six octets in the standard colon format.
func macStr(o ...byte) string {
	s := ""
	for i, b := range o {
		if i > 0 {
			s += ":"
		}
		s += fmt.Sprintf("%02x", b)
	}
	return s
}

func TestOUIVendor(t *testing.T) {
	// Any prefix stored in the table must resolve, whatever the MAC
	// separator style.
	var prefix, want string
	for p, name := range ouiTable {
		prefix, want = p, name
		break
	}
	o := []byte{hexVal(prefix[0])<<4 | hexVal(prefix[1]),
		hexVal(prefix[2])<<4 | hexVal(prefix[3]),
		hexVal(prefix[4])<<4 | hexVal(prefix[5])}
	for _, mac := range []string{
		macStr(o[0], o[1], o[2], 0x11, 0x22, 0x33),
		fmt.Sprintf("%02x-%02x-%02x-%02x-%02x-%02x", o[0], o[1], o[2], 0, 0, 1),
		fmt.Sprintf("%02x%02x%02x%02x%02x%02x", o[0], o[1], o[2], 0, 0, 1),
	} {
		if got := OUIVendor(mac); got != want {
			t.Errorf("OUIVendor(%q) = %q, want %q", mac, got, want)
		}
	}

	// Locally administered addresses never match a registry OUI.
	for _, first := range []byte{0x02, 0x06, 0x0a, 0x0e} {
		if got := OUIVendor(macStr(first, 0x11, 0x22, 0x33, 0x44, 0x55)); got != "" {
			t.Errorf("OUIVendor(locally administered) = %q, want empty", got)
		}
	}

	// Garbage input must not panic and must not resolve.
	for _, mac := range []string{"", "zz:zz:zz:zz:zz:zz", "12:34", "a1:b2:c3:d4:e5"} {
		if got := OUIVendor(mac); got != "" {
			t.Errorf("OUIVendor(%q) = %q, want empty", mac, got)
		}
	}
}

func hexVal(c byte) byte {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

func TestPortPresets(t *testing.T) {
	presets := PortPresets()
	names := make(map[string]bool, len(presets))
	for _, p := range presets {
		names[p.Name] = true
		if len(p.Ports) == 0 {
			t.Errorf("preset %q has no ports", p.Name)
		}
		prev := 0
		for _, port := range p.Ports {
			if port <= prev {
				t.Errorf("preset %q ports not ascending at %d", p.Name, port)
			}
			prev = port
		}
	}
	for _, want := range []string{"常用", "摄像头", "路由器", "NAS"} {
		if !names[want] {
			t.Errorf("missing preset %q", want)
		}
	}
}

func na(s string) netip.Addr { return netip.MustParseAddr(s) }
