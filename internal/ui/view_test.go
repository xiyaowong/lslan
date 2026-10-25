package ui

import (
	"net/netip"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// Smoke test: the view renders without error and NIC loading fills the
// range. Engine calls are faked; no real scan runs.
func TestSmoke(t *testing.T) {
	a := newApp()
	a.listNICs = func() ([]model.NIC, error) {
		return []model.NIC{{Name: "以太网", IP: netip.MustParseAddr("192.168.1.23"), PrefixLen: 24}}, nil
	}
	a.rangeForNIC = func(n model.NIC) (netip.Addr, netip.Addr) {
		return netip.MustParseAddr("192.168.1.0"), netip.MustParseAddr("192.168.1.255")
	}
	tt := ui.NewTester(a.View, 1000, 680)
	a.loadNICs()
	tt.Frame()
	if !tt.HasText("以太网 (192.168.1.23)") {
		t.Errorf("NIC option missing, texts %q", tt.Texts())
	}
	if a.startIP != "192.168.1.1" || a.endIP != "192.168.1.254" {
		t.Errorf("range = %s..%s, want 192.168.1.1..192.168.1.254", a.startIP, a.endIP)
	}
}
