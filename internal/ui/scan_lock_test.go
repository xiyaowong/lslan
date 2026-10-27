package ui

import (
	"context"
	"net/netip"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestScanStopsRunningTools: a new sweep clears the table and the
// selection, so a running tool would lose its pane and every way to stop
// it. The sweep ends them and says so.
func TestScanStopsRunningTools(t *testing.T) {
	a := newApp()
	a.listNICs = func() ([]model.NIC, error) {
		return []model.NIC{{Name: "以太网", IP: netip.MustParseAddr("192.168.1.23"), PrefixLen: 24}}, nil
	}
	a.rangeForNIC = func(model.NIC) (netip.Addr, netip.Addr) {
		return netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("192.168.1.254")
	}
	a.newScanner = func() scannerIface { return &countingScanner{starts: new(int)} }
	sess := &fakePingSession{samples: make(chan model.PingSample)}
	a.startPingFn = func(context.Context, netip.Addr) (pingSessionIface, error) { return sess, nil }
	a.newPingTerm = func() (pingTerminalIface, error) { return &fakePingTerminal{}, nil }
	tt := ui.NewTester(a.View, 1040, 720)
	a.loadNICs()

	host := ip("192.168.1.9")
	a.hosts[host] = model.Host{IP: host, State: model.StateOnline}
	a.selRow, a.selIP = 0, host
	a.startPing(host)
	tt.Frame()
	if a.pingCancel == nil {
		t.Fatal("the ping did not start")
	}

	if err := tt.Click("开始扫描"); err != nil {
		t.Fatalf("click scan: %v", err)
	}
	tt.Frame()
	if a.pingCancel != nil {
		t.Error("the ping outlived the sweep that hid its pane")
	}
	if !tt.HasText("已停止") {
		t.Errorf("the stop was not reported, texts %q", tt.Texts())
	}
}

// TestControlsLockedDuringScan: the adapter, the range and the octets are
// fixed while a sweep runs, so the screen cannot disagree with it.
func TestControlsLockedDuringScan(t *testing.T) {
	a := newApp()
	a.listNICs = func() ([]model.NIC, error) {
		return []model.NIC{{Name: "以太网", IP: netip.MustParseAddr("192.168.1.23"), PrefixLen: 24}}, nil
	}
	a.rangeForNIC = func(model.NIC) (netip.Addr, netip.Addr) {
		return netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("192.168.1.254")
	}
	a.newScanner = func() scannerIface { return &countingScanner{starts: new(int)} }
	tt := ui.NewTester(a.View, 1040, 720)
	a.loadNICs()
	tt.Frame()

	if err := tt.Click("开始扫描"); err != nil {
		t.Fatalf("click scan: %v", err)
	}
	tt.Frame()
	// Clicking a range button mid-sweep must not move the range.
	before := a.ipCells
	if err := tt.Click("整个网段"); err != nil {
		t.Logf("the range button is gone or disabled, as it should be: %v", err)
	}
	if a.ipCells != before {
		t.Error("the range changed during a sweep")
	}
}
