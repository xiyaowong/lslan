package ui

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
	"lslan/internal/scan"
)

// TestChangeNote: a sweep says who came and who went, by MAC so a reused
// address does not read as the same device.
func TestChangeNote(t *testing.T) {
	a := newApp()
	newcomer := model.Host{IP: ip("192.168.1.20"), State: model.StateOnline, MAC: "aa:aa:aa:aa:aa:01"}
	leaver := model.Host{IP: ip("192.168.1.21"), State: model.StateOnline, MAC: "aa:aa:aa:aa:aa:02"}
	staying := model.Host{IP: ip("192.168.1.22"), State: model.StateOnline, MAC: "aa:aa:aa:aa:aa:03"}

	// The first sweep has nobody to compare with.
	a.hosts[leaver.IP] = leaver
	a.hosts[staying.IP] = staying
	a.noteChanges()
	if a.scanNote != "" {
		t.Errorf("the first sweep drew a conclusion: %q", a.scanNote)
	}

	// The second: one gone, one new, one staying.
	a.lastSeen = a.macSet()
	a.hosts = map[netip.Addr]model.Host{newcomer.IP: newcomer, staying.IP: staying}
	a.noteChanges()
	if !strings.Contains(a.scanNote, "多 1") || !strings.Contains(a.scanNote, "少 1") {
		t.Errorf("note = %q, want one added and one gone", a.scanNote)
	}
}

// TestCancelledSweepDrawsNoConclusion: a sweep the user stopped proves
// nothing about who left, even if the table it left behind is smaller.
func TestCancelledSweepDrawsNoConclusion(t *testing.T) {
	a := newApp()
	a.lastSeen = map[string]bool{"aa:aa:aa:aa:aa:02": true}
	a.hosts = map[netip.Addr]model.Host{}
	a.scanning, a.scanCancelled = true, true // the user cancelled it
	a.applyEvent(scan.Event{Kind: scan.EventDone})
	if strings.Contains(a.scanNote, "比上次") {
		t.Errorf("a cancelled sweep reported a change: %q", a.scanNote)
	}
}

// TestCopyInventory: the rows on screen go to the clipboard as a table.
func TestCopyInventory(t *testing.T) {
	a := newApp()
	h1 := ip("192.168.1.5")
	h2 := ip("192.168.1.6")
	a.hosts[h1] = model.Host{IP: h1, State: model.StateOnline, Hostname: "nas", Vendor: "Synology", MAC: "aa:bb:cc:00:00:01"}
	a.hosts[h2] = model.Host{IP: h2, State: model.StateOffline, MAC: "aa:bb:cc:00:00:02"}
	tt := ui.NewTester(a.View, 1040, 720)
	tt.Frame()

	if err := tt.Click("复制清单"); err != nil {
		t.Fatalf("click copy: %v", err)
	}
	got := tt.Clipboard()
	if !strings.HasPrefix(got, "IP\t主机名\t厂商\tMAC 地址\t状态") {
		t.Errorf("no header: %q", got)
	}
	if !strings.Contains(got, "192.168.1.5\tnas\tSynology\taa:bb:cc:00:00:01\t在线") {
		t.Errorf("row missing: %q", got)
	}
	if strings.Contains(got, "192.168.1.6") {
		t.Error("an offline row was copied while offline rows are hidden")
	}
}
