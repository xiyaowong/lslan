package ui

import (
	"context"
	"net/netip"
	"testing"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
)

// TestAutoPortScan: opening a host scans its ports once per sweep; coming
// back to it does not scan again, and a new sweep allows one more.
func TestAutoPortScan(t *testing.T) {
	a := newApp()
	a.listNICs = func() ([]model.NIC, error) {
		return []model.NIC{{Name: "以太网", IP: netip.MustParseAddr("192.168.1.23"), PrefixLen: 24}}, nil
	}
	a.rangeForNIC = func(n model.NIC) (netip.Addr, netip.Addr) {
		return netip.MustParseAddr("192.168.1.1"), netip.MustParseAddr("192.168.1.254")
	}
	a.newScanner = func() scannerIface { return &fakeScanner{} }
	calls := 0
	a.scanPorts = func(ctx context.Context, src, ip netip.Addr, ports []int) ([]model.Port, error) {
		calls++
		return nil, nil
	}
	tt := ui.NewTester(a.View, 1000, 680)
	a.loadNICs()
	ip := netip.MustParseAddr("192.168.1.6")
	a.hosts[ip] = model.Host{IP: ip, State: model.StateOnline}
	tt.Frame()

	look := func() {
		t.Helper()
		if err := tt.Click(ip.String()); err != nil {
			t.Fatalf("click the host: %v", err)
		}
		tt.Frame()
		a.runPending()
	}
	leave := func() {
		t.Helper()
		a.selRow, a.selIP = -1, netip.Addr{}
		tt.Frame()
	}

	// The first look scans.
	look()
	if calls != 1 {
		t.Fatalf("scans after opening = %d, want 1", calls)
	}

	// Going away and back does not, within the sweep.
	leave()
	look()
	if calls != 1 {
		t.Errorf("scans after returning = %d, want still 1", calls)
	}

	// A new sweep forgets, so the next look scans again.
	a.startScan()
	tt.Frame()
	a.hosts[ip] = model.Host{IP: ip, State: model.StateOnline}
	leave()
	look()
	if calls != 2 {
		t.Errorf("scans after a new sweep = %d, want 2", calls)
	}
}
