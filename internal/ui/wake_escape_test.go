package ui

import (
	"net/netip"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestWakeDialogScanSaysWhy: the dialog's scan button explains itself
// inside the dialog. Its errors used to go to the status bar, which the
// dialog's backdrop covers, so nothing appeared to happen.
func TestWakeDialogScanSaysWhy(t *testing.T) {
	a := newApp()
	a.nics = nil
	a.nicErr = "" // still loading: no adapter to scan with
	tt := ui.NewTester(a.View, 1040, 720)
	a.openWake()
	tt.Frame()
	if !tt.HasText("先选一块网卡") {
		t.Errorf("no reason given in the dialog, texts %q", tt.Texts())
	}
}

// TestWakeWaitCanBeStopped: the wait for a woken device has a way out.
func TestWakeWaitCanBeStopped(t *testing.T) {
	wa := newWakeApp()
	wa.listNICs = func() ([]model.NIC, error) {
		return []model.NIC{{Name: "WLAN (192.168.3.205)", IP: netip.MustParseAddr("192.168.3.205"), PrefixLen: 24}}, nil
	}
	wa.rangeForNIC = func(model.NIC) (netip.Addr, netip.Addr) {
		return netip.MustParseAddr("192.168.3.1"), netip.MustParseAddr("192.168.3.254")
	}
	wa.wakeEvery, wa.wakeFor = 1000, 2000 // stays waiting for the frame
	tt := ui.NewTester(wa.View, 1040, 720)
	wa.loadNICs()
	target := ip("192.168.3.20")
	wa.hosts[target] = model.Host{IP: target, State: model.StateOffline}
	wa.wake.ipText, wa.wake.mac = target.String(), "aa:bb:cc:dd:ee:ff"
	tt.Frame()

	wa.startWake()
	tt.Frame()
	if !wa.wake.waiting {
		t.Fatal("the wait did not start")
	}
	if err := tt.Click("不等了"); err != nil {
		t.Fatalf("click stop waiting: %v", err)
	}
	if wa.wake.waiting {
		t.Error("the wait outlived the stop button")
	}
}
