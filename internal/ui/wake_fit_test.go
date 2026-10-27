package ui

import (
	"net/netip"
	"strconv"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestWakeDialogFitsSmallWindow: with the full set of picks and the
// sweep line, the dialog still fits the window's least height — the picks
// scroll rather than push the buttons off the bottom.
func TestWakeDialogFitsSmallWindow(t *testing.T) {
	for _, size := range []struct{ w, h int }{{1040, 720}, {860, 520}} {
		wa, _ := newScanableWakeApp(t)
		wa.listNICs = func() ([]model.NIC, error) {
			return []model.NIC{{Name: "WLAN", IP: netip.MustParseAddr("192.168.3.205"), PrefixLen: 24}}, nil
		}
		wa.rangeForNIC = func(model.NIC) (netip.Addr, netip.Addr) {
			return netip.MustParseAddr("192.168.3.1"), netip.MustParseAddr("192.168.3.254")
		}
		tt := ui.NewTester(wa.View, size.w, size.h)
		wa.loadNICs()
		for i := 1; i <= wakePickLimit; i++ {
			p := netip.MustParseAddr("192.168.3." + strconv.Itoa(i))
			wa.hosts[p] = model.Host{IP: p, State: model.StateOffline, MAC: "aa:bb:cc:dd:ee:ff", Vendor: "SomeVendor"}
		}
		wa.progress = model.Progress{Total: 254, Scanned: 254}
		tt.Frame()

		wa.openWake()
		tt.Frame()
		for _, s := range []string{"选一台设备", "重新扫描", "设备地址", "MAC 地址", "发送唤醒信号", "取消"} {
			r, ok := tt.Find(s)
			if !ok {
				t.Fatalf("%dx%d: %q missing from the dialog", size.w, size.h, s)
			}
			if r.X < 0 || r.Y < 0 || r.X+r.W > float32(size.w) || r.Y+r.H > float32(size.h) {
				t.Errorf("%dx%d: %q at %v runs past the window", size.w, size.h, s, r)
			}
		}
	}
}
