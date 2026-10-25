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

// TestSweepPortsProbing: while the sweep probes a host, its details say
// so rather than that it was never scanned.
func TestSweepPortsProbing(t *testing.T) {
	a := newApp()
	release := make(chan struct{})
	a.scanPorts = func(ctx context.Context, ip netip.Addr, ports []int) ([]model.Port, error) {
		<-release
		return []model.Port{{Number: 80, Open: true, Service: "http"}}, nil
	}
	tt := ui.NewTester(a.View, 1240, 800)
	ip := netip.MustParseAddr("192.168.1.5")
	a.hosts[ip] = model.Host{IP: ip, State: model.StateOnline}
	a.selRow, a.selIP = 0, ip
	tt.Frame()

	a.sweepPorts()
	tt.Frame()
	if !tt.HasText("正在扫描端口") {
		t.Errorf("probing host reads as never scanned, texts %q", tt.Texts())
	}

	close(release)
	deadline := time.Now().Add(time.Second)
	for a.sweepLeft > 0 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
		a.runPending()
	}
	tt.Frame()
	if tt.HasText("正在扫描端口") {
		t.Error("still probing after the sweep finished")
	}
	if !tt.HasText("80") {
		t.Errorf("the swept port is missing, texts %q", tt.Texts())
	}
}

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

// TestSweptPortsInDetail: the ports a sweep found show in the detail
// panel, and opening the host neither scans again nor hides them.
func TestSweptPortsInDetail(t *testing.T) {
	a := newApp()
	var calls atomic.Int32
	a.scanPorts = func(ctx context.Context, ip netip.Addr, ports []int) ([]model.Port, error) {
		calls.Add(1)
		return nil, nil
	}
	tt := ui.NewTester(a.View, 1240, 800)
	ip := netip.MustParseAddr("192.168.1.5")
	a.hosts[ip] = model.Host{IP: ip, State: model.StateOnline, Ports: []model.Port{
		{Number: 80, Open: true, Service: "http"},
		{Number: 445, Open: true, Service: "smb"},
		{Number: 22, Open: false, Service: "ssh"},
	}}
	a.autoScanned = map[netip.Addr]bool{ip: true} // the sweep did it
	tt.Frame()

	a.selRow, a.selIP = 0, ip
	tt.Frame()
	a.runPending()
	if n := calls.Load(); n != 0 {
		t.Errorf("opening a swept host probed again %d times", n)
	}
	if !tt.HasText("80") || !tt.HasText("445") {
		t.Errorf("the sweep's ports are missing, texts %q", tt.Texts())
	}
	if !tt.HasText("开放 2 个") {
		t.Errorf("open-port count missing, texts %q", tt.Texts())
	}
	if tt.HasText("还没扫描过") {
		t.Error("a swept host reads as never scanned")
	}
}
