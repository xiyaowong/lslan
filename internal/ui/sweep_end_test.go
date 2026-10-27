package ui

import (
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
	"lslan/internal/scan"
)

// TestSweepEndSaysSomething: every completed sweep ends with a sentence,
// not just a button returning to normal.
func TestSweepEndSaysSomething(t *testing.T) {
	cases := []struct {
		name  string
		hosts map[string]model.Host
		want  string
	}{
		{"none", map[string]model.Host{}, "没有设备在线"},
		{"some", map[string]model.Host{
			"192.168.1.5": {IP: ip("192.168.1.5"), State: model.StateOnline, MAC: "{{MAC_z6ybfx3d}}"},
		}, "找到 1 台设备"},
	}
	for _, c := range cases {
		a := newApp()
		a.scanning = true
		for _, h := range c.hosts {
			a.hosts[h.IP] = h
		}
		a.applyEvent(scan.Event{Kind: scan.EventDone})
		if !strings.Contains(a.scanNote, c.want) {
			t.Errorf("%s: note = %q, want it to contain %q", c.name, a.scanNote, c.want)
		}
	}
}

// TestDetailHeaderSaysKind: the panel names what the device is.
func TestDetailHeaderSaysKind(t *testing.T) {
	a := newApp()
	h := ip("192.168.1.1")
	a.hosts[h] = model.Host{IP: h, State: model.StateOnline, Gateway: true, MAC: "{{MAC_yhm3dm3c}}"}
	a.ports[h] = nil
	a.selRow, a.selIP = 0, h
	tt := ui.NewTester(a.View, 1040, 720)
	tt.Frame()
	if !tt.HasText("路由器") {
		t.Errorf("the detail panel never says what it is, texts %q", tt.Texts())
	}
}
