package ui

import (
	"net/netip"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestDefaultWindowLayout: at the default window size, with and without
// the detail panel, nothing runs past the window and both panes keep a
// usable width.
func TestDefaultWindowLayout(t *testing.T) {
	for _, size := range []struct{ w, h int }{{900, 620}, {860, 520}} {
		for _, selected := range []bool{false, true} {
			a := newApp()
			a.listNICs = func() ([]model.NIC, error) {
				return []model.NIC{{Name: "WLAN (192.168.3.205)", IP: netip.MustParseAddr("192.168.3.205"), PrefixLen: 24}}, nil
			}
			a.rangeForNIC = func(n model.NIC) (netip.Addr, netip.Addr) {
				return netip.MustParseAddr("192.168.3.1"), netip.MustParseAddr("192.168.3.254")
			}
			tt := ui.NewTester(a.View, size.w, size.h)
			a.loadNICs()
			ip := netip.MustParseAddr("192.168.3.1")
			a.hosts[ip] = model.Host{IP: ip, State: model.StateOnline, Hostname: "router", MAC: "aa:bb:cc:dd:ee:ff"}
			if selected {
				a.ports[ip] = nil // the sweep asked already
				a.selRow, a.selIP = 0, ip
			}
			tt.Frame()

			for _, text := range []string{"开始扫描", "整个网段", "顺带扫描端口", "显示离线地址", ip.String()} {
				r, ok := tt.Find(text)
				if !ok {
					t.Fatalf("%dx%d selected=%v: %q is missing", size.w, size.h, selected, text)
				}
				if r.X+r.W > float32(size.w) || r.Y+r.H > float32(size.h) {
					t.Errorf("%dx%d selected=%v: %q runs past the window at %v", size.w, size.h, selected, text, r)
				}
			}
			if !selected {
				continue
			}
			closeBtn, ok := tt.Find("关闭详情")
			if !ok {
				t.Fatalf("%dx%d: the detail panel is missing", size.w, size.h)
			}
			if closeBtn.X < float32(size.w)*0.5 {
				t.Errorf("%dx%d: the panel starts at x=%v, leaving the card too little", size.w, size.h, closeBtn.X)
			}
			if closeBtn.X+closeBtn.W > float32(size.w) {
				t.Errorf("%dx%d: the panel runs past the window", size.w, size.h)
			}
			// Every action and fact fits the panel's width, unclipped.
			for _, text := range []string{"Ping", "端口扫描", "Tracert", "浏览共享", "浏览器打开", "更多", "MAC 地址", "上次在线"} {
				r, ok := tt.Find(text)
				if !ok {
					t.Errorf("%dx%d: %q is missing from the panel", size.w, size.h, text)
					continue
				}
				if r.X+r.W > closeBtn.X+closeBtn.W {
					t.Errorf("%dx%d: %q at %v runs past the panel", size.w, size.h, text, r)
				}
			}
		}
	}
}
