package ui

import (
	"net/netip"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
	"lslan/internal/scan"
)

// TestRangeLimit: a range too large never starts, and the bar says why.
func TestRangeLimit(t *testing.T) {
	a := newApp()
	a.listNICs = func() ([]model.NIC, error) {
		return []model.NIC{{Name: "以太网", IP: netip.MustParseAddr("10.0.0.5"), PrefixLen: 8}}, nil
	}
	a.rangeForNIC = func(model.NIC) (netip.Addr, netip.Addr) {
		return netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("10.255.255.254")
	}
	starts := 0
	a.newScanner = func() scannerIface { return &countingScanner{starts: &starts} }
	tt := ui.NewTester(a.View, 1040, 720)
	a.loadNICs()
	tt.Frame()

	if !tt.HasText("一次最多扫") {
		t.Errorf("no limit hint for a /8, texts %q", tt.Texts())
	}
	if a.rangeFits() {
		t.Error("a /8 reads as scannable")
	}
	if err := tt.Click("开始扫描"); err != nil {
		t.Fatalf("click: %v", err)
	}
	if starts != 0 {
		t.Error("a /8 started a sweep")
	}
}

// TestCancelSaysStopping: cancelling shows at once that it is stopping,
// and the end of the sweep reports the cancel instead of a result.
func TestCancelSaysStopping(t *testing.T) {
	a := newApp()
	a.listNICs = func() ([]model.NIC, error) {
		return []model.NIC{{Name: "以太网", IP: netip.MustParseAddr("192.168.1.23"), PrefixLen: 24}}, nil
	}
	a.rangeForNIC = func(model.NIC) (netip.Addr, netip.Addr) {
		return netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("192.168.1.254")
	}
	sc := &countingScanner{starts: new(int)}
	a.newScanner = func() scannerIface { return sc }
	a.withPorts = true // cancelling must not start the port probes
	tt := ui.NewTester(a.View, 1040, 720)
	a.loadNICs()
	tt.Frame()

	if err := tt.Click("开始扫描"); err != nil {
		t.Fatalf("click scan: %v", err)
	}
	tt.Frame()
	if !a.scanning {
		t.Fatal("the sweep did not start")
	}
	if err := tt.Click("取消扫描"); err != nil {
		t.Fatalf("click cancel: %v", err)
	}
	tt.Frame()
	if !a.scanStopping {
		t.Error("cancelling did not show as stopping")
	}
	if !tt.HasText("正在停止") {
		t.Errorf("no stopping words, texts %q", tt.Texts())
	}

	// The engine reports the end; the toast carries the outcome and clears
	// the note in the same frame.
	a.applyEvent(scan.Event{Kind: scan.EventDone})
	if a.scanNote == "" {
		t.Error("the cancel was not reported")
	}
	tt.Frame()
	if a.sweepLeft != 0 {
		t.Errorf("a cancelled sweep started %d port probes", a.sweepLeft)
	}
}
