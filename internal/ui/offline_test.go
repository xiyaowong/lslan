package ui

import (
	"net/netip"
	"testing"

	"lslan/internal/model"
	"lslan/internal/scan"
)

// TestOfflineFilter: hosts the engine marks offline appear only when the
// 显示离线 switch is on, and do not count as discovered (issue #21).
func TestOfflineFilter(t *testing.T) {
	a := newApp()
	online := netip.MustParseAddr("192.168.1.5")
	gone := netip.MustParseAddr("192.168.1.6")
	a.applyEvent(scan.Event{Kind: scan.EventHost, Host: &model.Host{IP: online, State: model.StateOnline}})
	a.applyEvent(scan.Event{Kind: scan.EventHost, Host: &model.Host{IP: gone, State: model.StateOffline}})
	a.progress = model.Progress{Total: 2}

	if a.discoveredCount() != 1 {
		t.Errorf("discoveredCount = %d, want 1", a.discoveredCount())
	}
	a.showOffline = false
	if vis := a.visibleHosts(); len(vis) != 1 || vis[0].IP != online {
		t.Errorf("with the switch off, rows = %v, want only the online host", vis)
	}
	a.showOffline = true
	if vis := a.visibleHosts(); len(vis) != 2 {
		t.Errorf("with the switch on, rows = %d, want 2", len(vis))
	}
}
