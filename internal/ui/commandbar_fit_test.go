package ui

import (
	"net/netip"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

func TestCommandBarFitsSmallest(t *testing.T) {
	for _, size := range []struct{ w, h int }{{1040, 720}, {860, 520}} {
		for _, selected := range []bool{false, true} {
			a := newApp()
			a.listNICs = func() ([]model.NIC, error) {
				return []model.NIC{{Name: "WLAN", IP: netip.MustParseAddr("192.168.3.205"), PrefixLen: 24}}, nil
			}
			a.rangeForNIC = func(model.NIC) (netip.Addr, netip.Addr) {
				return netip.MustParseAddr("192.168.3.1"), netip.MustParseAddr("192.168.3.254")
			}
			tt := ui.NewTester(a.View, size.w, size.h)
			a.loadNICs()
			h := netip.MustParseAddr("192.168.3.254")
			a.hosts[h] = model.Host{IP: h, State: model.StateOnline}
			a.ports[h] = nil
			if selected {
				a.selRow, a.selIP = 0, h
			}
			tt.Frame()
			// The scan button is at the card's right; nothing may run past it.
			scan, ok := tt.Find("开始扫描")
			if !ok {
				t.Fatalf("%dx%d sel=%v: scan button missing", size.w, size.h, selected)
			}
			if scan.X+scan.W > float32(size.w) {
				t.Errorf("%dx%d sel=%v: scan button runs past the window", size.w, size.h, selected)
			}
			for _, s := range []string{"整个网段", "顺带扫描端口"} {
				r, ok := tt.Find(s)
				if !ok {
					t.Errorf("%dx%d sel=%v: %q missing", size.w, size.h, selected, s)
					continue
				}
				if r.X+r.W > float32(size.w) {
					t.Errorf("%dx%d sel=%v: %q runs past the window at %v", size.w, size.h, selected, s, r)
				}
			}
		}
	}
}
