package ui

import (
	"net/netip"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestIPCellBadgesFit: the IP cell carries the address and its badges.
// The column was too narrow for both, and a badge shrank to zero width
// instead of showing.
func TestIPCellBadgesFit(t *testing.T) {
	for _, size := range []struct{ w, h int }{{1040, 720}, {860, 520}} {
		a := newApp()
		a.listNICs = func() ([]model.NIC, error) {
			return []model.NIC{{Name: "WLAN", IP: netip.MustParseAddr("192.168.3.205"), PrefixLen: 24}}, nil
		}
		a.rangeForNIC = func(model.NIC) (netip.Addr, netip.Addr) {
			return netip.MustParseAddr("192.168.3.1"), netip.MustParseAddr("192.168.3.254")
		}
		tt := ui.NewTester(a.View, size.w, size.h)
		a.loadNICs()
		h := netip.MustParseAddr("192.168.3.254") // the widest address
		a.hosts[h] = model.Host{IP: h, State: model.StateOnline, Gateway: true, ViaMDNS: true}
		a.ports[h] = nil
		a.selRow, a.selIP = 0, h
		tt.Frame()
		for _, s := range []string{"网关", "服务发现"} {
			r, ok := tt.Find(s)
			if !ok {
				t.Fatalf("%dx%d: %q missing", size.w, size.h, s)
			}
			if r.W <= 0 {
				t.Errorf("%dx%d: %q is %v wide, want it visible", size.w, size.h, s, r.W)
			}
		}
	}
}

// TestCardGutter: the cards keep a gap below the title bar rather than
// touching it, and leave the window on the same side margin.
func TestCardGutter(t *testing.T) {
	a := newApp()
	a.listNICs = func() ([]model.NIC, error) {
		return []model.NIC{{Name: "WLAN", IP: netip.MustParseAddr("192.168.3.205"), PrefixLen: 24}}, nil
	}
	a.rangeForNIC = func(model.NIC) (netip.Addr, netip.Addr) {
		return netip.MustParseAddr("192.168.3.1"), netip.MustParseAddr("192.168.3.254")
	}
	tt := ui.NewTester(a.View, 1040, 720)
	a.loadNICs()
	tt.Frame()

	sel, ok := tt.Find("WLAN (192.168.3.205)")
	if !ok {
		t.Fatal("the adapter select is missing")
	}
	if sel.Y < float32(TitleBarHeight)+8 {
		t.Errorf("the card's content starts at y=%v, too close to the title bar", sel.Y)
	}
	scan, ok := tt.Find("开始扫描")
	if !ok {
		t.Fatal("the scan button is missing")
	}
	if right := scan.X + scan.W; right > 1040-float32(gutter) {
		t.Errorf("the scan button ends at %v, past the card's right margin", right)
	}
}
