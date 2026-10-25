package ui

import (
	"context"
	"net/netip"
	"sync/atomic"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"lslan/internal/model"
	"lslan/internal/scan"
)

// TestSweepPorts: with the option on, the end of a sweep probes the
// online hosts' ports and puts them on their rows.
func TestSweepPorts(t *testing.T) {
	a := newApp()
	var calls atomic.Int32
	a.scanPorts = func(ctx context.Context, ip netip.Addr, ports []int) ([]model.Port, error) {
		calls.Add(1)
		return []model.Port{{Number: 80, Open: true, Service: "http"}}, nil
	}
	tt := ui.NewTester(a.View, 1000, 680)
	online := netip.MustParseAddr("192.168.1.5")
	gone := netip.MustParseAddr("192.168.1.6")
	a.hosts[online] = model.Host{IP: online, State: model.StateOnline}
	a.hosts[gone] = model.Host{IP: gone, State: model.StateOffline}
	tt.Frame()

	a.withPorts = true
	a.applyEvent(scan.Event{Kind: scan.EventDone})
	deadline := time.Now().Add(time.Second)
	for calls.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	a.runPending()
	if n := calls.Load(); n != 1 {
		t.Fatalf("probes = %d, want 1, the offline host skipped", n)
	}
	if got := a.hosts[online].Ports; len(got) != 1 || got[0].Number != 80 {
		t.Errorf("ports of the online host = %v, want 80", got)
	}
	if !a.autoScanned[online] {
		t.Error("a swept host should not be probed again when its details open")
	}
}
