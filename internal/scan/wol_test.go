package scan

import (
	"net"
	"net/netip"
	"strings"
	"testing"

	"lslan/internal/model"
)

// TestParseMAC: the common spellings all normalize to the colon form,
// and anything that is not twelve hex digits is refused rather than
// guessed at.
func TestParseMAC(t *testing.T) {
	const want = "aa:bb:cc:dd:ee:ff"
	for _, in := range []string{
		"aa:bb:cc:dd:ee:ff",
		"AA:BB:CC:DD:EE:FF",
		"aa-bb-cc-dd-ee-ff",
		"aabbccddeeff",
		"aabb.ccdd.eeff",
		"  aa:bb:cc:dd:ee:ff  ",
	} {
		got, ok := ParseMAC(in)
		if !ok || got != want {
			t.Errorf("ParseMAC(%q) = %q, %v; want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "aa:bb:cc:dd:ee", "aa:bb:cc:dd:ee:ff:11:22", "gg:bb:cc:dd:ee:ff", "12345"} {
		if got, ok := ParseMAC(in); ok {
			t.Errorf("ParseMAC(%q) = %q, accepted", in, got)
		}
	}
}

// TestMagicPacket: six 0xFF bytes then the hardware address sixteen
// times — the frame every NIC listens for.
func TestMagicPacket(t *testing.T) {
	mac := [6]byte{0x01, 0x23, 0x45, 0x67, 0x89, 0xab}
	p := magicPacket(mac)
	if len(p) != 102 {
		t.Fatalf("packet is %d bytes, want 102", len(p))
	}
	if strings.Count(string(p[:6]), "\xff") != 6 {
		t.Errorf("header = % x, want six 0xFF", p[:6])
	}
	for i := 6; i < len(p); i += 6 {
		if [6]byte(p[i:i+6]) != mac {
			t.Fatalf("repetition at %d = % x, want % x", i, p[i:i+6], mac)
		}
	}
}

// TestWakeTargets: the frame goes to the limited broadcast, and to the
// LAN's own when the adapter names a subnet narrower than the world.
func TestWakeTargets(t *testing.T) {
	dsts := wakeTargets(model.NIC{IP: netip.MustParseAddr("192.168.1.23"), PrefixLen: 24})
	if len(dsts) != 2 {
		t.Fatalf("targets = %v, want the limited and the LAN broadcast", dsts)
	}
	if !dsts[0].Equal(net.IPv4bcast) {
		t.Errorf("first target = %v, want 255.255.255.255", dsts[0])
	}
	if got := dsts[1].String(); got != "192.168.1.255" {
		t.Errorf("LAN broadcast = %s, want 192.168.1.255", got)
	}
	// A NIC without an address carries no subnet of its own.
	if got := wakeTargets(model.NIC{}); len(got) != 1 {
		t.Errorf("targets of an empty NIC = %v, want the limited broadcast alone", got)
	}
}
