package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
	"lslan/internal/scan"
)

// TestQuietSweepEnd: a sweep that found the same devices as last time
// says nothing. What it found is on screen — the rows, the tallies, the
// button back to 开始扫描 — so a toast would only repeat it.
func TestQuietSweepEnd(t *testing.T) {
	for _, hosts := range []map[string]model.Host{
		{},
		{"192.168.1.5": {IP: ip("192.168.1.5"), State: model.StateOnline, MAC: "{{MAC_z6ybfx3d}}"}},
	} {
		a := newApp()
		a.scanning = true
		for _, h := range hosts {
			a.hosts[h.IP] = h
		}
		// The baseline is exactly what this sweep will find.
		a.compared = true
		a.lastSeen = a.macSet()
		a.applyEvent(scan.Event{Kind: scan.EventDone})
		if a.scanNote != "" {
			t.Errorf("a sweep with nothing new said %q", a.scanNote)
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
