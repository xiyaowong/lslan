package scan

import (
	"net/netip"
	"testing"
	"time"
)

func TestARPCacheReadsOncePerWindow(t *testing.T) {
	ip := netip.MustParseAddr("192.168.1.10")
	calls := 0
	c := newARPCache(500 * time.Millisecond)
	now := time.Unix(0, 0)
	c.read = func() map[netip.Addr]string {
		calls++
		return map[netip.Addr]string{ip: "aa:bb:cc:dd:ee:ff"}
	}
	c.now = func() time.Time { return now }

	// Many lookups inside one window share a single table read.
	for range 5 {
		mac, ok := c.mac(ip)
		if !ok || mac != "aa:bb:cc:dd:ee:ff" {
			t.Fatalf("mac(%v) = %q, %v; want aa:bb:cc:dd:ee:ff, true", ip, mac, ok)
		}
	}
	if calls != 1 {
		t.Fatalf("table read %d times within one window, want 1", calls)
	}

	// After the window lapses the next lookup refreshes.
	now = now.Add(501 * time.Millisecond)
	if _, ok := c.mac(ip); !ok {
		t.Error("entry lost after window refresh")
	}
	if calls != 2 {
		t.Fatalf("table read %d times after window lapsed, want 2", calls)
	}
}

func TestARPCacheUnknownIP(t *testing.T) {
	c := newARPCache(time.Minute)
	now := time.Unix(0, 0)
	c.now = func() time.Time { return now }
	reads := 0
	c.read = func() map[netip.Addr]string {
		reads++
		return map[netip.Addr]string{netip.MustParseAddr("192.168.1.1"): "aa:bb:cc:dd:ee:ff"}
	}

	if _, ok := c.mac(netip.MustParseAddr("10.0.0.1")); ok {
		t.Error("unexpected hit for unknown IP")
	}
	// A miss must not force a re-read before the window lapses.
	if _, ok := c.mac(netip.MustParseAddr("10.0.0.2")); ok {
		t.Error("unexpected hit for unknown IP")
	}
	if reads != 1 {
		t.Fatalf("table read %d times, want 1", reads)
	}
}

func TestARPCacheFailedReadCachedAsEmpty(t *testing.T) {
	c := newARPCache(time.Minute)
	now := time.Unix(0, 0)
	c.now = func() time.Time { return now }
	calls := 0
	c.read = func() map[netip.Addr]string {
		calls++
		return nil // API failure
	}
	for range 3 {
		if _, ok := c.mac(netip.MustParseAddr("10.0.0.1")); ok {
			t.Error("unexpected hit on failed read")
		}
	}
	if calls != 1 {
		t.Fatalf("failed read repeated %d times within the window, want 1", calls)
	}
}
